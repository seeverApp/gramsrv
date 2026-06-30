package store

import (
	"context"

	"telesrv/internal/domain"
)

// StarGiftStore 持久化 peer 收到的 Star 礼物实例（peer_star_gifts）。礼物目录是合成的
// 内存集合，不在此存储。
type StarGiftStore interface {
	// Create 写一条收到的礼物实例，返回行 id；频道礼物未显式给 saved_id 时以该行 id 作为 saved_id。
	Create(ctx context.Context, gift domain.SavedStarGift) (int64, error)
	// ListByOwner 按 id DESC keyset 分页返回某 owner 未转换的礼物；excludeUnsaved 时只返展示在资料的。
	ListByOwner(ctx context.Context, owner domain.Peer, excludeUnsaved bool, offset string, limit int) (domain.SavedStarGiftPage, error)
	// GetByRef 按协议引用取礼物实例：用户用 msg_id，频道用 saved_id。
	GetByRef(ctx context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, bool, error)
	// CountByOwner 返回某 owner 展示在资料的礼物数（非转换、非隐藏），供 full.stargifts_count。
	CountByOwner(ctx context.Context, owner domain.Peer) (int, error)
	// SetUnsaved 切换礼物在资料的展示（saveStarGift）；返回是否命中一行。
	SetUnsaved(ctx context.Context, ref domain.SavedStarGiftRef, unsaved bool) (bool, error)
	// MarkConverted 幂等地把礼物标记为已转换（convertStarGift），返回该行供调用方据 ConvertStars
	// 入账；已转换返回 domain.ErrStarGiftAlreadyConverted，不存在返回 domain.ErrStarGiftNotFound。
	MarkConverted(ctx context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, error)
}
