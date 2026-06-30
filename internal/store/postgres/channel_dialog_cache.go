package postgres

import (
	"context"

	"telesrv/internal/domain"
	"telesrv/internal/readmodelcache"
)

type channelDialogCacheKey struct {
	userID    int64
	channelID int64
}

// ChannelDialogCache 缓存 viewer 作用域的频道 dialog 投影,由统一缓存原语
// readmodelcache.Cache 承载(LRU 单条驱逐 / epoch 守卫 / singleflight 内建)。
//
// 仅在连接池(非事务)读路径使用。缓存值依赖 channel_base、channel_member(viewer,channel)、
// dialog_light(viewer,channel) 三个 read model;ReadModelChangeListener 在写侧 NOTIFY 时
// 失效对应键,重连时 flush。warm-from-list 经 put 回填(含 DefaultSendAs)。
type ChannelDialogCache struct {
	cache *readmodelcache.Cache[channelDialogCacheKey, domain.ChannelDialog]
}

func NewChannelDialogCache(max int) *ChannelDialogCache {
	cache := readmodelcache.New[channelDialogCacheKey, domain.ChannelDialog](readmodelcache.Config[channelDialogCacheKey, domain.ChannelDialog]{
		MaxEntries: max,
		Clone:      cloneChannelDialog,
	})
	if cache == nil {
		return nil
	}
	return &ChannelDialogCache{cache: cache}
}

func (c *ChannelDialogCache) get(userID, channelID int64) (domain.ChannelDialog, bool) {
	if c == nil || userID == 0 || channelID == 0 {
		return domain.ChannelDialog{}, false
	}
	return c.cache.Peek(channelDialogCacheKey{userID: userID, channelID: channelID})
}

func (c *ChannelDialogCache) getOrLoad(ctx context.Context, userID, channelID int64, load func() (domain.ChannelDialog, error)) (domain.ChannelDialog, error) {
	if c == nil || userID == 0 || channelID == 0 {
		return load()
	}
	return c.cache.GetOrLoad(ctx, channelDialogCacheKey{userID: userID, channelID: channelID}, load)
}

func (c *ChannelDialogCache) put(dialog domain.ChannelDialog) {
	if c == nil || dialog.UserID == 0 || dialog.ChannelID == 0 {
		return
	}
	c.cache.Store(channelDialogCacheKey{userID: dialog.UserID, channelID: dialog.ChannelID}, dialog)
}

// cacheEpoch 在「列表暖写回」前快照 epoch；配合 putIfEpoch 堵住 warm-vs-invalidation
// 竞态:DB 查询返回后到写回之间若收到失效(epoch 自增),陈旧投影写回被拒。
func (c *ChannelDialogCache) cacheEpoch() uint64 {
	if c == nil {
		return 0
	}
	return c.cache.LoadEpoch()
}

func (c *ChannelDialogCache) putIfEpoch(dialog domain.ChannelDialog, loadEpoch uint64) {
	if c == nil || dialog.UserID == 0 || dialog.ChannelID == 0 {
		return
	}
	c.cache.StoreIfEpoch(channelDialogCacheKey{userID: dialog.UserID, channelID: dialog.ChannelID}, dialog, loadEpoch)
}

func (c *ChannelDialogCache) delete(userID, channelID int64) {
	if c == nil || userID == 0 || channelID == 0 {
		return
	}
	c.cache.Invalidate(channelDialogCacheKey{userID: userID, channelID: channelID})
}

func (c *ChannelDialogCache) deleteChannel(channelID int64) {
	if c == nil || channelID == 0 {
		return
	}
	c.cache.InvalidateWhere(func(k channelDialogCacheKey) bool { return k.channelID == channelID })
}

func (c *ChannelDialogCache) flush() {
	if c == nil {
		return
	}
	c.cache.Flush()
}

func cloneChannelDialog(dialog domain.ChannelDialog) domain.ChannelDialog {
	if dialog.DefaultSendAs != nil {
		peer := *dialog.DefaultSendAs
		dialog.DefaultSendAs = &peer
	}
	return dialog
}
