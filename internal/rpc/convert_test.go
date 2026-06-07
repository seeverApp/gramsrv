package rpc

import (
	"testing"

	"github.com/gotd/td/tg"

	"telesrv/internal/domain"
)

func TestTGMessagesMessagesMarksViewerSelfAndKeepsProjectedPhone(t *testing.T) {
	const viewerID int64 = 1001
	res := tgMessagesMessages(viewerID, domain.MessageList{
		Users: []domain.User{
			{ID: viewerID, AccessHash: 11, Phone: "15550000001", FirstName: "Owner"},
			{ID: 1002, AccessHash: 22, Phone: "", FirstName: "Peer"},
		},
	})
	full, ok := res.(*tg.MessagesMessages)
	if !ok {
		t.Fatalf("result = %T, want *tg.MessagesMessages", res)
	}
	self, ok := full.Users[0].(*tg.User)
	if !ok || !self.Self || self.Phone != "15550000001" {
		t.Fatalf("self user = %+v ok=%v, want self with phone", full.Users[0], ok)
	}
	peer, ok := full.Users[1].(*tg.User)
	if !ok || peer.Self || peer.Phone != "" || peer.FirstName != "Peer" {
		t.Fatalf("peer user = %+v ok=%v, want projected non-self without phone", full.Users[1], ok)
	}
}
