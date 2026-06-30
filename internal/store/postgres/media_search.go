package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
)

// 共享媒体标签页读路径(迁移 0118):按类别查媒体索引拿到消息 id(JOIN 基表过滤软删 deleted),
// 再复用既有「按 id 取消息」路径(GetByIDs / GetChannelMessages)做富化与打包,并按索引顺序(id 倒序)重排。

const mediaSearchPageLimit = 100

func mediaCategoriesToInt16(cats []domain.MediaCategory) []int16 {
	out := make([]int16, 0, len(cats))
	for _, c := range cats {
		if c != domain.MediaCategoryNone {
			out = append(out, int16(c))
		}
	}
	return out
}

func mediaSearchPaging(req domain.MediaSearchRequest) (limit, offset int) {
	limit = req.Limit
	if limit < 0 || limit > mediaSearchPageLimit {
		limit = mediaSearchPageLimit
	}
	offset = req.AddOffset
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

type mediaSearchQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func mediaSearchCount(ctx context.Context, db mediaSearchQueryer, fallbackSQL string, args ...any) (int, error) {
	var count int
	if err := db.QueryRow(ctx, fallbackSQL, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func reorderMessagesByID(msgs []domain.Message, order []int) []domain.Message {
	byID := make(map[int]domain.Message, len(msgs))
	for _, m := range msgs {
		byID[m.ID] = m
	}
	out := make([]domain.Message, 0, len(order))
	for _, id := range order {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		}
	}
	return out
}

func reorderChannelMessagesByID(msgs []domain.ChannelMessage, order []int) []domain.ChannelMessage {
	byID := make(map[int]domain.ChannelMessage, len(msgs))
	for _, m := range msgs {
		byID[m.ID] = m
	}
	out := make([]domain.ChannelMessage, 0, len(order))
	for _, id := range order {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		}
	}
	return out
}

// SearchPrivateMedia 返回某私聊会话中属于给定类别的消息(newest-first 分页)。
func (s *MessageStore) SearchPrivateMedia(ctx context.Context, ownerUserID, peerID int64, req domain.MediaSearchRequest) (domain.MessageList, error) {
	cats := mediaCategoriesToInt16(req.Categories)
	if ownerUserID == 0 || peerID == 0 || len(cats) == 0 {
		return domain.MessageList{}, nil
	}
	limit, offset := mediaSearchPaging(req)
	maxID, minID, offsetID := int32(req.MaxID), int32(req.MinID), int32(req.OffsetID)

	count := req.KnownCount
	if !req.HasKnownCount {
		var err error
		count, err = mediaSearchCount(ctx, s.db, `
SELECT count(DISTINCT mi.box_id)::int
FROM message_box_media mi
JOIN message_boxes mb ON mb.owner_user_id = mi.owner_user_id AND mb.box_id = mi.box_id
WHERE mi.owner_user_id = $1 AND mi.peer_id = $2 AND mi.category = ANY($3::smallint[])
  AND NOT mb.deleted
  AND ($4 = 0 OR mi.box_id <= $4)
  AND ($5 = 0 OR mi.box_id >= $5)`, ownerUserID, peerID, cats, maxID, minID)
		if err != nil {
			return domain.MessageList{}, fmt.Errorf("count private media: %w", err)
		}
	}
	if limit == 0 {
		return domain.MessageList{Count: count}, nil
	}
	rows, err := s.db.Query(ctx, `
SELECT DISTINCT mi.box_id
FROM message_box_media mi
JOIN message_boxes mb ON mb.owner_user_id = mi.owner_user_id AND mb.box_id = mi.box_id
WHERE mi.owner_user_id = $1 AND mi.peer_id = $2 AND mi.category = ANY($3::smallint[])
  AND NOT mb.deleted
  AND ($4 = 0 OR mi.box_id <= $4)
  AND ($5 = 0 OR mi.box_id >= $5)
  AND ($6 = 0 OR mi.box_id < $6)
ORDER BY mi.box_id DESC
OFFSET $7 LIMIT $8`, ownerUserID, peerID, cats, maxID, minID, offsetID, offset, limit)
	if err != nil {
		return domain.MessageList{}, fmt.Errorf("list private media ids: %w", err)
	}
	defer rows.Close()
	ids := make([]int, 0, limit)
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return domain.MessageList{}, fmt.Errorf("scan private media id: %w", err)
		}
		ids = append(ids, int(id))
	}
	if err := rows.Err(); err != nil {
		return domain.MessageList{}, fmt.Errorf("iterate private media ids: %w", err)
	}

	list, err := s.GetByIDs(ctx, ownerUserID, ids)
	if err != nil {
		return domain.MessageList{}, err
	}
	list.Messages = reorderMessagesByID(list.Messages, ids)
	list.Count = count
	return list, nil
}

// CountPrivateMediaCategories 返回某私聊会话按基础媒体类别聚合的精确计数。
func (s *MessageStore) CountPrivateMediaCategories(ctx context.Context, ownerUserID, peerID int64) (domain.MediaCategoryCounts, error) {
	if ownerUserID == 0 || peerID == 0 {
		return domain.MediaCategoryCounts{}, nil
	}
	rows, err := s.db.Query(ctx, `
SELECT category, media_count
FROM private_media_category_counts
WHERE owner_user_id = $1 AND peer_id = $2 AND media_count > 0`, ownerUserID, peerID)
	if err != nil {
		return nil, fmt.Errorf("count private media categories: %w", err)
	}
	defer rows.Close()
	out := domain.MediaCategoryCounts{}
	for rows.Next() {
		var category int16
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("scan private media category count: %w", err)
		}
		out[domain.MediaCategory(category)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate private media category counts: %w", err)
	}
	return out, nil
}

