package postgres

import (
	"context"
	"errors"
	"testing"

	"telesrv/internal/domain"
)

func TestScheduledMessageEditPreservesContentWhenMessageUnset(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	suffix := randomSuffix(t)

	users := NewUserStore(pool)
	owner, err := users.Create(ctx, domain.User{AccessHash: 71, Phone: "+1777" + suffix + "01", FirstName: "ScheduledOwner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	peerUser, err := users.Create(ctx, domain.User{AccessHash: 72, Phone: "+1777" + suffix + "02", FirstName: "ScheduledPeer"})
	if err != nil {
		t.Fatalf("create peer: %v", err)
	}
	ids := []int64{owner.ID, peerUser.ID}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM scheduled_messages WHERE owner_user_id = $1", owner.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM dialogs WHERE user_id = ANY($1::bigint[])", ids)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1::bigint[])", ids)
	})

	messages := NewMessageStore(pool)
	peer := domain.Peer{Type: domain.PeerTypeUser, ID: peerUser.ID}
	media := &domain.MessageMedia{
		Kind: domain.MessageMediaKindDocument,
		Document: &domain.Document{
			ID:         910000000000000101,
			AccessHash: 9101,
			DCID:       2,
			MimeType:   "application/x-tgsticker",
			Attributes: []domain.DocumentAttribute{{Kind: domain.DocAttrSticker, Alt: "wave"}},
		},
	}
	scheduled, err := messages.CreateScheduledMessage(ctx, domain.ScheduleMessageRequest{
		OwnerUserID:  owner.ID,
		Peer:         peer,
		RandomID:     7001,
		Message:      "",
		Media:        media,
		ScheduleDate: 1700003600,
		Date:         1700000000,
	})
	if err != nil {
		t.Fatalf("create media scheduled message: %v", err)
	}

	dateOnly, err := messages.EditScheduledMessage(ctx, domain.EditScheduledMessageRequest{
		OwnerUserID:  owner.ID,
		Peer:         peer,
		ID:           scheduled.ID,
		ScheduleDate: 1700007200,
		Date:         1700000100,
	})
	if err != nil {
		t.Fatalf("date-only edit scheduled message: %v", err)
	}
	if dateOnly.Message != "" || dateOnly.ScheduleDate != 1700007200 || dateOnly.Media == nil || dateOnly.Media.Document == nil || dateOnly.Media.Document.ID != media.Document.ID {
		t.Fatalf("date-only scheduled edit = %+v, want original media/content and new date", dateOnly)
	}

	emptyCaption, err := messages.EditScheduledMessage(ctx, domain.EditScheduledMessageRequest{
		OwnerUserID:  owner.ID,
		Peer:         peer,
		ID:           scheduled.ID,
		SetMessage:   true,
		Message:      "",
		ScheduleDate: 1700010800,
		Date:         1700000200,
	})
	if err != nil {
		t.Fatalf("empty-caption edit scheduled media: %v", err)
	}
	if emptyCaption.Message != "" || emptyCaption.ScheduleDate != 1700010800 || emptyCaption.Media == nil || emptyCaption.Media.Document == nil || emptyCaption.Media.Document.ID != media.Document.ID {
		t.Fatalf("empty-caption scheduled edit = %+v, want media kept and new date", emptyCaption)
	}

	textOnly, err := messages.CreateScheduledMessage(ctx, domain.ScheduleMessageRequest{
		OwnerUserID:  owner.ID,
		Peer:         peer,
		RandomID:     7002,
		Message:      "text",
		ScheduleDate: 1700014400,
		Date:         1700000300,
	})
	if err != nil {
		t.Fatalf("create text scheduled message: %v", err)
	}
	_, err = messages.EditScheduledMessage(ctx, domain.EditScheduledMessageRequest{
		OwnerUserID:  owner.ID,
		Peer:         peer,
		ID:           textOnly.ID,
		SetMessage:   true,
		Message:      "",
		ScheduleDate: 1700018000,
		Date:         1700000400,
	})
	if !errors.Is(err, domain.ErrMessageEmpty) {
		t.Fatalf("empty text scheduled edit err = %v, want ErrMessageEmpty", err)
	}
}
