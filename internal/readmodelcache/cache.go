// Package readmodelcache provides telesrv 统一的进程内 read-model 缓存原语。
//
// 它是 ~21 个手搓 read-model 缓存(map + mutex + TTL + 整表 flush + 偶尔 singleflight +
// 偶尔 epoch)收敛后的唯一实现。它**不是**通用响应缓存:Telegram 响应对 viewer 敏感,
// 可缓存单元仍是 docs/read-model-architecture.md 定义的 per-viewer 事实/投影。本类型只是
// 机制——键、TTL、版本 token 仍是各缓存自己的策略。
//
// 它替每个使用方免费保证(正是 2026-06-17 缓存审计发现的整类 bug):
//   - 有界:LRU 单条驱逐,绝不整表 flush(避免 thundering-herd 重载悬崖);
//   - 一致:跨越一次失效的 load 不会把陈旧值写回(epoch 守卫)——曾钉死版本脊与
//     ChannelMemberCache 的 lost-update;
//   - 去重:同键并发 miss 收敛为一次 load(singleflight);
//   - 可选版本闸门:仅当 storedHash == currentHash 时复用缓存项。
//
// epoch 刻意是**进程内**的:它只关掉进程内 load-vs-invalidate 竞态。跨实例新鲜度由
// NOTIFY 监听器(以及尚未落地的 durable 失效层)负责,本原语不改变跨进程契约。
package readmodelcache

