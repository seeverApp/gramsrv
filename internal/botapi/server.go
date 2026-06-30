package botapi

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"telesrv/internal/domain"
)

type BotsService interface {
	BotInfo(ctx context.Context, botUserID int64) (domain.BotProfile, bool, error)
	SetBotMenuButton(ctx context.Context, botUserID int64, button domain.BotMenuButton) (int, error)
	GetBotMenuButton(ctx context.Context, botUserID int64) (domain.BotMenuButton, error)
	BotEmojiStatusPermission(ctx context.Context, botUserID, userID int64) (bool, error)
}

type UsersService interface {
	UpdateEmojiStatus(ctx context.Context, userID int64, documentID int64, until int) (domain.User, error)
}

type WebAppService interface {
	AnswerWebAppQueryFromBotAPI(ctx context.Context, botID int64, webAppQueryID string, result domain.BotInlineResult) (inlineMessageID string, err error)
	SavePreparedInlineMessageFromBotAPI(ctx context.Context, botID, userID int64, result domain.BotInlineResult, peerTypes []string) (id string, expireDate int, err error)
}

func Start(ctx context.Context, addr string, bots BotsService, users UsersService, webapps WebAppService, logger *zap.Logger) (*http.Server, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	handler := &handler{bots: bots, users: users, webapps: webapps, logger: logger}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		logger.Info("Bot API 网关已启用", zap.String("addr", addr))
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warn("Bot API 网关退出", zap.Error(err))
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	return srv, nil
}

type handler struct {
	bots    BotsService
	users   UsersService
	webapps WebAppService
	logger  *zap.Logger
}

func (h *handler) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handle)
	return mux
}

func (h *handler) handle(w http.ResponseWriter, r *http.Request) {
	token, method, ok := splitBotPath(r.URL.Path)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "METHOD_NOT_FOUND")
		return
	}
	botID, ok := h.authenticate(r.Context(), token)
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "ACCESS_TOKEN_INVALID")
		return
	}
	switch strings.ToLower(method) {
	case "setchatmenubutton":
		h.setChatMenuButton(w, r, botID)
	case "getchatmenubutton":
		h.getChatMenuButton(w, r, botID)
	case "setuseremojistatus":
		h.setUserEmojiStatus(w, r, botID)
	case "answerwebappquery":
		h.answerWebAppQuery(w, r, botID)
	case "savepreparedinlinemessage":
		h.savePreparedInlineMessage(w, r, botID)
	case "answershippingquery", "answerprecheckoutquery":
		writeAPIError(w, http.StatusNotImplemented, "BLOCKED_DURABLE_QUERY_STATE_MISSING")
	default:
		writeAPIError(w, http.StatusNotFound, "METHOD_NOT_FOUND")
	}
}

func splitBotPath(path string) (token, method string, ok bool) {
	rest := strings.TrimPrefix(path, "/bot")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		return "", "", false
	}
	token, method, found := strings.Cut(rest, "/")
	if !found || token == "" || method == "" {
		return "", "", false
	}
	return token, method, true
}

func (h *handler) authenticate(ctx context.Context, token string) (int64, bool) {
	if h.bots == nil {
		return 0, false
	}
	botID, secret, ok := domain.ParseBotToken(token)
	if !ok {
		return 0, false
	}
	profile, found, err := h.bots.BotInfo(ctx, botID)
	if err != nil || !found || profile.TokenSecret != secret {
		return 0, false
	}
	return botID, true
}

func (h *handler) setChatMenuButton(w http.ResponseWriter, r *http.Request, botID int64) {
	if h.bots == nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	values, err := requestValues(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	button, err := menuButtonFromAPI(values["menu_button"])
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "BUTTON_INVALID")
		return
	}
	if _, err := h.bots.SetBotMenuButton(r.Context(), botID, button); err != nil {
		writeAPIError(w, http.StatusBadRequest, "BUTTON_INVALID")
		return
	}
	writeAPIOK(w, true)
}

