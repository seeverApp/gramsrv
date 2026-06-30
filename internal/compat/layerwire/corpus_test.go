package layerwire

import (
	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// canonicalCorpus is a diverse set of canonical (gotd, Layer 227) objects shared
// by the walker and transcoder tests. It deliberately exercises changed types
// (message, messageMediaPhoto, keyboardButton*, dialog, channelFull, userFull,
// pollResults/pollAnswerVoters), nested containers, vectors, and multi-flags.
func canonicalCorpus() []bin.Encoder {
	photo := &tg.Photo{
		ID:            10,
		AccessHash:    11,
		FileReference: []byte{1, 2, 3},
		Date:          100,
		Sizes: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "x", W: 100, H: 100, Size: 2048},
			&tg.PhotoStrippedSize{Type: "i", Bytes: []byte{9, 8, 7}},
		},
		DCID: 2,
	}
	return []bin.Encoder{
		&tg.Message{ID: 1, PeerID: &tg.PeerUser{UserID: 2}, Date: 100, Message: "hi"},
		&tg.Message{
			Out:     true,
			ID:      2,
			FromID:  &tg.PeerUser{UserID: 3},
			PeerID:  &tg.PeerChannel{ChannelID: 4},
			Date:    101,
			Message: "rich",
			Media:   &tg.MessageMediaPhoto{Photo: photo, TTLSeconds: 5},
			Entities: []tg.MessageEntityClass{
				&tg.MessageEntityBold{Offset: 0, Length: 2},
				&tg.MessageEntityTextURL{Offset: 0, Length: 2, URL: "https://x"},
			},
			ReplyMarkup: &tg.ReplyInlineMarkup{Rows: []tg.KeyboardButtonRow{
				{Buttons: []tg.KeyboardButtonClass{
					&tg.KeyboardButtonCallback{Text: "ok", Data: []byte("d")},
					&tg.KeyboardButtonURL{Text: "go", URL: "https://y"},
				}},
			}},
			ReplyTo:   &tg.MessageReplyHeader{ReplyToMsgID: 1},
			FwdFrom:   tg.MessageFwdHeader{FromName: "n", Date: 99},
			Views:     7,
			Forwards:  2,
			Reactions: tg.MessageReactions{Results: []tg.ReactionCount{{Reaction: &tg.ReactionEmoji{Emoticon: "👍"}, Count: 3}}},
			GroupedID: 555,
		},
		&tg.MessageService{
			ID:     3,
			PeerID: &tg.PeerUser{UserID: 2},
			Date:   102,
			Action: &tg.MessageActionChatEditTitle{Title: "t"},
		},
		&tg.Updates{
			Updates: []tg.UpdateClass{
				&tg.UpdateNewMessage{Message: &tg.Message{ID: 9, PeerID: &tg.PeerUser{UserID: 2}, Date: 1, Message: "u"}, Pts: 1, PtsCount: 1},
				&tg.UpdateMessageID{ID: 9, RandomID: 123},
			},
			Users: []tg.UserClass{&tg.User{ID: 2, AccessHash: 5, FirstName: "A"}},
			Chats: []tg.ChatClass{&tg.Channel{ID: 4, AccessHash: 6, Title: "C", Photo: &tg.ChatPhotoEmpty{}}},
			Date:  100,
			Seq:   1,
		},
		&tg.User{
			ID:         2,
			AccessHash: 5,
			FirstName:  "A",
			Username:   "a",
			Photo:      &tg.UserProfilePhoto{PhotoID: 7, DCID: 2},
			Status:     &tg.UserStatusOnline{Expires: 999},
		},
		&tg.UserFull{
			ID:               2,
			About:            "hi",
			Settings:         tg.PeerSettings{},
			NotifySettings:   tg.PeerNotifySettings{},
			CommonChatsCount: 0,
		},
		&tg.Channel{ID: 4, AccessHash: 6, Title: "C", Megagroup: true, Photo: &tg.ChatPhotoEmpty{}},
		&tg.ChannelFull{
			ID:              4,
			About:           "about",
			ReadInboxMaxID:  1,
			ReadOutboxMaxID: 1,
			UnreadCount:     0,
			ChatPhoto:       &tg.PhotoEmpty{ID: 0},
			NotifySettings:  tg.PeerNotifySettings{},
			Pts:             1,
		},
		&tg.Dialog{
			Peer:           &tg.PeerUser{UserID: 2},
			TopMessage:     2,
			ReadInboxMaxID: 1,
			NotifySettings: tg.PeerNotifySettings{},
		},
		&tg.Poll{
			ID:       1,
			Question: tg.TextWithEntities{Text: "q?"},
			Answers: []tg.PollAnswerClass{
				&tg.PollAnswer{Text: tg.TextWithEntities{Text: "a"}, Option: []byte{0}},
				&tg.PollAnswer{Text: tg.TextWithEntities{Text: "b"}, Option: []byte{1}},
			},
		},
		&tg.PollResults{
			Results: []tg.PollAnswerVoters{
				{Option: []byte{0}, Voters: 3, Chosen: true},
				{Option: []byte{1}, Voters: 1},
			},
			TotalVoters: 4,
		},
		&tg.MessagesDialogs{
			Dialogs:  []tg.DialogClass{&tg.Dialog{Peer: &tg.PeerUser{UserID: 2}, TopMessage: 2, NotifySettings: tg.PeerNotifySettings{}}},
			Messages: []tg.MessageClass{&tg.Message{ID: 2, PeerID: &tg.PeerUser{UserID: 2}, Date: 1, Message: "x"}},
			Chats:    []tg.ChatClass{},
			Users:    []tg.UserClass{&tg.User{ID: 2, AccessHash: 5, FirstName: "A"}},
		},
		&tg.MessagesMessages{
			Messages: []tg.MessageClass{&tg.Message{ID: 2, PeerID: &tg.PeerUser{UserID: 2}, Date: 1, Message: "x"}},
			Chats:    []tg.ChatClass{},
			Users:    []tg.UserClass{&tg.User{ID: 2, AccessHash: 5, FirstName: "A"}},
		},
	}
}
