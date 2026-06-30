package rpc

import (
	"testing"
	"time"
)

func TestIdleBackoffSequenceAndReset(t *testing.T) {
	backoff := newIdleBackoff(100*time.Millisecond, 450*time.Millisecond)
	wantIdle := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		450 * time.Millisecond,
		450 * time.Millisecond,
	}
	for i, want := range wantIdle {
		if got := backoff.IdleDelay(); got != want {
			t.Fatalf("idle delay[%d] = %v, want %v", i, got, want)
		}
	}
	if got := backoff.ActiveDelay(); got != 100*time.Millisecond {
		t.Fatalf("active delay = %v, want base interval", got)
	}
	if got := backoff.IdleDelay(); got != 100*time.Millisecond {
		t.Fatalf("idle delay after reset = %v, want base interval", got)
	}
}

func TestIdleBackoffSanitizesMaxBelowBase(t *testing.T) {
	backoff := newIdleBackoff(2*time.Second, time.Second)
	if got := backoff.IdleDelay(); got != 2*time.Second {
		t.Fatalf("first idle delay = %v, want base interval", got)
	}
	if got := backoff.IdleDelay(); got != 2*time.Second {
		t.Fatalf("second idle delay = %v, want clamped max at base interval", got)
	}
}
