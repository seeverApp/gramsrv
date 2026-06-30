package botapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"telesrv/internal/domain"
	"telesrv/internal/store"
)

func TestAnswerWebAppQueryParsesArticleAndCallsService(t *testing.T) {
	webapps := &fakeWebAppService{}
	bots := &fakeBotAPIBots{profile: domain.BotProfile{BotUserID: 1001, TokenSecret: "secret"}}
	h := (&handler{bots: bots, webapps: webapps}).routes()
	body := `{
		"web_app_query_id": "web-query-1",
		"result": {
			"type": "article",
			"id": "share-1",
			"title": "Share",
			"description": "from mini app",
			"url": "https://example.com/share",
			"input_message_content": {
				"message_text": "hello mini app",
				"disable_web_page_preview": true,
				"entities": [{"type": "bold", "offset": 0, "length": 5}]
			},
			"reply_markup": {
				"inline_keyboard": [[
					{"text": "Open", "url": "https://example.com/open"},
					{"text": "Tap", "callback_data": "cb"}
				]]
			}
		}
	}`
	rec := performBotAPIRequest(t, h, bots.profile, "answerWebAppQuery", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("response ok = false: %s", rec.Body.String())
	}
	if !webapps.answerCalled || webapps.answerBotID != bots.profile.BotUserID || webapps.answerQueryID != "web-query-1" {
		t.Fatalf("answer call = %#v", webapps)
	}
	got := webapps.answerResult
	if got.ID != "share-1" || got.Type != "article" || got.Message != "hello mini app" || !got.NoWebpage || got.URL != "https://example.com/share" {
		t.Fatalf("result = %#v", got)
	}
	if len(got.Entities) != 1 || got.Entities[0].Type != domain.MessageEntityBold {
		t.Fatalf("entities = %#v", got.Entities)
	}
	if got.ReplyMarkup == nil || len(got.ReplyMarkup.Inline) != 1 || len(got.ReplyMarkup.Inline[0]) != 2 {
		t.Fatalf("reply markup = %#v", got.ReplyMarkup)
	}
}

func TestSavePreparedInlineMessageParsesPeerTypes(t *testing.T) {
	webapps := &fakeWebAppService{preparedID: "prepared-1", preparedExpire: 123456}
	bots := &fakeBotAPIBots{profile: domain.BotProfile{BotUserID: 1001, TokenSecret: "secret"}}
	h := (&handler{bots: bots, webapps: webapps}).routes()
	body := `{
		"user_id": 2001,
		"allow_user_chats": true,
		"allow_channel_chats": true,
		"result": {
			"type": "article",
			"id": "prepared-share",
			"title": "Prepared",
			"input_message_content": {"message_text": "share me"}
		}
	}`
	rec := performBotAPIRequest(t, h, bots.profile, "savePreparedInlineMessage", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !webapps.preparedCalled || webapps.preparedBotID != 1001 || webapps.preparedUserID != 2001 {
		t.Fatalf("prepared call = %#v", webapps)
	}
	wantPeers := []string{store.InlineQueryPeerTypePM, store.InlineQueryPeerTypeBroadcast}
	if !reflect.DeepEqual(webapps.preparedPeerTypes, wantPeers) {
		t.Fatalf("peer types = %#v, want %#v", webapps.preparedPeerTypes, wantPeers)
	}
	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			ID             string `json:"id"`
			ExpirationDate int    `json:"expiration_date"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || resp.Result.ID != "prepared-1" || resp.Result.ExpirationDate != 123456 {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestAnswerWebAppQueryRejectsUnsupportedResult(t *testing.T) {
	webapps := &fakeWebAppService{}
	bots := &fakeBotAPIBots{profile: domain.BotProfile{BotUserID: 1001, TokenSecret: "secret"}}
	h := (&handler{bots: bots, webapps: webapps}).routes()
	body := `{
		"web_app_query_id": "web-query-1",
		"result": {"type": "photo", "id": "bad", "photo_url": "https://example.com/p.jpg"}
	}`
	rec := performBotAPIRequest(t, h, bots.profile, "answerWebAppQuery", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if webapps.answerCalled {
		t.Fatalf("unsupported result should not call webapp service")
	}
	var resp apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.OK || resp.Description != "RESULT_TYPE_INVALID" {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func performBotAPIRequest(t *testing.T, h http.Handler, profile domain.BotProfile, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	token := domain.FormatBotToken(profile.BotUserID, profile.TokenSecret)
	req := httptest.NewRequest(http.MethodPost, "/bot"+token+"/"+method, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

type fakeBotAPIBots struct {
	profile domain.BotProfile
}

func (f *fakeBotAPIBots) BotInfo(context.Context, int64) (domain.BotProfile, bool, error) {
	return f.profile, true, nil
}

func (f *fakeBotAPIBots) SetBotMenuButton(context.Context, int64, domain.BotMenuButton) (int, error) {
	return 0, nil
}

func (f *fakeBotAPIBots) GetBotMenuButton(context.Context, int64) (domain.BotMenuButton, error) {
	return domain.BotMenuButton{Type: domain.BotMenuButtonDefault}, nil
}

func (f *fakeBotAPIBots) BotEmojiStatusPermission(context.Context, int64, int64) (bool, error) {
	return true, nil
}

type fakeWebAppService struct {
	answerCalled  bool
	answerBotID   int64
	answerQueryID string
	answerResult  domain.BotInlineResult

	preparedCalled    bool
	preparedBotID     int64
	preparedUserID    int64
	preparedResult    domain.BotInlineResult
	preparedPeerTypes []string
	preparedID        string
	preparedExpire    int
}

func (f *fakeWebAppService) AnswerWebAppQueryFromBotAPI(_ context.Context, botID int64, queryID string, result domain.BotInlineResult) (string, error) {
	f.answerCalled = true
	f.answerBotID = botID
	f.answerQueryID = queryID
	f.answerResult = result
	return "", nil
}

func (f *fakeWebAppService) SavePreparedInlineMessageFromBotAPI(_ context.Context, botID, userID int64, result domain.BotInlineResult, peerTypes []string) (string, int, error) {
	f.preparedCalled = true
	f.preparedBotID = botID
	f.preparedUserID = userID
	f.preparedResult = result
	f.preparedPeerTypes = append([]string(nil), peerTypes...)
	return f.preparedID, f.preparedExpire, nil
}
