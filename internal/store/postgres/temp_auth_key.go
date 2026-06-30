package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
	"telesrv/internal/store/postgres/sqlcgen"
)

// TempAuthKeyBindingStore 用 PostgreSQL 实现 store.TempAuthKeyBindingStore。
type TempAuthKeyBindingStore struct {
	q *sqlcgen.Queries
}

// NewTempAuthKeyBindingStore 基于 pgx 连接池（或事务）创建 TempAuthKeyBindingStore。
func NewTempAuthKeyBindingStore(db sqlcgen.DBTX) *TempAuthKeyBindingStore {
	return &TempAuthKeyBindingStore{q: sqlcgen.New(db)}
}

func (s *TempAuthKeyBindingStore) Save(ctx context.Context, b domain.TempAuthKeyBinding) error {
	if err := s.q.UpsertTempAuthKeyBinding(ctx, sqlcgen.UpsertTempAuthKeyBindingParams{
		TempAuthKeyID:    authKeyIDToInt64(b.TempAuthKeyID),
		PermAuthKeyID:    b.PermAuthKeyID,
		Nonce:            b.Nonce,
		TempSessionID:    b.TempSessionID,
		ExpiresAt:        int32(b.ExpiresAt),
		EncryptedMessage: b.EncryptedMessage,
	}); err != nil {
		return fmt.Errorf("upsert temp auth key binding: %w", err)
	}
	return nil
}

// DeleteExpired 实现 store.TempAuthKeyBindingStore：删除 auth_keys 中过期的 temp key，
// temp_auth_key_bindings 经 ON DELETE CASCADE 一并清除，过期 key 的入站帧随之失效。
func (s *TempAuthKeyBindingStore) DeleteExpired(ctx context.Context, expiredBefore int64, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	n, err := s.q.DeleteExpiredTempAuthKeys(ctx, sqlcgen.DeleteExpiredTempAuthKeysParams{
		ExpiresAt: int32(expiredBefore),
		Limit:     int32(limit),
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired temp auth keys: %w", err)
	}
	return int(n), nil
}

func (s *TempAuthKeyBindingStore) GetByTemp(ctx context.Context, tempAuthKeyID [8]byte) (domain.TempAuthKeyBinding, bool, error) {
	row, err := s.q.GetTempAuthKeyBinding(ctx, authKeyIDToInt64(tempAuthKeyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TempAuthKeyBinding{}, false, nil
		}
		return domain.TempAuthKeyBinding{}, false, fmt.Errorf("get temp auth key binding: %w", err)
	}
	return domain.TempAuthKeyBinding{
		TempAuthKeyID:    authKeyIDFromInt64(row.TempAuthKeyID),
		PermAuthKeyID:    row.PermAuthKeyID,
		Nonce:            row.Nonce,
		TempSessionID:    row.TempSessionID,
		ExpiresAt:        int(row.ExpiresAt),
		EncryptedMessage: append([]byte(nil), row.EncryptedMessage...),
	}, true, nil
}
