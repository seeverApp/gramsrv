package maintenance

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeOutboxRetention struct {
	calls int
}

func (f *fakeOutboxRetention) DeleteFailed(context.Context, time.Duration, int) (int, error) {
	f.calls++
	return 0, nil
}

type fakeTempKeyRetention struct {
	calls         int
	expiredBefore int64
	limit         int
}

func (f *fakeTempKeyRetention) DeleteExpired(_ context.Context, expiredBefore int64, limit int) (int, error) {
	f.calls++
	f.expiredBefore = expiredBefore
	f.limit = limit
	return 3, nil
}

func TestRetentionWorkerReclaimsExpiredTempKeys(t *testing.T) {
	outbox := &fakeOutboxRetention{}
	temp := &fakeTempKeyRetention{}
	w := NewRetentionWorker(outbox, temp, zap.NewNop(), time.Hour, time.Hour, 100)

	w.runOnce(context.Background())

	if outbox.calls != 1 || temp.calls != 1 {
		t.Fatalf("calls outbox=%d temp=%d, want 1/1", outbox.calls, temp.calls)
	}
	if temp.limit != 100 {
		t.Fatalf("limit = %d, want batch 100", temp.limit)
	}
	wantBefore := time.Now().Add(-tempAuthKeyExpiryGrace).Unix()
	if diff := temp.expiredBefore - wantBefore; diff < -5 || diff > 5 {
		t.Fatalf("expiredBefore = %d, want ≈ now-grace (%d)", temp.expiredBefore, wantBefore)
	}
}

func TestRetentionWorkerSkipsNilTempKeyStore(t *testing.T) {
	outbox := &fakeOutboxRetention{}
	w := NewRetentionWorker(outbox, nil, zap.NewNop(), time.Hour, time.Hour, 100)
	w.runOnce(context.Background()) // 不应 panic
	if outbox.calls != 1 {
		t.Fatalf("outbox calls = %d, want 1", outbox.calls)
	}
}
