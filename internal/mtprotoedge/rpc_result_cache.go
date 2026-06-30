package mtprotoedge

import (
	"container/list"
	"sync"
	"time"
)

const (
	rpcResultCacheTTL        = 3 * time.Minute
	rpcResultCacheMaxEntries = 4096
	rpcResultCacheMaxBytes   = 64 << 20
)

type rpcResultCacheKey struct {
	authKeyID [8]byte
	sessionID int64
	reqMsgID  int64
}

type rpcResultCacheEntry struct {
	key       rpcResultCacheKey
	encoded   *encodedOutboundMessage
	size      int
	expiresAt time.Time
}

type rpcResultCache struct {
	mu         sync.Mutex
	now        func() time.Time
	ttl        time.Duration
	maxEntries int
	maxBytes   int
	bytes      int
	order      *list.List
	byKey      map[rpcResultCacheKey]*list.Element
}

func newRPCResultCache(now func() time.Time) *rpcResultCache {
	if now == nil {
		now = time.Now
	}
	return &rpcResultCache{
		now:        now,
		ttl:        rpcResultCacheTTL,
		maxEntries: rpcResultCacheMaxEntries,
		maxBytes:   rpcResultCacheMaxBytes,
		order:      list.New(),
		byKey:      make(map[rpcResultCacheKey]*list.Element),
	}
}

func (c *rpcResultCache) Get(authKeyID [8]byte, sessionID, reqMsgID int64) (*encodedOutboundMessage, bool) {
	if c == nil || reqMsgID == 0 {
		return nil, false
	}
	key := rpcResultCacheKey{authKeyID: authKeyID, sessionID: sessionID, reqMsgID: reqMsgID}
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.byKey[key]
	if !ok {
		return nil, false
	}
	entry := elem.Value.(*rpcResultCacheEntry)
	if !entry.expiresAt.After(now) {
		c.removeElement(elem)
		return nil, false
	}
	return cloneEncodedOutboundMessage(entry.encoded), true
}

func (c *rpcResultCache) Put(authKeyID [8]byte, sessionID, reqMsgID int64, encoded *encodedOutboundMessage) {
	if c == nil || reqMsgID == 0 || encoded == nil {
		return
	}
	copied := cloneEncodedOutboundMessage(encoded)
	if copied == nil {
		return
	}
	size := len(copied.body)
	if c.maxBytes > 0 && size > c.maxBytes {
		return
	}

	key := rpcResultCacheKey{authKeyID: authKeyID, sessionID: sessionID, reqMsgID: reqMsgID}
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.expireLocked(now)
	if elem, ok := c.byKey[key]; ok {
		c.removeElement(elem)
	}
	entry := &rpcResultCacheEntry{
		key:       key,
		encoded:   copied,
		size:      size,
		expiresAt: now.Add(c.ttl),
	}
	elem := c.order.PushBack(entry)
	c.byKey[key] = elem
	c.bytes += size
	c.trimLocked()
}

func (c *rpcResultCache) expireLocked(now time.Time) {
	for elem := c.order.Front(); elem != nil; {
		next := elem.Next()
		entry := elem.Value.(*rpcResultCacheEntry)
		if entry.expiresAt.After(now) {
			return
		}
		c.removeElement(elem)
		elem = next
	}
}

func (c *rpcResultCache) trimLocked() {
	for c.order.Len() > 0 {
		tooManyEntries := c.maxEntries > 0 && c.order.Len() > c.maxEntries
		tooManyBytes := c.maxBytes > 0 && c.bytes > c.maxBytes
		if !tooManyEntries && !tooManyBytes {
			return
		}
		c.removeElement(c.order.Front())
	}
}

func (c *rpcResultCache) removeElement(elem *list.Element) {
	if elem == nil {
		return
	}
	entry := elem.Value.(*rpcResultCacheEntry)
	delete(c.byKey, entry.key)
	c.bytes -= entry.size
	if c.bytes < 0 {
		c.bytes = 0
	}
	c.order.Remove(elem)
}

func cloneEncodedOutboundMessage(src *encodedOutboundMessage) *encodedOutboundMessage {
	if src == nil {
		return nil
	}
	body := append([]byte(nil), src.body...)
	return &encodedOutboundMessage{
		body:     body,
		typeID:   src.typeID,
		reqMsgID: src.reqMsgID,
	}
}