// SearchChannelMedia 返回某频道中属于给定类别的消息(newest-first 分页)。
func (s *ChannelStore) SearchChannelMedia(ctx context.Context, viewerUserID, channelID int64, req domain.MediaSearchRequest) (domain.ChannelHistory, error) {
	cats := mediaCategoriesToInt16(req.Categories)
	if viewerUserID == 0 || channelID == 0 || len(cats) == 0 {
		return domain.ChannelHistory{}, nil
	}
	limit, offset := mediaSearchPaging(req)
	maxID, minID, offsetID := int32(req.MaxID), int32(req.MinID), int32(req.OffsetID)

	channel, member, err := s.getChannelForMember(ctx, s.db, viewerUserID, channelID)
	if err != nil {
		return domain.ChannelHistory{}, err
	}
	count := req.KnownCount
	if !req.HasKnownCount {
		var err error
		count, err = mediaSearchCount(ctx, s.db, `
SELECT count(DISTINCT mi.id)::int
FROM channel_message_media mi
JOIN channel_messages m ON m.channel_id = mi.channel_id AND m.id = mi.id
WHERE mi.channel_id = $1 AND mi.category = ANY($2::smallint[])
  AND NOT m.deleted
  AND ($3 <= 0 OR mi.id > $3)
  AND ($4 = 0 OR mi.id <= $4)
  AND ($5 = 0 OR mi.id >= $5)`, channelID, cats, int32(member.AvailableMinID), maxID, minID)
		if err != nil {
			return domain.ChannelHistory{}, fmt.Errorf("count channel media: %w", err)
		}
	}
	if limit == 0 {
		return domain.ChannelHistory{Channel: channel, Self: member, Count: count}, nil
	}
	rows, err := s.db.Query(ctx, `
SELECT DISTINCT mi.id
FROM channel_message_media mi
JOIN channel_messages m ON m.channel_id = mi.channel_id AND m.id = mi.id
WHERE mi.channel_id = $1 AND mi.category = ANY($2::smallint[])
  AND NOT m.deleted
  AND ($3 <= 0 OR mi.id > $3)
  AND ($4 = 0 OR mi.id <= $4)
  AND ($5 = 0 OR mi.id >= $5)
  AND ($6 = 0 OR mi.id < $6)
ORDER BY mi.id DESC
OFFSET $7 LIMIT $8`, channelID, cats, int32(member.AvailableMinID), maxID, minID, offsetID, offset, limit)
	if err != nil {
		return domain.ChannelHistory{}, fmt.Errorf("list channel media ids: %w", err)
	}
	defer rows.Close()
	ids := make([]int, 0, limit)
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			return domain.ChannelHistory{}, fmt.Errorf("scan channel media id: %w", err)
		}
		ids = append(ids, int(id))
	}
	if err := rows.Err(); err != nil {
		return domain.ChannelHistory{}, fmt.Errorf("iterate channel media ids: %w", err)
	}

	hist, err := s.getChannelMessagesForMember(ctx, viewerUserID, channel, member, ids)
	if err != nil {
		return domain.ChannelHistory{}, err
	}
	hist.Messages = reorderChannelMessagesByID(hist.Messages, ids)
	hist.Count = count
	return hist, nil
}

// CountChannelMediaCategories 返回某频道对当前 viewer 可见消息按基础媒体类别聚合的精确计数。
func (s *ChannelStore) CountChannelMediaCategories(ctx context.Context, viewerUserID, channelID int64) (domain.MediaCategoryCounts, error) {
	if viewerUserID == 0 || channelID == 0 {
		return domain.MediaCategoryCounts{}, nil
	}
	var availableMinID int32
	if err := s.db.QueryRow(ctx, `
SELECT available_min_id
FROM channel_members
WHERE channel_id = $1 AND user_id = $2
  AND status = 'active'
  AND NOT COALESCE((banned_rights->>'ViewMessages')::boolean, false)`, channelID, viewerUserID).Scan(&availableMinID); err != nil {
		if err == pgx.ErrNoRows {
			return domain.MediaCategoryCounts{}, nil
		}
		return nil, fmt.Errorf("load channel media count member visibility: %w", err)
	}
	if availableMinID <= 0 {
		return s.countFullChannelMediaCategories(ctx, channelID)
	}
	rows, err := s.db.Query(ctx, `
SELECT mi.category, count(*)::int
FROM channel_message_media mi
JOIN channel_messages m ON m.channel_id = mi.channel_id AND m.id = mi.id
WHERE mi.channel_id = $1
  AND NOT m.deleted
  AND mi.id > $2
GROUP BY mi.category`, channelID, availableMinID)
	if err != nil {
		return nil, fmt.Errorf("count channel media categories: %w", err)
	}
	defer rows.Close()
	out := domain.MediaCategoryCounts{}
	for rows.Next() {
		var category int16
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("scan channel media category count: %w", err)
		}
		out[domain.MediaCategory(category)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel media category counts: %w", err)
	}
	return out, nil
}

func (s *ChannelStore) countFullChannelMediaCategories(ctx context.Context, channelID int64) (domain.MediaCategoryCounts, error) {
	rows, err := s.db.Query(ctx, `
SELECT category, media_count
FROM channel_media_category_counts
WHERE channel_id = $1 AND media_count > 0`, channelID)
	if err != nil {
		return nil, fmt.Errorf("count full channel media categories: %w", err)
	}
	defer rows.Close()
	out := domain.MediaCategoryCounts{}
	for rows.Next() {
		var category int16
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("scan full channel media category count: %w", err)
		}
		out[domain.MediaCategory(category)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate full channel media category counts: %w", err)
	}
	return out, nil
}
