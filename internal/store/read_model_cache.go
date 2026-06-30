package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"telesrv/internal/domain"
)

const (
	defaultReadModelHashCacheTTL = 30 * time.Minute
	defaultReadModelHashCacheMax = 65536
)

type readModelHashCacheEntry struct {
	hash     int64
	expireAt time.Time
}

type readModelHashInflight struct {
	done chan struct{}
	err  error
}

// CachedReadModelVersionStore caches read_model_versions hash tokens in-process.
// Correctness is driven by the same read-model NOTIFY stream that invalidates the
// heavier projection caches; TTL only bounds missed out-of-band writes.
type CachedReadModelVersionStore struct {
	base ReadModelVersionStore
	ttl  time.Duration
	max  int
	now  func() time.Time

	mu       sync.RWMutex
	m        map[ReadModelKey]readModelHashCacheEntry
	inflight map[ReadModelKey]*readModelHashInflight
	// epoch 在每次 invalidate/update/flush 时自增；一次锁外 DB load 若跨越了一次失效，
	// finishReadModelHashInflight 会拒绝把 stale hash 写回，避免 NOTIFY 送来的新 hash 被覆盖。
	// 与 contacts/privacy 读模缓存的 epoch 守卫同构。
	epoch uint64
}

func NewCachedReadModelVersionStore(base ReadModelVersionStore, ttl time.Duration, max int) *CachedReadModelVersionStore {
	if base == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = defaultReadModelHashCacheTTL
	}
	if max <= 0 {
		max = defaultReadModelHashCacheMax
	}
	return &CachedReadModelVersionStore{
		base:     base,
		ttl:      ttl,
		max:      max,
		now:      time.Now,
		m:        make(map[ReadModelKey]readModelHashCacheEntry, 1024),
		inflight: make(map[ReadModelKey]*readModelHashInflight),
	}
}

func (s *CachedReadModelVersionStore) ReadModelHash(ctx context.Context, model string, ownerUserID int64, peerType domain.PeerType, peerID int64) (int64, bool, error) {
	if s == nil || s.base == nil || model == "" {
		return 0, false, nil
	}
	key := ReadModelKey{Model: model, OwnerUserID: ownerUserID, PeerType: peerType, PeerID: peerID}
	rows, err := s.ReadModelHashes(ctx, []ReadModelKey{key})
	if err != nil {
		return 0, false, err
	}
	hash := rows[key]
	return hash, hash != 0, nil
}

