package messages

import (
	"context"
	"testing"

	"telesrv/internal/domain"
	"telesrv/internal/store/memory"
)

func TestServiceProjectsMessageUsersForViewerContacts(t *testing.T) {
	ctx := context.Background()
	const ownerID int64 = 1001
	const friendID int64 = 1002
	const strangerID int64 = 1003
	contacts := memory.NewContactStore()
	if _, err := contacts.Upsert(ctx, ownerID, domain.ContactInput{
		ContactUserID: friendID,
		Phone:         "15550000002",
		FirstName:     "Remark",
		LastName:      "Friend",
	}); err != nil {
		t.Fatalf("upsert contact: %v", err)
	}
	store := projectionMessageStore{list: domain.MessageList{
		Users: []domain.User{
			{ID: ownerID, Phone: "15550000001", FirstName: "Owner"},
			{ID: friendID, AccessHash: 22, Phone: "15550000002", FirstName: "Public", LastName: "Name"},
			{ID: strangerID, AccessHash: 33, Phone: "15550000003", FirstName: "Stranger"},
		},
	}}
	svc := NewService(store, nil, WithContactStore(contacts))

	list, err := svc.GetHistory(ctx, ownerID, domain.MessageFilter{Limit: 10})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	friend := findUser(t, list.Users, friendID)
	if !friend.Contact || friend.FirstName != "Remark" || friend.LastName != "Friend" || friend.Phone != "15550000002" {
		t.Fatalf("friend projection = %+v, want contact remark and phone", friend)
	}
	stranger := findUser(t, list.Users, strangerID)
	if stranger.Contact || stranger.Phone != "" || stranger.FirstName != "Stranger" {
		t.Fatalf("stranger projection = %+v, want non-contact with hidden phone", stranger)
	}
	self := findUser(t, list.Users, ownerID)
	if self.Phone != "15550000001" {
		t.Fatalf("self phone = %q, want preserved", self.Phone)
	}
}

func findUser(t *testing.T, users []domain.User, id int64) domain.User {
	t.Helper()
	for _, user := range users {
		if user.ID == id {
			return user
		}
	}
	t.Fatalf("user %d not found in %+v", id, users)
	return domain.User{}
}

type projectionMessageStore struct {
	list domain.MessageList
}

func (s projectionMessageStore) Create(context.Context, domain.Message) (domain.Message, error) {
	return domain.Message{}, nil
}

func (s projectionMessageStore) SendPrivateText(context.Context, domain.SendPrivateTextRequest) (domain.SendPrivateTextResult, error) {
	return domain.SendPrivateTextResult{}, nil
}

func (s projectionMessageStore) ForwardPrivateMessages(context.Context, domain.ForwardPrivateMessagesRequest) (domain.ForwardPrivateMessagesResult, error) {
	return domain.ForwardPrivateMessagesResult{}, nil
}

func (s projectionMessageStore) ReadHistory(context.Context, domain.ReadHistoryRequest) (domain.ReadHistoryResult, error) {
	return domain.ReadHistoryResult{}, nil
}

func (s projectionMessageStore) ReadMessageContents(context.Context, domain.ReadMessageContentsRequest) (domain.ReadMessageContentsResult, error) {
	return domain.ReadMessageContentsResult{}, nil
}

func (s projectionMessageStore) GetOutboxReadDate(context.Context, domain.OutboxReadDateRequest) (int, error) {
	return 0, nil
}

func (s projectionMessageStore) SetMessageReactions(context.Context, domain.SetPrivateMessageReactionsRequest) (domain.PrivateMessageReactionsResult, error) {
	return domain.PrivateMessageReactionsResult{}, nil
}

func (s projectionMessageStore) GetMessageReactions(context.Context, domain.PrivateMessageReactionsRequest) (domain.PrivateMessageReactionsResult, error) {
	return domain.PrivateMessageReactionsResult{}, nil
}

func (s projectionMessageStore) EditMessage(context.Context, domain.EditMessageRequest) (domain.EditMessageResult, error) {
	return domain.EditMessageResult{}, nil
}

func (s projectionMessageStore) DeleteMessages(context.Context, domain.DeleteMessagesRequest) (domain.DeleteMessagesResult, error) {
	return domain.DeleteMessagesResult{}, nil
}

func (s projectionMessageStore) DeleteHistory(context.Context, domain.DeleteHistoryRequest) (domain.DeleteMessagesResult, error) {
	return domain.DeleteMessagesResult{}, nil
}

func (s projectionMessageStore) GetByIDs(context.Context, int64, []int) (domain.MessageList, error) {
	return s.list, nil
}

func (s projectionMessageStore) ListByUser(context.Context, int64, domain.MessageFilter) (domain.MessageList, error) {
	return s.list, nil
}
