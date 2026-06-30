package store

import "context"

// BoxIDAllocator 分配用户视角 message box id。box_id 允许空洞，但不能回退。
type BoxIDAllocator interface {
	NextBoxID(ctx context.Context, userID int64) (int, error)
	CurrentBoxID(ctx context.Context, userID int64) (int, error)
}

// CounterSource 用于 Redis 计数器冷启动时从 PostgreSQL durable log 恢复当前值。
type CounterSource interface {
	Current(ctx context.Context, userID int64) (int, error)
}