func (h *handler) getChatMenuButton(w http.ResponseWriter, r *http.Request, botID int64) {
	if h.bots == nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	button, err := h.bots.GetBotMenuButton(r.Context(), botID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	writeAPIOK(w, apiMenuButton(button))
}

func (h *handler) setUserEmojiStatus(w http.ResponseWriter, r *http.Request, botID int64) {
	if h.bots == nil || h.users == nil {
		writeAPIError(w, http.StatusNotImplemented, "BLOCKED_USER_EMOJI_STATUS_SERVICE_MISSING")
		return
	}
	values, err := requestValues(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	userID, err := strconv.ParseInt(values["user_id"], 10, 64)
	if err != nil || userID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "USER_ID_INVALID")
		return
	}
	allowed, err := h.bots.BotEmojiStatusPermission(r.Context(), botID, userID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	if !allowed {
		writeAPIError(w, http.StatusForbidden, "USER_PERMISSION_DENIED")
		return
	}
	var documentID int64
	if raw := strings.TrimSpace(values["emoji_status_custom_emoji_id"]); raw != "" {
		documentID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || documentID < 0 {
			writeAPIError(w, http.StatusBadRequest, "EMOJI_STATUS_INVALID")
			return
		}
	}
	var until int
	if raw := strings.TrimSpace(values["emoji_status_expiration_date"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeAPIError(w, http.StatusBadRequest, "EMOJI_STATUS_INVALID")
			return
		}
		until = n
	}
	if _, err := h.users.UpdateEmojiStatus(r.Context(), userID, documentID, until); err != nil {
		if errors.Is(err, domain.ErrPremiumRequired) {
			writeAPIError(w, http.StatusBadRequest, "PREMIUM_ACCOUNT_REQUIRED")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
		return
	}
	writeAPIOK(w, true)
}

func (h *handler) answerWebAppQuery(w http.ResponseWriter, r *http.Request, botID int64) {
	if h.webapps == nil {
		writeAPIError(w, http.StatusNotImplemented, "BLOCKED_WEBAPP_QUERY_SERVICE_MISSING")
		return
	}
	values, err := requestValues(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	queryID := strings.TrimSpace(values["web_app_query_id"])
	if queryID == "" {
		writeAPIError(w, http.StatusBadRequest, "QUERY_ID_INVALID")
		return
	}
	result, err := inlineResultFromAPI(values["result"])
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	inlineID, err := h.webapps.AnswerWebAppQueryFromBotAPI(r.Context(), botID, queryID, result)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiErrorDescription(err))
		return
	}
	resp := map[string]any{}
	if inlineID != "" {
		resp["inline_message_id"] = inlineID
	}
	writeAPIOK(w, resp)
}

func (h *handler) savePreparedInlineMessage(w http.ResponseWriter, r *http.Request, botID int64) {
	if h.webapps == nil {
		writeAPIError(w, http.StatusNotImplemented, "BLOCKED_PREPARED_INLINE_SERVICE_MISSING")
		return
	}
	values, err := requestValues(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	userID, err := strconv.ParseInt(values["user_id"], 10, 64)
	if err != nil || userID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "USER_ID_INVALID")
		return
	}
	result, err := inlineResultFromAPI(values["result"])
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	peerTypes := preparedPeerTypesFromAPI(values)
	id, expireDate, err := h.webapps.SavePreparedInlineMessageFromBotAPI(r.Context(), botID, userID, result, peerTypes)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apiErrorDescription(err))
		return
	}
	writeAPIOK(w, map[string]any{"id": id, "expiration_date": expireDate})
}

func requestValues(r *http.Request) (map[string]string, error) {
	out := map[string]string{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			return nil, err
		}
		for k, v := range body {
			switch x := v.(type) {
			case string:
				out[k] = x
			default:
				b, _ := json.Marshal(x)
				out[k] = string(b)
			}
		}
		if _, nested := out["menu_button"]; !nested {
			if _, direct := body["type"]; direct {
				b, _ := json.Marshal(body)
				out["menu_button"] = string(b)
			}
		}
		return out, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for k, v := range r.Form {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out, nil
}

func menuButtonFromAPI(raw string) (domain.BotMenuButton, error) {
	if strings.TrimSpace(raw) == "" {
		return domain.BotMenuButton{Type: domain.BotMenuButtonDefault}, nil
	}
	var payload struct {
		Type   string `json:"type"`
		Text   string `json:"text"`
		WebApp struct {
			URL string `json:"url"`
		} `json:"web_app"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return domain.BotMenuButton{}, err
	}
	switch payload.Type {
	case "default":
		return domain.BotMenuButton{Type: domain.BotMenuButtonDefault}, nil
	case "commands":
		return domain.BotMenuButton{Type: domain.BotMenuButtonCommands}, nil
	case "web_app":
		return domain.BotMenuButton{Type: domain.BotMenuButtonWebView, Text: payload.Text, URL: payload.WebApp.URL}, nil
	default:
		return domain.BotMenuButton{}, fmt.Errorf("unknown menu button type")
	}
}

func apiMenuButton(button domain.BotMenuButton) map[string]any {
	switch button.Type {
	case domain.BotMenuButtonCommands:
		return map[string]any{"type": "commands"}
	case domain.BotMenuButtonWebView:
		return map[string]any{
			"type": "web_app",
			"text": button.Text,
			"web_app": map[string]any{
				"url": button.URL,
			},
		}
	default:
		return map[string]any{"type": "default"}
	}
}

func writeAPIOK(w http.ResponseWriter, result any) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
}

func writeAPIError(w http.ResponseWriter, status int, description string) {
	writeJSON(w, status, map[string]any{"ok": false, "error_code": status, "description": description})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func apiErrorDescription(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToUpper(err.Error())
	for _, marker := range []string{
		"QUERY_ID_INVALID",
		"USER_ID_INVALID",
		"RESULT_ID_INVALID",
		"RESULT_ID_EMPTY",
		"RESULT_TYPE_INVALID",
		"MESSAGE_EMPTY",
		"MESSAGE_TOO_LONG",
		"BUTTON_INVALID",
		"BUTTON_DATA_INVALID",
		"BUTTON_URL_INVALID",
		"BOT_INVALID",
		"USER_BOT_REQUIRED",
	} {
		if strings.Contains(text, marker) {
			return marker
		}
	}
	return "BAD_REQUEST"
}

func randomNonZeroInt64() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	v := int64(binary.LittleEndian.Uint64(b[:]) & 0x7fffffffffffffff)
	if v == 0 {
		return time.Now().UnixNano()
	}
	return v
}
