package rpc

import (
	"context"
	"time"
)

const defaultIdleDispatchMaxInterval = 5 * time.Second

type idleBackoff struct {
	base time.Duration
	max  time.Duration
	next time.Duration
}

func newIdleBackoff(base, max time.Duration) idleBackoff {
	if base <= 0 {
		base = time.Second
	}
	if max < base {
		max = base
	}
	return idleBackoff{
		base: base,
		max:  max,
		next: base,
	}
}

func (b *idleBackoff) ActiveDelay() time.Duration {
	b.next = b.base
	return b.base
}

func (b *idleBackoff) IdleDelay() time.Duration {
	delay := b.next
	if b.next < b.max {
		b.next *= 2
		if b.next > b.max {
			b.next = b.max
		}
	}
	return delay
}

func runIdleBackoffLoop(ctx context.Context, interval, maxIdleInterval time.Duration, dispatch func(context.Context) bool) {
	if dispatch == nil {
		return
	}
	backoff := newIdleBackoff(interval, maxIdleInterval)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if dispatch(ctx) {
			timer.Reset(backoff.ActiveDelay())
			continue
		}
		timer.Reset(backoff.IdleDelay())
	}
}
