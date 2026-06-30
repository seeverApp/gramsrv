package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
	"telesrv/internal/store/postgres/sqlcgen"
)

// StarGiftStore 用 PostgreSQL 实现 store.StarGiftStore（peer 收到的 Star 礼物实例）。
type StarGiftStore struct {
	db sqlcgen.DBTX
}

// NewStarGiftStore 基于 pgx 连接池（或事务）创建 StarGiftStore。
func NewStarGiftStore(db sqlcgen.DBTX) *StarGiftStore {
	return &StarGiftStore{db: db}
}

func (s *StarGiftStore) Create(ctx context.Context, gift domain.SavedStarGift) (int64, error) {
	if !validSavedStarGift(gift) {
		return 0, domain.ErrStarGiftInvalid
	}
	var id int64
	err := s.db.QueryRow(ctx, `
WITH next_id AS (
    SELECT nextval(pg_get_serial_sequence('public.peer_star_gifts', 'id'))::bigint AS id
)
INSERT INTO peer_star_gifts (id, owner_peer_type, owner_peer_id, from_user_id, gift_id, msg_id, saved_id, gift_date, name_hidden, unsaved, converted, convert_stars, message)
SELECT next_id.id, $1,$2,$3,$4,$5,
       CASE WHEN $1 = 'channel' AND $6::bigint = 0 THEN next_id.id ELSE $6::bigint END,
       $7,$8,$9,false,$10,$11
FROM next_id
RETURNING id`,
		string(gift.Owner.Type), gift.Owner.ID, gift.FromUserID, gift.GiftID, gift.MsgID, gift.SavedID, gift.Date,
		gift.NameHidden, gift.Unsaved, gift.ConvertStars, gift.Message).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create star gift: %w", err)
	}
	return id, nil
}

func (s *StarGiftStore) ListByOwner(ctx context.Context, owner domain.Peer, excludeUnsaved bool, offset string, limit int) (domain.SavedStarGiftPage, error) {
	if !validStarGiftOwner(owner) {
		return domain.SavedStarGiftPage{}, nil
	}
	if limit <= 0 || limit > domain.MaxSavedStarGiftsLimit {
		limit = domain.MaxSavedStarGiftsLimit
	}
	// 总数（未转换 + 可选 excludeUnsaved 过滤）。
	countQuery := `SELECT COUNT(*) FROM peer_star_gifts WHERE owner_peer_type = $1 AND owner_peer_id = $2 AND NOT converted`
	if excludeUnsaved {
		countQuery += ` AND NOT unsaved`
	}
	var total int
	if err := s.db.QueryRow(ctx, countQuery, string(owner.Type), owner.ID).Scan(&total); err != nil {
		return domain.SavedStarGiftPage{}, fmt.Errorf("count star gifts: %w", err)
	}
	page := domain.SavedStarGiftPage{Count: total}

	where := "owner_peer_type = $1 AND owner_peer_id = $2 AND NOT converted"
	if excludeUnsaved {
		where += " AND NOT unsaved"
	}
	args := []any{string(owner.Type), owner.ID, limit + 1}
	if cursor, ok := domain.DecodeStarGiftCursor(offset); ok {
		where += " AND id < $4"
		args = append(args, cursor)
	}
	rows, err := s.db.Query(ctx, `
SELECT id, owner_peer_type, owner_peer_id, from_user_id, gift_id, msg_id, saved_id, gift_date, name_hidden, unsaved, converted, convert_stars, message
FROM peer_star_gifts
WHERE `+where+`
ORDER BY id DESC
LIMIT $3`, args...)
	if err != nil {
		return domain.SavedStarGiftPage{}, fmt.Errorf("list star gifts: %w", err)
	}
	defer rows.Close()
	gifts := make([]domain.SavedStarGift, 0, limit)
	for rows.Next() {
		g, err := scanSavedStarGift(rows)
		if err != nil {
			return domain.SavedStarGiftPage{}, err
		}
		gifts = append(gifts, g)
	}
	if err := rows.Err(); err != nil {
		return domain.SavedStarGiftPage{}, fmt.Errorf("iterate star gifts: %w", err)
	}
	if len(gifts) > limit {
		gifts = gifts[:limit]
		page.NextOffset = domain.EncodeStarGiftCursor(gifts[len(gifts)-1].ID)
	}
	page.Gifts = gifts
	return page, nil
}

