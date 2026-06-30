package rpc

import (
	"crypto/rand"
	"encoding/binary"
	"hash/fnv"
	"sync"
	"sync/atomic"

	"telesrv/internal/domain"
)

// callbackRegistry 是 bot callback query 的进程内挂起表：messages.getBotCallbackAnswer
// 注册一个 (query_id → chan)，把 updateBotCallbackQuery 推给 bot 后阻塞等待；bot 经
// messages.setBotCallbackAnswer 用同一 query_id 解挂。单实例可行；多实例需共享通道
// （getBotCallbackAnswer 与 setBotCallbackAnswer 落不同实例则等不到 → 超时），记架构 todo。
type callbackRegistry struct {
	mu      sync.Mutex
	pending map[int64]*pendingCallback
}

type pendingCallback struct {
	ch        chan domain.BotCallbackAnswer
	done      chan struct{} // deregister 时关闭：唤醒超时退出的等待者，消除「答案投递到已离开 select 的 ch」竞态
	botUserID int64
	userID    int64
}

func newCallbackRegistry() *callbackRegistry {
	return &callbackRegistry{pending: make(map[int64]*pendingCallback)}
}

// register 登记一次挂起的 callback，返回全局唯一 query_id 与接收通道。调用方必须
// defer deregister(queryID)，无论是否收到答案（超时三件套之一，防 goroutine/表泄漏）。
func (c *callbackRegistry) register(botUserID, userID int64) (int64, *pendingCallback) {
	p := &pendingCallback{
		ch:        make(chan domain.BotCallbackAnswer, 1),
		done:      make(chan struct{}),
		botUserID: botUserID,
		userID:    userID,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var queryID int64
	for {
		queryID = randomNonZeroInt64()
		if _, exists := c.pending[queryID]; !exists {
			break
		}
	}
	c.pending[queryID] = p
	return queryID, p
}

// deregister 移除挂起条目并关闭 done（超时/解挂后必调，幂等）。关闭 done 让仍在
// select 的等待者立即醒来，避免 resolve 把答案投递到一个等待者已离开的 ch（TOCTOU）。
func (c *callbackRegistry) deregister(queryID int64) {
	c.mu.Lock()
	if p, ok := c.pending[queryID]; ok {
		delete(c.pending, queryID)
		close(p.done)
	}
	c.mu.Unlock()
}

// size 返回当前挂起条目数（测试用：断言超时/解挂后归零、无泄漏）。
func (c *callbackRegistry) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// resolve 把 bot 的答案投递给等待者。鉴权：仅该 query 的属主 bot 可解挂（callerBotID
// 必须等于注册时的 botUserID，I6）。返回是否成功投递（query 未注册/已超时/非属主 → false）。
func (c *callbackRegistry) resolve(callerBotID, queryID int64, ans domain.BotCallbackAnswer) bool {
	c.mu.Lock()
	p, ok := c.pending[queryID]
	if !ok || p.botUserID != callerBotID {
		c.mu.Unlock()
		return false
	}
	delete(c.pending, queryID)
	c.mu.Unlock()
	// ch 有 1 容量缓冲，非阻塞投递；等待者已超时退出时缓冲被 GC，不阻塞。
	select {
	case p.ch <- ans:
	default:
	}
	return true
}

// randomNonZeroInt64 取密码学随机非零 int64。register 在持锁下调用，故此处禁止
// 无限重试——熵源异常时退化为单调序列兜底（query_id 只需进程内唯一，register 的
// 撞键复核会再保证唯一性），绝不卡住整个 registry。
func randomNonZeroInt64() int64 {
	var buf [8]byte
	for i := 0; i < 8; i++ {
		if _, err := rand.Read(buf[:]); err != nil {
			break
		}
		if v := int64(binary.LittleEndian.Uint64(buf[:])); v != 0 {
			return v
		}
	}
	if v := callbackFallbackSeq.Add(1); v != 0 {
		return v
	}
	return 1
}

// callbackFallbackSeq 是熵源失败时的单调兜底序列（极罕见路径）。
var callbackFallbackSeq atomic.Int64

// chatInstanceFor 为 (bot,user) 私聊派生稳定的 chat_instance（同一对话多次 callback
// 间恒定，I8）。当前用确定性 hash 派生（持久化记 todo）。
func chatInstanceFor(botUserID, userID int64) int64 {
	h := fnv.New64a()
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(botUserID))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(userID))
	_, _ = h.Write(buf[:])
	v := int64(h.Sum64())
	if v == 0 {
		v = 1
	}
	return v
}
