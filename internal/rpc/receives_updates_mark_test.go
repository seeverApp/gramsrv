package rpc

import (
	"context"
	"testing"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/clock"
	"github.com/gotd/td/tg"
	"go.uber.org/zap/zaptest"
)

func dispatchForReceivesUpdates(t *testing.T, sessions *captureSessions, wrapWithoutUpdates, loggedIn bool) {
	t.Helper()
	r := New(Config{DC: 2, IP: "127.0.0.1", Port: 2398}, Deps{Sessions: sessions}, zaptest.NewLogger(t), clock.System)

	var inner bin.Buffer
	if err := (&tg.HelpGetConfigRequest{}).Encode(&inner); err != nil {
		t.Fatalf("encode help.getConfig: %v", err)
	}
	var in bin.Buffer
	if wrapWithoutUpdates {
		in.PutID(tg.InvokeWithoutUpdatesRequestTypeID)
	}
	in.Put(inner.Buf)

	ctx := context.Background()
	if loggedIn {
		ctx = WithUserID(ctx, 1000000001)
	}
	if _, err := r.Dispatch(ctx, [8]byte{1}, 42, &in); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
}

// TestDispatchMarksSessionReceivesUpdates 验证已登录连接发出的裸 RPC（未包
// invokeWithoutUpdates）即视为 updates 接收声明。仅靠 updates.getState/getDifference
// 置位会漏掉热恢复重连的客户端：它不重建同步基线，置位永不发生时主动推送一直
// 暂存直至超时丢弃，表现为另一端消息不再实时同步。
func TestDispatchMarksSessionReceivesUpdates(t *testing.T) {
	sessions := &captureSessions{}
	dispatchForReceivesUpdates(t, sessions, false, true)
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if !sessions.receives {
		t.Fatal("plain RPC from logged-in session must mark receivesUpdates")
	}
	if sessions.sessionID != 42 {
		t.Fatalf("marked session_id = %d, want 42", sessions.sessionID)
	}
}

// TestDispatchSkipsReceivesUpdatesForInvokeWithoutUpdates 验证 invokeWithoutUpdates
// 包装的请求（media/temp 连接）不会把该 session 标记为 updates 接收者。
func TestDispatchSkipsReceivesUpdatesForInvokeWithoutUpdates(t *testing.T) {
	sessions := &captureSessions{}
	dispatchForReceivesUpdates(t, sessions, true, true)
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.receives {
		t.Fatal("invokeWithoutUpdates-wrapped RPC must not mark receivesUpdates")
	}
}

// TestDispatchSkipsReceivesUpdatesWhenLoggedOut 验证未登录连接的 RPC 不置位。
func TestDispatchSkipsReceivesUpdatesWhenLoggedOut(t *testing.T) {
	sessions := &captureSessions{}
	dispatchForReceivesUpdates(t, sessions, false, false)
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.receives {
		t.Fatal("RPC without bound user must not mark receivesUpdates")
	}
}
