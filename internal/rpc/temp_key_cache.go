package rpc

import (
	"container/list"
	"sync"
	"time"
)

const defaultTempKeyResolveCacheMaxEntries = 4096

type tempKeyResolveEntry struct {
	perm     [8]byte
	expireAt time.Time
}

type tempKeyResolveCacheItem struct {
	raw   [8]byte
	entry tempKeyResolveEntry
}

type tempKeyResolveCache struct {
	mu      sync.Mutex
	max     int
	entries map[[8]byte]*list.Element
	order   *list.List
}

func newTempKeyResolveCache(maxEntries int) *tempKeyResolveCache {
	if maxEntries <= 0 {
		maxEntries = defaultTempKeyResolveCacheMaxEntries
	}
	return &tempKeyResolveCache{
		max:     maxEntries,
		entries: make(map[[8]byte]*list.Element, maxEntries),
		order:   list.New(),
	}
}

func (c *tempKeyResolveCache) Get(rawAuthKeyID, expectedPermAuthKeyID [8]byte, now time.Time) ([8]byte, bool) {
	if c == nil {
		return [8]byte{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el := c.entries[rawAuthKeyID]
	if el == nil {
		return [8]byte{}, false
	}
	item := el.Value.(tempKeyResolveCacheItem)
	if !item.entry.expireAt.After(now) || item.entry.perm != expectedPermAuthKeyID {
		c.removeElementLocked(el)
		return [8]byte{}, false
	}
	c.order.MoveToBack(el)
	return item.entry.perm, true
}

func (c *tempKeyResolveCache) Store(rawAuthKeyID, permAuthKeyID [8]byte, expireAt, now time.Time) {
	if c == nil || c.max <= 0 || rawAuthKeyID == ([8]byte{}) || permAuthKeyID == ([8]byte{}) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.entries[rawAuthKeyID]; el != nil {
		el.Value = tempKeyResolveCacheItem{raw: rawAuthKeyID, entry: tempKeyResolveEntry{perm: permAuthKeyID, expireAt: expireAt}}
		c.order.MoveToBack(el)
		return
	}
	c.evictExpiredLocked(now)
	el := c.order.PushBack(tempKeyResolveCacheItem{raw: rawAuthKeyID, entry: tempKeyResolveEntry{perm: permAuthKeyID, expireAt: expireAt}})
	c.entries[rawAuthKeyID] = el
	for len(c.entries) > c.max {
		c.removeElementLocked(c.order.Front())
	}
}

func (c *tempKeyResolveCache) Delete(rawAuthKeyID [8]byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el := c.entries[rawAuthKeyID]; el != nil {
		c.removeElementLocked(el)
	}
}

func (c *tempKeyResolveCache) DeleteByPerm(permAuthKeyID [8]byte) [][8]byte {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	rawAuthKeyIDs := make([][8]byte, 0)
	for el := c.order.Front(); el != nil; {
		next := el.Next()
		item := el.Value.(tempKeyResolveCacheItem)
		if item.entry.perm == permAuthKeyID {
			rawAuthKeyIDs = append(rawAuthKeyIDs, item.raw)
			c.removeElementLocked(el)
		}
		el = next
	}
	return rawAuthKeyIDs
}

func (c *tempKeyResolveCache) evictExpiredLocked(now time.Time) {
	for el := c.order.Front(); el != nil; {
		next := el.Next()
		item := el.Value.(tempKeyResolveCacheItem)
		if !item.entry.expireAt.After(now) {
			c.removeElementLocked(el)
		}
		el = next
	}
}

func (c *tempKeyResolveCache) removeElementLocked(el *list.Element) {
	if el == nil {
		return
	}
	item := el.Value.(tempKeyResolveCacheItem)
	delete(c.entries, item.raw)
	c.order.Remove(el)
}