func (s *StarGiftStore) GetByRef(ctx context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, bool, error) {
	if !ref.Valid() {
		return domain.SavedStarGift{}, false, nil
	}
	where, args := savedStarGiftRefWhere(ref)
	row := s.db.QueryRow(ctx, `
SELECT id, owner_peer_type, owner_peer_id, from_user_id, gift_id, msg_id, saved_id, gift_date, name_hidden, unsaved, converted, convert_stars, message
FROM peer_star_gifts
WHERE `+where, args...)
	g, err := scanSavedStarGift(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SavedStarGift{}, false, nil
		}
		return domain.SavedStarGift{}, false, err
	}
	return g, true, nil
}

func (s *StarGiftStore) CountByOwner(ctx context.Context, owner domain.Peer) (int, error) {
	if !validStarGiftOwner(owner) {
		return 0, nil
	}
	var n int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM peer_star_gifts WHERE owner_peer_type = $1 AND owner_peer_id = $2 AND NOT converted AND NOT unsaved`, string(owner.Type), owner.ID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count star gifts: %w", err)
	}
	return n, nil
}

func (s *StarGiftStore) SetUnsaved(ctx context.Context, ref domain.SavedStarGiftRef, unsaved bool) (bool, error) {
	if !ref.Valid() {
		return false, domain.ErrStarGiftNotFound
	}
	where, args := savedStarGiftRefWhere(ref)
	args = append(args, unsaved)
	tag, err := s.db.Exec(ctx, `
UPDATE peer_star_gifts SET unsaved = $4
WHERE `+where+` AND NOT converted`, args...)
	if err != nil {
		return false, fmt.Errorf("set star gift unsaved: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *StarGiftStore) MarkConverted(ctx context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, error) {
	if !ref.Valid() {
		return domain.SavedStarGift{}, domain.ErrStarGiftNotFound
	}
	out := domain.SavedStarGift{}
	err := withTx(ctx, s.db, "convert star gift", func(tx pgx.Tx) error {
		where, args := savedStarGiftRefWhere(ref)
		row := tx.QueryRow(ctx, `
SELECT id, owner_peer_type, owner_peer_id, from_user_id, gift_id, msg_id, saved_id, gift_date, name_hidden, unsaved, converted, convert_stars, message
FROM peer_star_gifts
WHERE `+where+` FOR UPDATE`, args...)
		g, err := scanSavedStarGift(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrStarGiftNotFound
			}
			return err
		}
		if g.Converted {
			return domain.ErrStarGiftAlreadyConverted
		}
		if _, err := tx.Exec(ctx, `UPDATE peer_star_gifts SET converted = true, unsaved = true WHERE id = $1`, g.ID); err != nil {
			return fmt.Errorf("mark star gift converted: %w", err)
		}
		g.Converted = true
		g.Unsaved = true
		out = g
		return nil
	})
	if err != nil {
		return domain.SavedStarGift{}, err
	}
	return out, nil
}

func scanSavedStarGift(row rowScanner) (domain.SavedStarGift, error) {
	var g domain.SavedStarGift
	var ownerType string
	if err := row.Scan(&g.ID, &ownerType, &g.Owner.ID, &g.FromUserID, &g.GiftID, &g.MsgID, &g.SavedID, &g.Date,
		&g.NameHidden, &g.Unsaved, &g.Converted, &g.ConvertStars, &g.Message); err != nil {
		return domain.SavedStarGift{}, err
	}
	g.Owner.Type = domain.PeerType(ownerType)
	return g, nil
}

func savedStarGiftRefWhere(ref domain.SavedStarGiftRef) (string, []any) {
	args := []any{string(ref.Owner.Type), ref.Owner.ID}
	switch ref.Owner.Type {
	case domain.PeerTypeChannel:
		args = append(args, ref.SavedID)
		return "owner_peer_type = $1 AND owner_peer_id = $2 AND saved_id = $3", args
	default:
		args = append(args, ref.MsgID)
		return "owner_peer_type = $1 AND owner_peer_id = $2 AND msg_id = $3", args
	}
}

func validSavedStarGift(g domain.SavedStarGift) bool {
	if g.GiftID == 0 || !validStarGiftOwner(g.Owner) {
		return false
	}
	switch g.Owner.Type {
	case domain.PeerTypeUser:
		return g.MsgID > 0 && g.SavedID == 0
	case domain.PeerTypeChannel:
		return g.MsgID == 0 && g.SavedID >= 0
	default:
		return false
	}
}

func validStarGiftOwner(owner domain.Peer) bool {
	return owner.ID != 0 && (owner.Type == domain.PeerTypeUser || owner.Type == domain.PeerTypeChannel)
}
