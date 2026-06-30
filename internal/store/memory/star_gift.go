package memory

import (
	"context"
	"sort"
	"sync"

	"telesrv/internal/domain"
)

// StarGiftStore 是 store.StarGiftStore 的内存实现。
type StarGiftStore struct {
	mu     sync.Mutex
	nextID int64
	gifts  []domain.SavedStarGift // 追加序
}

// NewStarGiftStore 创建内存 StarGiftStore。
func NewStarGiftStore() *StarGiftStore {
	return &StarGiftStore{}
}

func (s *StarGiftStore) Create(_ context.Context, gift domain.SavedStarGift) (int64, error) {
	if !validSavedStarGift(gift) {
		return 0, domain.ErrStarGiftInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	gift.ID = s.nextID
	if gift.Owner.Type == domain.PeerTypeChannel && gift.SavedID == 0 {
		gift.SavedID = gift.ID
	}
	gift.Converted = false
	s.gifts = append(s.gifts, gift)
	return gift.ID, nil
}

func (s *StarGiftStore) ListByOwner(_ context.Context, owner domain.Peer, excludeUnsaved bool, offset string, limit int) (domain.SavedStarGiftPage, error) {
	if !validStarGiftOwner(owner) {
		return domain.SavedStarGiftPage{}, nil
	}
	if limit <= 0 || limit > domain.MaxSavedStarGiftsLimit {
		limit = domain.MaxSavedStarGiftsLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]domain.SavedStarGift, 0)
	for _, g := range s.gifts {
		if g.Owner != owner || g.Converted {
			continue
		}
		if excludeUnsaved && g.Unsaved {
			continue
		}
		matched = append(matched, g)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })
	page := domain.SavedStarGiftPage{Count: len(matched)}
	cursor, hasCursor := domain.DecodeStarGiftCursor(offset)
	out := make([]domain.SavedStarGift, 0, limit)
	for _, g := range matched {
		if hasCursor && g.ID >= cursor {
			continue
		}
		out = append(out, g)
		if len(out) == limit {
			break
		}
	}
	if len(out) == limit {
		// 还有更早的则给下一页游标。
		last := out[len(out)-1].ID
		for _, g := range matched {
			if g.ID < last {
				page.NextOffset = domain.EncodeStarGiftCursor(last)
				break
			}
		}
	}
	page.Gifts = out
	return page, nil
}

func (s *StarGiftStore) GetByRef(_ context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, bool, error) {
	if !ref.Valid() {
		return domain.SavedStarGift{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.gifts {
		if savedStarGiftMatchesRef(g, ref) {
			return g, true, nil
		}
	}
	return domain.SavedStarGift{}, false, nil
}

func (s *StarGiftStore) CountByOwner(_ context.Context, owner domain.Peer) (int, error) {
	if !validStarGiftOwner(owner) {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, g := range s.gifts {
		if g.Owner == owner && !g.Converted && !g.Unsaved {
			n++
		}
	}
	return n, nil
}

func (s *StarGiftStore) SetUnsaved(_ context.Context, ref domain.SavedStarGiftRef, unsaved bool) (bool, error) {
	if !ref.Valid() {
		return false, domain.ErrStarGiftNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.gifts {
		if savedStarGiftMatchesRef(s.gifts[i], ref) && !s.gifts[i].Converted {
			s.gifts[i].Unsaved = unsaved
			return true, nil
		}
	}
	return false, nil
}

func (s *StarGiftStore) MarkConverted(_ context.Context, ref domain.SavedStarGiftRef) (domain.SavedStarGift, error) {
	if !ref.Valid() {
		return domain.SavedStarGift{}, domain.ErrStarGiftNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.gifts {
		if savedStarGiftMatchesRef(s.gifts[i], ref) {
			if s.gifts[i].Converted {
				return domain.SavedStarGift{}, domain.ErrStarGiftAlreadyConverted
			}
			s.gifts[i].Converted = true
			s.gifts[i].Unsaved = true
			return s.gifts[i], nil
		}
	}
	return domain.SavedStarGift{}, domain.ErrStarGiftNotFound
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

func savedStarGiftMatchesRef(g domain.SavedStarGift, ref domain.SavedStarGiftRef) bool {
	if g.Owner != ref.Owner {
		return false
	}
	switch ref.Owner.Type {
	case domain.PeerTypeUser:
		return g.MsgID == ref.MsgID
	case domain.PeerTypeChannel:
		return g.SavedID == ref.SavedID
	default:
		return false
	}
}
