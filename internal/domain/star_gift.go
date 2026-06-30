package domain

import (
	"encoding/base64"
	"errors"
	"strconv"
)

// Star gift（payments.sendStarsForm + inputInvoiceStarGift）领域模型。目录是从已 seed 的
// animated_emoji 合成的静态集合（StarGift）；peer 收到的礼物实例落 peer_star_gifts（SavedStarGift）。
// 与 Stars 账本配合：发礼 Debit、转换回 Stars 时 Credit。

// StarGift 是一个可购买礼物目录项（合成、非用户持有）。
type StarGift struct {
	ID           int64
	Stars        int64    // 购买价（Stars）
	ConvertStars int64    // 收礼人可转换回的 Stars（v1 = Stars，全额）
	Title        string   // 可选标题
	Sticker      Document // 礼物贴纸快照（tg 投影必须是带 sticker 属性的有效 Document，否则客户端丢弃）
}

// SavedStarGift 是一条已收到的礼物实例（peer_star_gifts 一行）。
type SavedStarGift struct {
	ID           int64
	Owner        Peer   // 收礼 peer（user/channel）
	FromUserID   int64  // 送礼人（匿名也保留真实值供账本，下发时按 NameHidden 决定是否暴露）
	GiftID       int64  // → StarGift.ID
	MsgID        int    // 用户礼物的私聊 msg_id；频道礼物不进历史，固定为 0
	SavedID      int64  // 频道礼物 inputSavedStarGiftChat.saved_id；用户礼物为 0
	Date         int    // 收到时刻 Unix 秒
	NameHidden   bool   // 送礼人请求隐藏姓名
	Unsaved      bool   // 未展示在个人资料（saveStarGift 切换）
	Converted    bool   // 已转换回 Stars（终态，从列表排除）
	ConvertStars int64  // 转换可退回的 Stars
	Message      string // 附言（可选）
}

// SavedStarGiftRef 是 payments.getSavedStarGift/saveStarGift/convertStarGift 的协议中立引用。
// 用户礼物使用 inputSavedStarGiftUser.msg_id；频道礼物使用 inputSavedStarGiftChat.peer + saved_id。
type SavedStarGiftRef struct {
	Owner   Peer
	MsgID   int
	SavedID int64
}

// Valid reports whether the reference has the identity required by its owner kind.
func (r SavedStarGiftRef) Valid() bool {
	switch r.Owner.Type {
	case PeerTypeUser:
		return r.Owner.ID != 0 && r.MsgID > 0
	case PeerTypeChannel:
		return r.Owner.ID != 0 && r.SavedID > 0
	default:
		return false
	}
}

// SavedStarGiftPage 是一页已收到礼物 + keyset 分页游标。
type SavedStarGiftPage struct {
	Gifts      []SavedStarGift
	NextOffset string // 空 = 无更多页（末页必须省略，客户端据此停止翻页）
	Count      int    // 总数（未转换、按 excludeUnsaved 过滤后）
}

// Star gift 边界常量。
const (
	// MaxSavedStarGiftsLimit 是 getSavedStarGifts 单页上限。
	MaxSavedStarGiftsLimit = 100
	// MaxStarGiftMessageRunes 限制附言长度（对齐 stargifts_message_length_max 量级）。
	MaxStarGiftMessageRunes = 255
	// MaxStarGiftsOffsetBytes 是 keyset 游标字符串长度上限。
	MaxStarGiftsOffsetBytes = 64
)

// Star gift 哨兵错误（rpc 层 errors.Is 映射为 tgerr）。
var (
	// ErrStarGiftInvalid 表示礼物 id 不在目录里。
	ErrStarGiftInvalid = errors.New("stargift: invalid gift id")
	// ErrStarGiftNotFound 表示找不到该已收到礼物实例。
	ErrStarGiftNotFound = errors.New("stargift: saved gift not found")
	// ErrStarGiftAlreadyConverted 表示礼物已转换回 Stars（不可重复转换）。
	ErrStarGiftAlreadyConverted = errors.New("stargift: already converted")
)

// StarGiftCatalogHash 由目录的 (gift_id, stars) 折叠出稳定 hash，供 getStarGifts NotModified。
func StarGiftCatalogHash(catalog []StarGift) int {
	var h uint64
	for _, g := range catalog {
		h ^= uint64(g.ID)
		h = h*0x4f25 + uint64(g.ID)
		h = h*0x4f25 + uint64(g.Stars)
	}
	return int(h & 0x7fffffff)
}

// EncodeStarGiftCursor / DecodeStarGiftCursor 是 saved gifts keyset 游标（最后一条实例 id）。
func EncodeStarGiftCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

// DecodeStarGiftCursor 反解游标；无法解析（含空串）返回 ok=false（调用方从首页开始）。
func DecodeStarGiftCursor(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
