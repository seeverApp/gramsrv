package files

import (
	"context"
	"fmt"

	"telesrv/internal/domain"
)

// Star gift 目录：从已 seed 的 animated_emoji 集按 emoticon 精选贴纸文档合成（复用文档行与
// blob，不复制字节），镜像 EnsureDefaultEmojiStatusSet。目录是静态的，不入库。
// 礼物 ID 取明显隔离的常量段避免撞键。

const starGiftIDBase int64 = 8_888_000_000_000_000

type starGiftSeed struct {
	id       int64
	emoticon string
	stars    int64
	title    string
}

// starGiftSeeds 是固定礼物目录（emoticon 需在 animated_emoji 集里，否则该礼物被跳过）。
// convert_stars = stars（v1 全额转换，视作用新购 Stars 买入）。
var starGiftSeeds = []starGiftSeed{
	{starGiftIDBase + 1, "❤", 15, "Heart"},
	{starGiftIDBase + 2, "\U0001f382", 50, "Cake"},     // 🎂
	{starGiftIDBase + 3, "\U0001f389", 100, "Party"},   // 🎉
	{starGiftIDBase + 4, "\U0001f525", 250, "Fire"},    // 🔥
	{starGiftIDBase + 5, "\U0001f3c6", 500, "Trophy"},  // 🏆
	{starGiftIDBase + 6, "\U0001f48e", 1000, "Diamond"}, // 💎
	{starGiftIDBase + 7, "\U0001f680", 2500, "Rocket"}, // 🚀
}

// BuildStarGiftCatalog 合成可购买礼物目录：解析每个 seed emoticon 的贴纸文档，跳过未 seed 的。
// animated_emoji 未 seed 时返回空目录（客户端显示空礼物面板，购买流仍可对已知 gift_id 工作）。
func (s *Service) BuildStarGiftCatalog(ctx context.Context) ([]domain.StarGift, error) {
	source, found, err := s.media.GetStickerSetBySystemKey(ctx, "animated_emoji")
	if err != nil {
		return nil, fmt.Errorf("lookup animated_emoji set for star gifts: %w", err)
	}
	if !found || len(source.Packs) == 0 {
		return nil, nil
	}
	byEmoticon := make(map[string]int64, len(source.Packs))
	for _, pack := range source.Packs {
		key := normalizeStatusEmoticon(pack.Emoticon)
		if key == "" || len(pack.DocumentIDs) == 0 {
			continue
		}
		if _, ok := byEmoticon[key]; !ok {
			byEmoticon[key] = pack.DocumentIDs[0]
		}
	}
	// 收集要加载的文档 id（去重）。
	docIDs := make([]int64, 0, len(starGiftSeeds))
	chosen := make([]starGiftSeed, 0, len(starGiftSeeds))
	seen := make(map[int64]struct{})
	for _, seed := range starGiftSeeds {
		id, ok := byEmoticon[normalizeStatusEmoticon(seed.emoticon)]
		if !ok || id == 0 {
			continue
		}
		chosen = append(chosen, seed)
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			docIDs = append(docIDs, id)
		}
	}
	if len(chosen) == 0 {
		return nil, nil
	}
	docs, err := s.media.GetDocuments(ctx, docIDs)
	if err != nil {
		return nil, fmt.Errorf("load star gift sticker documents: %w", err)
	}
	docByID := make(map[int64]domain.Document, len(docs))
	for _, d := range docs {
		docByID[d.ID] = d
	}
	catalog := make([]domain.StarGift, 0, len(chosen))
	for _, seed := range chosen {
		id := byEmoticon[normalizeStatusEmoticon(seed.emoticon)]
		doc, ok := docByID[id]
		if !ok || doc.ID == 0 {
			continue
		}
		catalog = append(catalog, domain.StarGift{
			ID:           seed.id,
			Stars:        seed.stars,
			ConvertStars: seed.stars,
			Title:        seed.title,
			Sticker:      doc,
		})
	}
	return catalog, nil
}