import (
	"container/list"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Config 配置一个 Cache 实例。MaxEntries 是强制的:不允许构造无界缓存。
type Config[K comparable, V any] struct {
	// MaxEntries 是 LRU 上界。<=0 时 New 返回 nil(等价"禁用缓存",沿用各处
	// New*Cache(max<=0)->nil 的惯例;所有方法对 nil 安全,退化为直接 load)。
	MaxEntries int
	// TTL 仅作安全兜底(漏掉的带外写)。0 = 纯事件驱动,无时间过期。
	TTL time.Duration
	// Clone 在 store 与返回两个边界上对值做深拷贝,隔离调用方与缓存项的别名突变。
	// nil = 值本身 copy-safe(标量/扁平结构),不拷贝。
	Clone func(V) V
	// KeyString 生成 singleflight 键;nil 时默认 fmt.Sprint(K)。仅当 K 的 fmt 表示
	// 有歧义(可能两个不同键打印相同)时才需要自定义。
	KeyString func(K) string
	// Now 注入时钟,仅用于 TTL 过期判断;nil 时默认 time.Now。生产一律留空,
	// 测试可注入假时钟以确定地推进 TTL。
	Now func() time.Time
}

type lruEntry[K comparable, V any] struct {
	key      K
	value    V
	hash     int64
	expireAt time.Time // 零值 = 不过期
}

// Cache 是泛型 read-model 缓存。零值不可用,必须经 New 构造。nil *Cache 合法:
// 所有方法对 nil 安全,GetOrLoad 退化为直接调用 load。
type Cache[K comparable, V any] struct {
	mu        sync.Mutex
	ll        *list.List // LRU 顺序,Front=最近使用
	items     map[K]*list.Element
	cap       int
	ttl       time.Duration
	epoch     uint64
	sf        singleflight.Group
	clone     func(V) V
	keyString func(K) string
	now       func() time.Time
}

// New 构造一个 Cache。MaxEntries<=0 时返回 nil(禁用缓存,沿用既有惯例)。
func New[K comparable, V any](cfg Config[K, V]) *Cache[K, V] {
	if cfg.MaxEntries <= 0 {
		return nil
	}
	keyString := cfg.KeyString
	if keyString == nil {
		keyString = func(k K) string { return fmt.Sprint(k) }
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Cache[K, V]{
		ll:        list.New(),
		items:     make(map[K]*list.Element, initialMapHint(cfg.MaxEntries)),
		cap:       cfg.MaxEntries,
		ttl:       cfg.TTL,
		clone:     cfg.Clone,
		keyString: keyString,
		now:       now,
	}
}

func initialMapHint(max int) int {
	if max < 1024 {
		return max
	}
	return 1024
}

type loadResult[V any] struct {
	value  V
	stored bool
}

// GetOrLoad 命中即返回,否则经 singleflight 调 load。load 受 epoch 守卫:
// 在 load 前快照 epoch,若 load 期间发生过任何 Invalidate/Flush,则**拒绝**把这次
// load 的结果写回缓存,并重查/重载以取最新值(避免 lost-update 把陈旧值钉进缓存)。
func (c *Cache[K, V]) GetOrLoad(ctx context.Context, key K, load func() (V, error)) (V, error) {
	return c.getOrLoad(ctx, key, 0, false, load)
}

// GetOrLoadVersioned 在 GetOrLoad 基础上加版本闸门:仅当缓存项的 storedHash ==
// currentHash 时复用,否则重载。currentHash==0 表示"版本未知/绕过版本闸门"
// (与既有 snap.hash==currentHash 检查的语义一致)。
func (c *Cache[K, V]) GetOrLoadVersioned(ctx context.Context, key K, currentHash int64, load func() (V, error)) (V, error) {
	return c.getOrLoad(ctx, key, currentHash, true, load)
}

func (c *Cache[K, V]) getOrLoad(ctx context.Context, key K, currentHash int64, versioned bool, load func() (V, error)) (V, error) {
	if c == nil {
		return load()
	}
	for {
		if v, ok := c.lookup(key, currentHash, versioned); ok {
			return v, nil
		}
		res, err, _ := c.sf.Do(c.singleflightKey(key, currentHash, versioned), func() (any, error) {
			// 进入 singleflight 后再查一次:可能有并发者刚写入。
			if v, ok := c.lookup(key, currentHash, versioned); ok {
				return loadResult[V]{value: v, stored: true}, nil
			}
			loadEpoch := c.cacheEpoch()
			v, err := load()
			if err != nil {
				return loadResult[V]{}, err
			}
			stored := c.storeIfEpoch(key, v, currentHash, loadEpoch)
			return loadResult[V]{value: c.cloneValue(v), stored: stored}, nil
		})
		if err != nil {
			var zero V
			return zero, err
		}
		result := res.(loadResult[V])
		if result.stored {
			return result.value, nil
		}
		// store 被 epoch 守卫拒绝(load 期间发生过失效):本次 load 的值可能已陈旧。
		// 重查缓存——若有更新值(warm/后续 load)直接用,否则重新 load 取 DB 最新态。
		if err := ctx.Err(); err != nil {
			var zero V
			return zero, err
		}
	}
}

func (c *Cache[K, V]) lookup(key K, currentHash int64, versioned bool) (V, bool) {
	var zero V
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return zero, false
	}
	ent := el.Value.(*lruEntry[K, V])
	if c.expired(ent) {
		// 被动 TTL 过期:纯 per-key 删除,**不**自增 epoch——否则会误杀此刻在飞的
		// 不相关 load(epoch 是全局的),造成 thrash。
		c.removeElement(el)
		return zero, false
	}
	if versioned && currentHash != 0 && ent.hash != currentHash {
		return zero, false
	}
	c.ll.MoveToFront(el)
	return c.cloneValue(ent.value), true
}

// storeIfEpoch 仅在 epoch 未变(load 期间无失效)时写入,返回是否写入。
func (c *Cache[K, V]) storeIfEpoch(key K, v V, hash int64, loadEpoch uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epoch != loadEpoch {
		return false
	}
	c.storeLocked(key, v, hash)
	return true
}

func (c *Cache[K, V]) storeLocked(key K, v V, hash int64) {
	if el, ok := c.items[key]; ok {
		ent := el.Value.(*lruEntry[K, V])
		ent.value = c.cloneValue(v)
		ent.hash = hash
		ent.expireAt = c.expireAt()
		c.ll.MoveToFront(el)
		return
	}
	ent := &lruEntry[K, V]{key: key, value: c.cloneValue(v), hash: hash, expireAt: c.expireAt()}
	c.items[key] = c.ll.PushFront(ent)
	if c.ll.Len() > c.cap {
		c.evictOldest()
	}
}

// Store 把一个已在手的值写入缓存(warm-from-list 路径)。不自增 epoch:它不是失效,
// 不应取消其它在飞 load。
func (c *Cache[K, V]) Store(key K, v V) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.storeLocked(key, v, 0)
	c.mu.Unlock()
}

