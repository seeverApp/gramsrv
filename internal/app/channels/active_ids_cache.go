package channels

import (
	"context"
	"time"

	"telesrv/internal/app/readmodel"
	"telesrv/internal/domain"
	"telesrv/internal/readmodelcache"
)

const (
	defaultActiveChannelIDsReadModelTTL = 24 * time.Hour
	activeChannelIDsReadModelMaxEntries = 8192
	activeChannelIDsNoVersionHash       = -1
)

type activeChannelIDsCacheKey struct {
	userID         int64
	afterChannelID int64
	limit          int
}

// activeChannelIDsReadModelCache 由统一缓存原语承载(版本闸门 / epoch 守卫 / LRU / clone)。
// 无版本(version 缺失)时用 activeChannelIDsNoVersionHash 哨兵作版本,仍享 TTL+epoch 失效。
type activeChannelIDsReadModelCache struct {
	cache *readmodelcache.Cache[activeChannelIDsCacheKey, []int64]
}

func newActiveChannelIDsReadModelCache(ttl time.Duration) *activeChannelIDsReadModelCache {
	if ttl <= 0 {
		ttl = defaultActiveChannelIDsReadModelTTL
	}
	return &activeChannelIDsReadModelCache{
		cache: readmodelcache.New[activeChannelIDsCacheKey, []int64](readmodelcache.Config[activeChannelIDsCacheKey, []int64]{
			MaxEntries: activeChannelIDsReadModelMaxEntries,
			TTL:        ttl,
			Clone:      cloneInt64s,
		}),
	}
}

func (c *activeChannelIDsReadModelCache) getOrLoad(ctx context.Context, key activeChannelIDsCacheKey, hash int64, load func() ([]int64, error)) ([]int64, error) {
	if c == nil {
		return load()
	}
	return c.cache.GetOrLoadVersioned(ctx, key, hash, load)
}

func (c *activeChannelIDsReadModelCache) invalidateUsers(userIDs ...int64) {
	if c == nil || len(userIDs) == 0 {
		return
	}
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID != 0 {
			seen[userID] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return
	}
	c.cache.InvalidateWhere(func(k activeChannelIDsCacheKey) bool {
		_, ok := seen[k.userID]
		return ok
	})
}

func (s *Service) cachedActiveChannelIDsForUser(ctx context.Context, userID, afterChannelID int64, limit int) ([]int64, error) {
	if s.activeIDsCache == nil || s.versions == nil {
		return s.channels.ListActiveChannelIDsForUser(ctx, userID, afterChannelID, limit)
	}
	hash, ok, err := s.versions.ReadModelHash(ctx, readmodel.ModelChannelActiveIDs, userID, domain.PeerTypeUser, userID)
	if err != nil {
		return nil, err
	}
	if !ok || hash == 0 {
		hash = activeChannelIDsNoVersionHash
	}
	key := activeChannelIDsCacheKey{userID: userID, afterChannelID: afterChannelID, limit: limit}
	return s.activeIDsCache.getOrLoad(ctx, key, hash, func() ([]int64, error) {
		return s.channels.ListActiveChannelIDsForUser(ctx, userID, afterChannelID, limit)
	})
}

func cloneInt64s(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	return append([]int64(nil), in...)
}
