package rpc

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"

	"telesrv/internal/domain"
)

// richTextBlocks 构造一组纯文本 IV 页面块，用于富文本往返断言。
func richTextBlocks() []tg.PageBlockClass {
	return []tg.PageBlockClass{
		&tg.PageBlockTitle{Text: &tg.TextPlain{Text: "Rich Title"}},
		&tg.PageBlockParagraph{Text: &tg.TextPlain{Text: "First paragraph."}},
	}
}

// assertRichTextBlocks 校验投影出的 RichMessage 携带 richTextBlocks 的两个块（标题+段落）。
func assertRichTextBlocks(t *testing.T, label string, rich tg.RichMessage) {
	t.Helper()
	if !rich.Rtl {
		t.Errorf("%s: rtl = false, want true", label)
	}
	if len(rich.Blocks) != 2 {
		t.Fatalf("%s: blocks = %d, want 2", label, len(rich.Blocks))
	}
	title, ok := rich.Blocks[0].(*tg.PageBlockTitle)
	if !ok {
		t.Fatalf("%s: block[0] = %T, want *tg.PageBlockTitle", label, rich.Blocks[0])
	}
	if tp, ok := title.Text.(*tg.TextPlain); !ok || tp.Text != "Rich Title" {
		t.Errorf("%s: title text = %+v, want plain %q", label, title.Text, "Rich Title")
	}
	para, ok := rich.Blocks[1].(*tg.PageBlockParagraph)
	if !ok {
		t.Fatalf("%s: block[1] = %T, want *tg.PageBlockParagraph", label, rich.Blocks[1])
	}
	if tp, ok := para.Text.(*tg.TextPlain); !ok || tp.Text != "First paragraph." {
		t.Errorf("%s: paragraph text = %+v, want plain %q", label, para.Text, "First paragraph.")
	}
}

// TestSendMessageRichMessageTextBlocksRoundTrip 验证 Layer 227 富文本（inputRichMessage 的
// blocks 形态）经 send → 发送方 echo / getMessages / getRichMessage 全链路原样往返。
func TestSendMessageRichMessageTextBlocksRoundTrip(t *testing.T) {
	ctx := context.Background()
	r, owner, friend := newMediaTestRouter(t)

	updates, err := r.onMessagesSendMessage(WithUserID(ctx, owner.ID), &tg.MessagesSendMessageRequest{
		Peer:     &tg.InputPeerUser{UserID: friend.ID, AccessHash: friend.AccessHash},
		Message:  "rich",
		RandomID: 7001,
		RichMessage: &tg.InputRichMessage{
			Rtl:    true,
			Blocks: richTextBlocks(),
		},
	})
	if err != nil {
		t.Fatalf("send rich message: %v", err)
	}
	echo := newMessageFromUpdates(t, updates)
	rich, ok := echo.GetRichMessage()
	if !ok {
		t.Fatalf("send echo missing rich message")
	}
	assertRichTextBlocks(t, "send echo", rich)

	// getMessages（发送方按 box id 拉取）也应带富文本。
	got, err := r.onMessagesGetMessages(WithUserID(ctx, owner.ID), []tg.InputMessageClass{&tg.InputMessageID{ID: echo.ID}})
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	stored := singleStoredMessage(t, got)
	rich, ok = stored.GetRichMessage()
	if !ok {
		t.Fatalf("getMessages missing rich message")
	}
	assertRichTextBlocks(t, "getMessages", rich)

	// getRichMessage（按 peer+id 拉取完整富文本）应带富文本。
	gotRich, err := r.onMessagesGetRichMessage(WithUserID(ctx, owner.ID), &tg.MessagesGetRichMessageRequest{
		Peer: &tg.InputPeerUser{UserID: friend.ID, AccessHash: friend.AccessHash},
		ID:   echo.ID,
	})
	if err != nil {
		t.Fatalf("get rich message: %v", err)
	}
	stored = singleStoredMessage(t, gotRich)
	rich, ok = stored.GetRichMessage()
	if !ok {
		t.Fatalf("getRichMessage missing rich message")
	}
	assertRichTextBlocks(t, "getRichMessage", rich)
}