// LoadEpoch 在「外部构建值再写回」模式下,于构建前快照 epoch;之后用 StoreIfEpoch 写回。
// 适用于值在缓存之外构建(需要 ctx、多次往返)、无法套进 GetOrLoad 的调用方(如 RPC 投影)。
func (c *Cache[K, V]) LoadEpoch() uint64 {
	if c == nil {
		return 0
	}
	return c.cacheEpoch()
}

// StoreIfEpoch 仅在 epoch 自 loadEpoch 以来未变(构建期间没有失效)时写回外部构建的值。
// 与 LoadEpoch 配对,把 GetOrLoad 内建的 epoch 守卫开放给外部构建路径。
func (c *Cache[K, V]) StoreIfEpoch(key K, v V, loadEpoch uint64) {
	if c == nil {
		return
	}
	c.storeIfEpoch(key, v, 0, loadEpoch)
}

// StoreVersioned 同 Store,但带版本 hash(供版本闸门缓存的 warm 路径使用)。
func (c *Cache[K, V]) StoreVersioned(key K, hash int64, v V) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.storeLocked(key, v, hash)
	c.mu.Unlock()
}

// GetOrLoadBatch 一趟解析多个键:per-key 命中(TTL + 可选版本闸门)→ 把所有 miss 一次性
// 批量 load → per-key epoch 守卫写回。服务那些「按键查缓存、把 miss 合批打一次后端」的缓存
// (dialog-peer / privacy / photo / bot profile),避免单键 GetOrLoad 把一次批量 DB 退化成 N 次。
//
//   - versionOf(key) 返回 (hash, cacheable):cacheable=false 表示该键绕过缓存(永远重载、不写回,
//     如 dialog-peer 中 version 缺失的 peer);hash!=0 启用版本闸门(仅当 stored hash 匹配才复用);
//     hash==0 且 cacheable 表示纯 TTL 缓存(无版本,如 photo/privacy/bot)。
//   - loadMissing 必须为它收到的**每个** key 返回一个值(含「查过但不存在」的负缓存哨兵),
//     这样负结果也会被缓存,杜绝无结果键反复打后端。
//
// 若一次失效在批量 load 期间到达(epoch 变更),整趟重试,确保 pre-invalidation 的批量数据
// 不会遮蔽这次失效(比各缓存原先「静默拒绝写回但仍返回旧批数据」更强;ctx 取消兜底防自旋)。
func (c *Cache[K, V]) GetOrLoadBatch(
	ctx context.Context,
	keys []K,
	versionOf func(K) (hash int64, cacheable bool),
	loadMissing func(context.Context, []K) (map[K]V, error),
) (map[K]V, error) {
	if len(keys) == 0 {
		return map[K]V{}, nil
	}
	if c == nil {
		return loadMissing(ctx, dedupeKeys(keys))
	}
	for {
		out := make(map[K]V, len(keys))
		loadEpoch := c.cacheEpoch()
		// 在查阶段就把每个 miss 的 (hash, cacheable) 快照下来,写回时复用同一份——绝不在写回
		// 时重算 versionOf:否则一个版本在查与写之间变更的 key,会把按旧 hash 加载的值以新 hash
		// 戳入缓存,随后被当作新版数据命中(stale-as-fresh,旁路版本闸门)。
		missing := make([]batchMiss[K], 0, len(keys))
		seen := make(map[K]struct{}, len(keys))
		for _, key := range keys {
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			hash, cacheable := versionOf(key)
			if cacheable {
				if v, ok := c.lookup(key, hash, hash != 0); ok {
					out[key] = v
					continue
				}
			}
			missing = append(missing, batchMiss[K]{key: key, hash: hash, cacheable: cacheable})
		}
		if len(missing) == 0 {
			return out, nil
		}
		missingKeys := make([]K, len(missing))
		for i := range missing {
			missingKeys[i] = missing[i].key
		}
		loaded, err := loadMissing(ctx, missingKeys)
		if err != nil {
			return nil, err
		}
		if c.cacheEpoch() != loadEpoch {
			// 失效在批量 load 期间到达:重试整趟,避免用 pre-invalidation 数据遮蔽它。
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			continue
		}
		for _, m := range missing {
			v, ok := loaded[m.key]
			if !ok {
				continue
			}
			out[m.key] = v
			if m.cacheable {
				c.storeIfEpoch(m.key, v, m.hash, loadEpoch)
			}
		}
		return out, nil
	}
}

