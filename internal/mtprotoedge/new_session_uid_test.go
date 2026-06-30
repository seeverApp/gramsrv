package mtprotoedge

import (
	"testing"
	"time"

	"github.com/gotd/td/mt"
	"github.com/gotd/td/proto"
	"github.com/gotd/td/tg"
)

// TestNewSessionCreatedUniqueIDPerSession 验证两次 session 建立收到的
// new_session_created.unique_id 互不相同。客户端按 unique_id 去重，复用同一值
// 会让断线重连后的 new_session_created 被吞掉，依赖它触发的差分补拉随之丢失。
func TestNewSessionCreatedUniqueIDPerSession(t *testing.T) {
	const dc = 2
	addr, pub, _ := startTestServer(t, Options{DC: dc})
	conn, auth, cipher := dialHandshake(t, addr, dc, pub)

	clientMsgID := proto.NewMessageIDGen(time.Now)
	firstMsgID := clientMsgID.New(proto.MessageFromClient)
	sendEncryptedWithSeq(t, conn, cipher, auth, firstMsgID, 1, &tg.HelpGetConfigRequest{})
	first := collectReplies(t, conn, cipher, auth.AuthKey, mt.NewSessionCreatedTypeID)
	var created1 mt.NewSessionCreated
	if err := created1.Decode(mustHave(t, first, mt.NewSessionCreatedTypeID, "first new_session_created")); err != nil {
		t.Fatalf("decode first new_session_created: %v", err)
	}

	nextSessionID := auth.SessionID + 1
	if nextSessionID == 0 {
		nextSessionID++
	}
	secondMsgID := clientMsgID.New(proto.MessageFromClient)
	body := encodeClientMessageBodyForTest(t, &tg.HelpGetConfigRequest{})
	sendEncryptedWithSessionSaltAndSeq(t, conn, cipher, auth, nextSessionID, auth.ServerSalt, secondMsgID, 1, body)
	second := collectReplies(t, conn, cipher, auth.AuthKey, mt.NewSessionCreatedTypeID)
	var created2 mt.NewSessionCreated
	if err := created2.Decode(mustHave(t, second, mt.NewSessionCreatedTypeID, "second new_session_created")); err != nil {
		t.Fatalf("decode second new_session_created: %v", err)
	}

	if created1.UniqueID == created2.UniqueID {
		t.Fatalf("new_session_created.unique_id reused across sessions: %d", created1.UniqueID)
	}
}
