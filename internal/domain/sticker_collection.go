package domain

import "errors"

// ErrStickerInvalid 表示输入文档不是合法的贴纸/GIF（faveSticker/saveGif 等）。
var ErrStickerInvalid = errors.New("sticker document invalid")

// StickerCollectionKind 区分一个用户的几类个人贴纸集合。
type StickerCollectionKind string

const (
	// StickerCollectionFaved 收藏的贴纸（messages.faveSticker / getFavedStickers）。
	StickerCollectionFaved StickerCollectionKind = "faved"
	// StickerCollectionRecent 最近使用的贴纸（saveRecentSticker / getRecentStickers）。
	StickerCollectionRecent StickerCollectionKind = "recent"
	// StickerCollectionRecentAttached attach 菜单里最近用于媒体的贴纸（attached=true）。
	StickerCollectionRecentAttached StickerCollectionKind = "recent_attached"
	// StickerCollectionGif 保存的 GIF（messages.saveGif / getSavedGifs）。
	StickerCollectionGif StickerCollectionKind = "gif"
)

// 各集合容量上界（最新在前，超出截断最旧）。仅为限制存储增长，非严格 Telegram 配额；
// faved 不做 premium 分档（用户决策范围外），统一一个上界。
const (
	MaxFavedStickers  = 100
	MaxRecentStickers = 30
	MaxSavedGifs      = 200
)

// MaxStickerCollectionItems 返回某类集合的容量上界。
func MaxStickerCollectionItems(kind StickerCollectionKind) int {
	switch kind {
	case StickerCollectionFaved:
		return MaxFavedStickers
	case StickerCollectionGif:
		return MaxSavedGifs
	default:
		return MaxRecentStickers
	}
}

// StickerCollectionItem 是集合内一项：文档 id + 入列时间（recent 的 used_at / faved 的收藏时刻）。
type StickerCollectionItem struct {
	DocumentID int64
	Date       int
}