func (s *CachedReadModelVersionStore) ReadModelHashes(ctx context.Context, keys []ReadModelKey) (map[ReadModelKey]int64, error) {
	out := make(map[ReadModelKey]int64, len(keys))
	if s == nil || s.base == nil || len(keys) == 0 {
		return out, nil
	}
	now := s.now()
	misses := make([]ReadModelKey, 0, len(keys))
	done := make(map[ReadModelKey]struct{}, len(keys))
	seen := make(map[ReadModelKey]struct{}, len(keys))

	s.mu.RLock()
	for _, key := range keys {
		if key.Model == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if entry, ok := s.m[key]; ok && entry.expireAt.After(now) {
			out[key] = entry.hash
			done[key] = struct{}{}
			continue
		}
		misses = append(misses, key)
	}
	s.mu.RUnlock()

	if len(misses) == 0 {
		return out, nil
	}
	sortReadModelKeys(misses)

	for len(done) < len(seen) {
		owned := make([]ReadModelKey, 0, len(misses))
		waiting := make(map[ReadModelKey]*readModelHashInflight)
		var loadEpoch uint64
		now = s.now()
		s.mu.Lock()
		loadEpoch = s.epoch
		for _, key := range misses {
			if _, ok := done[key]; ok {
				continue
			}
			if entry, ok := s.m[key]; ok && entry.expireAt.After(now) {
				out[key] = entry.hash
				done[key] = struct{}{}
				continue
			}
			if inflight := s.inflight[key]; inflight != nil {
				waiting[key] = inflight
				continue
			}
			inflight := &readModelHashInflight{done: make(chan struct{})}
			s.inflight[key] = inflight
			owned = append(owned, key)
		}
		s.mu.Unlock()
		if len(owned) == 0 && len(waiting) == 0 {
			break
		}
		if len(owned) > 0 {
			loaded, err := s.base.ReadModelHashes(ctx, owned)
			if err != nil {
				s.finishReadModelHashInflight(owned, nil, err, time.Time{}, loadEpoch)
				return nil, err
			}
			expireAt := s.now().Add(s.ttl)
			s.finishReadModelHashInflight(owned, loaded, nil, expireAt, loadEpoch)
			// 失效可能在 load 期间到达并写入更新的 hash；优先返回缓存里的当前值
			// (可能是 NOTIFY 刚写入的新 hash)，而不是这次 load 读到的可能已过期的值。
			effNow := s.now()
			s.mu.RLock()
			for _, key := range owned {
				if entry, ok := s.m[key]; ok && entry.expireAt.After(effNow) {
					out[key] = entry.hash
				} else {
					out[key] = loaded[key]
				}
				done[key] = struct{}{}
			}
			s.mu.RUnlock()
		}
		for key, inflight := range waiting {
			select {
			case <-inflight.done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if inflight.err != nil {
				return nil, inflight.err
			}
			s.mu.RLock()
			entry, ok := s.m[key]
			s.mu.RUnlock()
			if ok && entry.expireAt.After(s.now()) {
				out[key] = entry.hash
			}
			done[key] = struct{}{}
		}
	}
	return out, nil
}

func (s *CachedReadModelVersionStore) finishReadModelHashInflight(keys []ReadModelKey, loaded map[ReadModelKey]int64, err error, expireAt time.Time, loadEpoch uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil && s.epoch == loadEpoch {
		if len(s.m)+len(keys) > s.max {
			s.m = make(map[ReadModelKey]readModelHashCacheEntry, 1024)
		}
		if expireAt.IsZero() {
			expireAt = s.now().Add(s.ttl)
		}
		for _, key := range keys {
			hash := loaded[key]
			s.m[key] = readModelHashCacheEntry{hash: hash, expireAt: expireAt}
		}
	}
	for _, key := range keys {
		if inflight := s.inflight[key]; inflight != nil {
			inflight.err = err
			delete(s.inflight, key)
			close(inflight.done)
		}
	}
}

func (s *CachedReadModelVersionStore) InvalidateReadModel(key ReadModelKey) {
	if s == nil || key.Model == "" {
		return
	}
	s.mu.Lock()
	delete(s.m, key)
	s.epoch++
	s.mu.Unlock()
}

func (s *CachedReadModelVersionStore) UpdateReadModelHash(key ReadModelKey, hash int64) {
	if s == nil || key.Model == "" {
		return
	}
	if hash == 0 {
		s.InvalidateReadModel(key)
		return
	}
	s.mu.Lock()
	if len(s.m)+1 > s.max {
		s.m = make(map[ReadModelKey]readModelHashCacheEntry, 1024)
	}
	s.m[key] = readModelHashCacheEntry{hash: hash, expireAt: s.now().Add(s.ttl)}
	// 写入权威新 hash 后自增 epoch：任何此刻在飞的 load 都不得再用旧值覆盖它。
	s.epoch++
	s.mu.Unlock()
}

func (s *CachedReadModelVersionStore) FlushReadModelCache() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m = make(map[ReadModelKey]readModelHashCacheEntry, 1024)
	s.epoch++
	s.mu.Unlock()
}

func sortReadModelKeys(keys []ReadModelKey) {
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		if a.OwnerUserID != b.OwnerUserID {
			return a.OwnerUserID < b.OwnerUserID
		}
		if a.PeerType != b.PeerType {
			return a.PeerType < b.PeerType
		}
		return a.PeerID < b.PeerID
	})
}
