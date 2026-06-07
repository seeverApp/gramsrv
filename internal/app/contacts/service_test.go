package contacts

import (
	"context"
	"errors"
	"testing"

	"telesrv/internal/domain"
	"telesrv/internal/store/memory"
)

func TestImportContactsBatchesPhonesAndDedupesUpserts(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserStore()
	contactsStore := memory.NewContactStore()
	owner, err := users.Create(ctx, domain.User{Phone: "100", FirstName: "Owner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	target, err := users.Create(ctx, domain.User{Phone: "15551234567", FirstName: "Alice"})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	svc := NewService(contactsStore, users)

	res, err := svc.ImportContacts(ctx, owner.ID, []domain.ContactInput{
		{ClientID: 11, Phone: "+1 (555) 123-4567", FirstName: "A"},
		{ClientID: 12, Phone: "15551234567", FirstName: "Alice Final"},
	})
	if err != nil {
		t.Fatalf("ImportContacts: %v", err)
	}
	if len(res.Imported) != 2 {
		t.Fatalf("imported = %d, want 2", len(res.Imported))
	}
	if res.Imported[0].UserID != target.ID || res.Imported[1].UserID != target.ID {
		t.Fatalf("imported user ids = %+v, want target %d", res.Imported, target.ID)
	}
	if len(res.Contacts) != 1 {
		t.Fatalf("contacts = %d, want 1 deduped upsert", len(res.Contacts))
	}
	if res.Contacts[0].FirstName != "Alice Final" {
		t.Fatalf("contact first name = %q, want final input", res.Contacts[0].FirstName)
	}
}

func TestAcceptContactSharesPhoneAndClearsShareContact(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserStore()
	contactsStore := memory.NewContactStore()
	alice, err := users.Create(ctx, domain.User{Phone: "15550000001", FirstName: "Alice", LastName: "A"})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := users.Create(ctx, domain.User{Phone: "15550000002", FirstName: "Bob", LastName: "B"})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	svc := NewService(contactsStore, users)

	if _, err := svc.AddContact(ctx, alice.ID, domain.ContactInput{
		ContactUserID: bob.ID,
		Phone:         bob.Phone,
		FirstName:     "Bobby",
		LastName:      "Remark",
	}); err != nil {
		t.Fatalf("alice add bob: %v", err)
	}
	settings, err := svc.GetPeerSettings(ctx, alice.ID, domain.Peer{Type: domain.PeerTypeUser, ID: bob.ID})
	if err != nil {
		t.Fatalf("alice peer settings before accept: %v", err)
	}
	if !settings.ShareContact {
		t.Fatalf("alice settings before accept = %+v, want share contact", settings)
	}

	contact, err := svc.AcceptContact(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("AcceptContact: %v", err)
	}
	if !contact.Mutual || !contact.User.Mutual {
		t.Fatalf("accepted contact = %+v, want mutual", contact)
	}
	aliceSettings, err := svc.GetPeerSettings(ctx, alice.ID, domain.Peer{Type: domain.PeerTypeUser, ID: bob.ID})
	if err != nil {
		t.Fatalf("alice peer settings after accept: %v", err)
	}
	if aliceSettings.ShareContact || aliceSettings.AddContact {
		t.Fatalf("alice settings after accept = %+v, want no share/add", aliceSettings)
	}
	bobSettings, err := svc.GetPeerSettings(ctx, bob.ID, domain.Peer{Type: domain.PeerTypeUser, ID: alice.ID})
	if err != nil {
		t.Fatalf("bob peer settings after accept: %v", err)
	}
	if bobSettings.ShareContact || bobSettings.AddContact {
		t.Fatalf("bob settings after accept = %+v, want no share/add", bobSettings)
	}
	reverse, found, err := contactsStore.Get(ctx, bob.ID, alice.ID)
	if err != nil || !found {
		t.Fatalf("bob contact alice found=%v err=%v", found, err)
	}
	if reverse.Phone != alice.Phone || reverse.FirstName != alice.FirstName || reverse.LastName != alice.LastName || !reverse.Mutual {
		t.Fatalf("bob contact alice = %+v, want alice phone/name and mutual", reverse)
	}

	repeated, err := svc.AcceptContact(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("AcceptContact repeat: %v", err)
	}
	if !repeated.Mutual {
		t.Fatalf("repeated accept = %+v, want mutual", repeated)
	}
}

func TestAcceptContactRequiresExistingContactRequest(t *testing.T) {
	ctx := context.Background()
	users := memory.NewUserStore()
	contactsStore := memory.NewContactStore()
	alice, err := users.Create(ctx, domain.User{Phone: "15550000001", FirstName: "Alice"})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := users.Create(ctx, domain.User{Phone: "15550000002", FirstName: "Bob"})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	svc := NewService(contactsStore, users)

	if _, err := svc.AcceptContact(ctx, alice.ID, bob.ID); !errors.Is(err, ErrContactReqMissing) {
		t.Fatalf("AcceptContact without contact err = %v, want ErrContactReqMissing", err)
	}
}
