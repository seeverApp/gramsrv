// Package stargifts 实现 Star 礼物应用服务：礼物目录（从 seed 合成、懒加载缓存）+ peer 收到的
// 礼物实例 CRUD。扣费/退款/服务消息投递由 rpc 层编排（复用 Stars 账本 + SendPrivateText），
// 本层只管目录与持久化。
package stargifts

import (
	"context"
	"sync"

	"telesrv/internal/domain"
	"telesrv/internal/store"
)

// CatalogProvider 合成礼物目录（app/files 实现）。
type CatalogProvider interface {
	BuildStarGiftCatalog(ctx context.Context) ([]domain.StarGift, error)
}

// Service 是 Star 礼物应用服务。
type Service struct {
	store   store.StarGiftStore
	catalog CatalogProvider

	mu    sync.Mutex
	built bool
	gifts []domain.StarGift
	byID  map[int64]domain.StarGift
	hash  int
}

// NewService 创建 Star 礼物服务。
func NewService(st store.StarGiftStore, catalog CatalogProvider) *Service {
	return &Service{store: st, catalog: catalog}
}

// ensureCatalog 懒加载并缓存目录（静态数据，构建一次）。
func (s *Service) ensureCatalog(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.built {
		return nil
	}
	gifts, err := s.catalog.BuildStarGiftCatalog(ctx)
	if err != nil {
		return err
	}
	s.gifts = gifts
	s.byID = make(map[int64]domain.StarGift, len(gifts))
	for _, g := range gifts {
		s.byID[g.ID] = g
	}
	s.hash = domain.StarGiftCatalogHash(gifts)
	s.built = true
	return nil
}

// Catalog 返回礼物目录。
func (s *Service) Catalog(ctx context.Context) ([]domain.StarGift, error) {
	if err := s.ensureCatalog(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.StarGift, len(s.gifts))
	copy(out, s.gifts)
	return out, nil
}

// CatalogHash 返回目录 hash（getStarGifts NotModified 判定）。
func (s *Service) CatalogHash(ctx context.Context) (int, error) {
	if err := s.ensureCatalog(ctx); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hash, nil
}

// GiftByID 返回目录中指定礼物，不存在返回 ok=false。
func (s *Service) GiftByID(ctx context.Context, id int64) (domain.StarGift, bool, error) {
	if err := s.ensureCatalog(ctx); err != nil {
		return domain.StarGift{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.byID[id]
	return g, ok, nil
}

// RecordSavedGift 持久化一条收到的礼物实例，返回行 id。
func (s *Service) RecordSavedGift(ctx context.Context, gift domain.SavedStarGift) (int64, error) {
	return s.store.Create(ctx, gift)
}

// ListSaved 分页返回某 owner 收到的礼物。
func (s *Service) ListSaved(ctx context.Context, owner domain.Peer, excludeUnsaved bool, offset string, limit int) (domain.SavedStarGiftPage, error) {
	if len(offset) > domain.MaxStarGiftsOffsetBytes {
		offset = ""
	}
	if limit <= 0 || limit > domain.MaxSavedStarGiftsLimit {
		limit = domain.MaxSavedStarGiftsLimit
	}
	return s.store.ListByOwner(ctx, owner, excludeUnsaved, offset, limit)
}

// GetSaved 按协议引用取礼物实例。
func (s *Service) GetSaved(ctx context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, bool, error) {
	return s.store.GetByRef(ctx, ref)
}

// CountSaved 返回某 owner 展示在资料的礼物数（full.stargifts_count）。
func (s *Service) CountSaved(ctx context.Context, owner domain.Peer) (int, error) {
	return s.store.CountByOwner(ctx, owner)
}

// ToggleSaved 切换礼物在资料的展示（saveStarGift）。
func (s *Service) ToggleSaved(ctx context.Context, ref domain.SavedStarGiftRef, unsaved bool) (bool, error) {
	return s.store.SetUnsaved(ctx, ref, unsaved)
}

// Convert 把礼物标记为已转换（convertStarGift），返回该行供调用方据 ConvertStars 入账。
func (s *Service) Convert(ctx context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, error) {
	return s.store.MarkConverted(ctx, ref)
}
