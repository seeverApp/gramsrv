package postgres

import (
	"context"
	"testing"
	"time"

	"telesrv/internal/domain"
)

// TestDialogDraftGetRoundTrip 回归：GetDraft（draft_message 事件重放按 peer 重载权威态）
// 与 SaveDraft/DeleteDraft 的键语义一致（user, peer, top_message_id）。
func TestDialogDraftGetRoundTrip(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	suffix := randomSuffix(t)
	owner, err := NewUserStore(pool).Create(ctx, domain.User{AccessHash: 31, Phone: "+1777" + suffix + "01", FirstName: "DraftOwner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	userID := owner.ID
	peer := domain.Peer{Type: domain.PeerTypeUser, ID: userID + 1}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM dialog_drafts WHERE user_id = $1", userID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	})

	dialogs := NewDialogStore(pool)
	if _, found, err := dialogs.GetDraft(ctx, userID, peer, 0); err != nil || found {
		t.Fatalf("get missing draft = found %v err %v, want absent", found, err)
	}
	saved := domain.DialogDraft{Peer: peer, Message: "pg roundtrip", Date: int(time.Now().Unix())}
	if err := dialogs.SaveDraft(ctx, userID, saved); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	got, found, err := dialogs.GetDraft(ctx, userID, peer, 0)
	if err != nil || !found {
		t.Fatalf("get draft = found %v err %v, want present", found, err)
	}
	if got.Message != "pg roundtrip" || got.Peer != peer {
		t.Fatalf("draft = %+v, want saved payload", got)
	}
	if _, err := dialogs.DeleteDraft(ctx, userID, peer, 0); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	if _, found, err := dialogs.GetDraft(ctx, userID, peer, 0); err != nil || found {
		t.Fatalf("get deleted draft = found %v err %v, want absent", found, err)
	}
}