// TestGetRichMessageWrongPeerReturnsEmpty 验证 getRichMessage 的 peer 校验：用不匹配的 peer
// 拉取应返回 messageEmpty（不跨会话泄漏）。
func TestGetRichMessageWrongPeerReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	r, owner, friend := newMediaTestRouter(t)

	updates, err := r.onMessagesSendMessage(WithUserID(ctx, owner.ID), &tg.MessagesSendMessageRequest{
		Peer:        &tg.InputPeerUser{UserID: friend.ID, AccessHash: friend.AccessHash},
		Message:     "rich",
		RandomID:    7002,
		RichMessage: &tg.InputRichMessage{Rtl: true, Blocks: richTextBlocks()},
	})
	if err != nil {
		t.Fatalf("send rich message: %v", err)
	}
	echo := newMessageFromUpdates(t, updates)

	// 用 self peer（≠ 该消息盒的 peer=friend）拉取 → messageEmpty。
	gotRich, err := r.onMessagesGetRichMessage(WithUserID(ctx, owner.ID), &tg.MessagesGetRichMessageRequest{
		Peer: &tg.InputPeerSelf{},
		ID:   echo.ID,
	})
	if err != nil {
		t.Fatalf("get rich message wrong peer: %v", err)
	}
	box, ok := gotRich.(*tg.MessagesMessages)
	if !ok || len(box.Messages) != 1 {
		t.Fatalf("getRichMessage wrong peer = %T %+v, want one messages.messages", gotRich, gotRich)
	}
	if _, ok := box.Messages[0].(*tg.MessageEmpty); !ok {
		t.Fatalf("getRichMessage wrong peer message = %T, want *tg.MessageEmpty", box.Messages[0])
	}
}

// TestSendMessageRichMessageEmbeddedPhoto 验证富文本内嵌图片：按 id 解析为媒体快照存储，
// 投影时复用 tgPhoto 还原。
func TestSendMessageRichMessageEmbeddedPhoto(t *testing.T) {
	ctx := context.Background()
	r, owner, friend := newMediaTestRouter(t)
	files, ok := r.deps.Files.(*fakeFiles)
	if !ok {
		t.Fatalf("deps.Files = %T, want *fakeFiles", r.deps.Files)
	}
	files.photos[889] = domain.Photo{ID: 889, AccessHash: 42, DCID: 2, Sizes: []domain.PhotoSize{{Kind: domain.PhotoSizeKindDefault, Type: "x", W: 800, H: 600}}}

	updates, err := r.onMessagesSendMessage(WithUserID(ctx, owner.ID), &tg.MessagesSendMessageRequest{
		Peer:     &tg.InputPeerUser{UserID: friend.ID, AccessHash: friend.AccessHash},
		Message:  "rich+photo",
		RandomID: 7003,
		RichMessage: &tg.InputRichMessage{
			Blocks: []tg.PageBlockClass{&tg.PageBlockParagraph{Text: &tg.TextPlain{Text: "see photo"}}},
			Photos: []tg.InputPhotoClass{&tg.InputPhoto{ID: 889, AccessHash: 42}},
		},
	})
	if err != nil {
		t.Fatalf("send rich message with photo: %v", err)
	}
	echo := newMessageFromUpdates(t, updates)
	rich, ok := echo.GetRichMessage()
	if !ok {
		t.Fatalf("send echo missing rich message")
	}
	if len(rich.Photos) != 1 {
		t.Fatalf("rich photos = %d, want 1", len(rich.Photos))
	}
	photo, ok := rich.Photos[0].(*tg.Photo)
	if !ok {
		t.Fatalf("rich photo = %T, want *tg.Photo", rich.Photos[0])
	}
	if photo.ID != 889 {
		t.Errorf("rich photo id = %d, want 889", photo.ID)
	}
}

// singleStoredMessage 从 messages.messages 取出唯一一条非空 *tg.Message。
func singleStoredMessage(t *testing.T, res tg.MessagesMessagesClass) *tg.Message {
	t.Helper()
	box, ok := res.(*tg.MessagesMessages)
	if !ok || len(box.Messages) != 1 {
		t.Fatalf("messages = %T %+v, want one messages.messages", res, res)
	}
	msg, ok := box.Messages[0].(*tg.Message)
	if !ok {
		t.Fatalf("stored message = %T, want *tg.Message", box.Messages[0])
	}
	return msg
}