type batchMiss[K comparable] struct {
	key       K
	hash      int64
	cacheable bool
}

func dedupeKeys[K comparable](keys []K) []K {
	seen := make(map[K]struct{}, len(keys))
	out := make([]K, 0, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// Peek 直接读,不触发 load、不算 LRU touch(warm/测试路径)。过期项报 miss 但不删除
// (保持 Peek 无副作用)。
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	var zero V
	if c == nil {
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return zero, false
	}
	ent := el.Value.(*lruEntry[K, V])
	if c.expired(ent) {
		return zero, false
	}
	return c.cloneValue(ent.value), true
}

// Invalidate 删除指定键并自增 epoch(关掉任何此刻在飞 load 的写回)。
func (c *Cache[K, V]) Invalidate(keys ...K) {
	if c == nil || len(keys) == 0 {
		return
	}
	c.mu.Lock()
	c.epoch++
	for _, key := range keys {
		if el, ok := c.items[key]; ok {
			c.removeElement(el)
		}
	}
	c.mu.Unlock()
}

// InvalidateWhere 删除所有满足 pred 的键并自增 epoch(viewer/channel 维度扇出失效,
// 取代各缓存手写的 deleteChannel/InvalidateViewer 遍历)。
func (c *Cache[K, V]) InvalidateWhere(pred func(K) bool) {
	if c == nil || pred == nil {
		return
	}
	c.mu.Lock()
	c.epoch++
	for key, el := range c.items {
		if pred(key) {
			c.removeElement(el)
		}
	}
	c.mu.Unlock()
}

// Flush 清空缓存并自增 epoch(监听器断线重连兜底)。
func (c *Cache[K, V]) Flush() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.epoch++
	c.ll.Init()
	c.items = make(map[K]*list.Element, initialMapHint(c.cap))
	c.mu.Unlock()
}

// Len 返回当前缓存项数(测试/指标用)。
func (c *Cache[K, V]) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	n := c.ll.Len()
	c.mu.Unlock()
	return n
}

func (c *Cache[K, V]) cacheEpoch() uint64 {
	c.mu.Lock()
	e := c.epoch
	c.mu.Unlock()
	return e
}

func (c *Cache[K, V]) expired(ent *lruEntry[K, V]) bool {
	return c.ttl > 0 && !ent.expireAt.IsZero() && !ent.expireAt.After(c.now())
}

func (c *Cache[K, V]) expireAt() time.Time {
	if c.ttl <= 0 {
		return time.Time{}
	}
	return c.now().Add(c.ttl)
}

func (c *Cache[K, V]) evictOldest() {
	if el := c.ll.Back(); el != nil {
		c.removeElement(el)
	}
}

func (c *Cache[K, V]) removeElement(el *list.Element) {
	c.ll.Remove(el)
	delete(c.items, el.Value.(*lruEntry[K, V]).key)
}

func (c *Cache[K, V]) cloneValue(v V) V {
	if c.clone == nil {
		return v
	}
	return c.clone(v)
}

func (c *Cache[K, V]) singleflightKey(key K, hash int64, versioned bool) string {
	s := c.keyString(key)
	if versioned && hash != 0 {
		s += "@" + strconv.FormatInt(hash, 10)
	}
	return s
}
