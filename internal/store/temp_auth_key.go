package store

import (
	"context"

	"telesrv/internal/domain"
)

// TempAuthKeyBindingStore 持久化 auth.bindTempAuthKey 的 temp→perm 绑定。
type TempAuthKeyBindingStore interface {
	Save(ctx context.Context, binding domain.TempAuthKeyBinding) error
	GetByTemp(ctx context.Context, tempAuthKeyID [8]byte) (domain.TempAuthKeyBinding, bool, error)
	// DeleteExpired 回收过期早于 expiredBefore（unix 秒）的 temp 绑定，单次最多 limit 条，
	// 返回回收数。PFS temp key 定期轮换，无回收时绑定表无界堆积。
	// postgres 实现删除 auth_keys 中的 temp key 行（绑定经 ON DELETE CASCADE 一并清除），
	// 让过期 temp key 的入站帧立即失效；memory 替身仅删绑定。
	DeleteExpired(ctx context.Context, expiredBefore int64, limit int) (int, error)
}
