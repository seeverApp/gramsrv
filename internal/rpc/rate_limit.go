package rpc

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const sendRateLimitKeyPrefix = "messages:send:"

const (
	channelDifferenceRateLimitKeyPrefix = "updates:channeldifference:"
	peerDialogsRateLimitKeyPrefix       = "messages:peerdialogs:"
	defaultCatchupRateWindow            = time.Minute
)

// checkCatchupRateLimit 对 difference 类 catch-up RPC（getChannelDifference / getPeerDialogs）按
// 每用户频率限速，超限返回 FLOOD_WAIT（设计 Phase 2 / §10.3）。keyPrefix 区分两类（各自独立计数），
// 共用 cfg.CatchupRateLimit/Window 阈值。Limiter 未装配或阈值 <=0 时不限速（行为不变）。
func (r *Router) checkCatchupRateLimit(ctx context.Context, userID int64, keyPrefix string) error {
	if r.deps.Limiter == nil || userID == 0 {
		return nil
	}
	limit := r.cfg.CatchupRateLimit
	if limit <= 0 {
		return nil
	}
	window := r.cfg.CatchupRateWindow
	if window <= 0 {
		window = defaultCatchupRateWindow
	}
	allowed, retryAfter, err := r.deps.Limiter.AllowN(ctx, keyPrefix+strconv.FormatInt(userID, 10), 1, limit, window)
	if err != nil {
		return internalErr()
	}
	if allowed {
		return nil
	}
	r.log.Debug("catch-up rpc rate limited (flood wait)",
		zap.Int64("user_id", userID), zap.String("kind", keyPrefix), zap.Int("retry_after", retryAfter))
	return floodWaitErr(retryAfter)
}

func (r *Router) checkSendRateLimit(ctx context.Context, userID int64, cost int) error {
	if r.deps.Limiter == nil || userID == 0 || cost <= 0 {
		return nil
	}
	limit := r.cfg.SendRateLimit
	if limit <= 0 {
		return nil
	}
	window := r.cfg.SendRateWindow
	if window <= 0 {
		window = sendMessageRateWindow
	}
	allowed, retryAfter, err := r.deps.Limiter.AllowN(ctx, sendRateLimitKeyPrefix+strconv.FormatInt(userID, 10), cost, limit, window)
	if err != nil {
		return internalErr()
	}
	if allowed {
		return nil
	}
	r.metrics().MessageRateLimited(retryAfter)
	return floodWaitErr(retryAfter)
}
