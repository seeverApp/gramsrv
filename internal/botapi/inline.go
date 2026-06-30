package botapi

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"telesrv/internal/domain"
	"telesrv/internal/store"
)

func inlineResultFromAPI(raw string) (domain.BotInlineResult, error) {
	if strings.TrimSpace(raw) == "" {
		return domain.BotInlineResult{}, errors.New("RESULT_ID_INVALID")
	}
	var payload apiInlineResult
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return domain.BotInlineResult{}, errors.New("RESULT_TYPE_INVALID")
	}
	if payload.ID == "" {
		return domain.BotInlineResult{}, errors.New("RESULT_ID_EMPTY")
	}
	if len(payload.ID) > domain.MaxBotInlineResultIDLen {
		return domain.BotInlineResult{}, errors.New("RESULT_ID_INVALID")
	}
	if payload.Type != "article" {
		return domain.BotInlineResult{}, errors.New("RESULT_TYPE_INVALID")
	}
	if payload.URL != "" && !validBotAPIHTTPSURL(payload.URL) {
		return domain.BotInlineResult{}, errors.New("BUTTON_URL_INVALID")
	}
	message, entities, noWebpage, err := inputTextMessageContentFromAPI(payload)
	if err != nil {
		return domain.BotInlineResult{}, err
	}
	markup, err := replyMarkupFromAPI(payload.ReplyMarkup)
	if err != nil {
		return domain.BotInlineResult{}, err
	}
	return domain.BotInlineResult{
		ID:          payload.ID,
		Type:        payload.Type,
		Title:       payload.Title,
		Description: payload.Description,
		URL:         payload.URL,
		Message:     message,
		Entities:    entities,
		ReplyMarkup: markup,
		NoWebpage:   noWebpage,
	}, nil
}

func inputTextMessageContentFromAPI(payload apiInlineResult) (string, []domain.MessageEntity, bool, error) {
	var content apiInputTextMessageContent
	if len(payload.InputMessageContent) > 0 && string(payload.InputMessageContent) != "null" {
		if err := json.Unmarshal(payload.InputMessageContent, &content); err != nil {
			return "", nil, false, errors.New("MESSAGE_EMPTY")
		}
	} else if payload.MessageText != "" {
		content.MessageText = payload.MessageText
	}
	if content.ParseMode != "" {
		return "", nil, false, errors.New("ENTITY_PARSE_UNSUPPORTED")
	}
	message := content.MessageText
	if message == "" {
		return "", nil, false, errors.New("MESSAGE_EMPTY")
	}
	if utf8.RuneCountInString(message) > domain.MaxMessageTextLength {
		return "", nil, false, errors.New("MESSAGE_TOO_LONG")
	}
	entities, err := messageEntitiesFromAPI(content.Entities)
	if err != nil {
		return "", nil, false, err
	}
	noWebpage := content.DisableWebPagePreview || content.LinkPreviewOptions.IsDisabled
	return message, entities, noWebpage, nil
}

func messageEntitiesFromAPI(in []apiMessageEntity) ([]domain.MessageEntity, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > domain.MaxMessageEntityCount {
		return nil, errors.New("ENTITIES_TOO_LONG")
	}
	out := make([]domain.MessageEntity, 0, len(in))
	for _, entity := range in {
		if entity.Offset < 0 || entity.Length <= 0 {
			return nil, errors.New("ENTITY_BOUNDS_INVALID")
		}
		mapped, ok := apiEntityType(entity.Type)
		if !ok {
			return nil, errors.New("ENTITY_TYPE_UNSUPPORTED")
		}
		item := domain.MessageEntity{
			Type:     mapped,
			Offset:   entity.Offset,
			Length:   entity.Length,
			URL:      entity.URL,
			Language: entity.Language,
		}
		if entity.User != nil {
			item.UserID = entity.User.ID
		}
		if entity.CustomEmojiID != "" {
			id, err := strconv.ParseInt(entity.CustomEmojiID, 10, 64)
			if err != nil || id <= 0 {
				return nil, errors.New("ENTITY_TYPE_UNSUPPORTED")
			}
			item.DocumentID = id
		}
		out = append(out, item)
	}
	return out, nil
}

func apiEntityType(in string) (domain.MessageEntityType, bool) {
	switch in {
	case "bold":
		return domain.MessageEntityBold, true
	case "italic":
		return domain.MessageEntityItalic, true
	case "underline":
		return domain.MessageEntityUnderline, true
	case "strikethrough":
		return domain.MessageEntityStrike, true
	case "code":
		return domain.MessageEntityCode, true
	case "pre":
		return domain.MessageEntityPre, true
	case "text_link":
		return domain.MessageEntityTextURL, true
	case "text_mention":
		return domain.MessageEntityMentionName, true
	case "spoiler":
		return domain.MessageEntitySpoiler, true
	case "blockquote":
		return domain.MessageEntityBlockquote, true
	case "custom_emoji":
		return domain.MessageEntityCustomEmoji, true
	case "mention":
		return domain.MessageEntityMention, true
	case "hashtag":
		return domain.MessageEntityHashtag, true
	case "cashtag":
		return domain.MessageEntityCashtag, true
	case "bot_command":
		return domain.MessageEntityBotCommand, true
	case "url":
		return domain.MessageEntityURL, true
	case "email":
		return domain.MessageEntityEmail, true
	case "phone_number":
		return domain.MessageEntityPhone, true
	default:
		return "", false
	}
}

func replyMarkupFromAPI(raw json.RawMessage) (*domain.MessageReplyMarkup, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var payload apiInlineKeyboardMarkup
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, errors.New("BUTTON_INVALID")
	}
	if len(payload.InlineKeyboard) == 0 {
		return nil, nil
	}
	out := &domain.MessageReplyMarkup{Inline: make([][]domain.MarkupButton, 0, len(payload.InlineKeyboard))}
	for _, row := range payload.InlineKeyboard {
		domainRow := make([]domain.MarkupButton, 0, len(row))
		for _, button := range row {
			item, err := markupButtonFromAPI(button)
			if err != nil {
				return nil, err
			}
			domainRow = append(domainRow, item)
		}
		out.Inline = append(out.Inline, domainRow)
	}
	if err := domain.ValidateReplyMarkup(out); err != nil {
		return nil, replyMarkupErrFromDomain(err)
	}
	if out.IsZero() {
		return nil, nil
	}
	return out, nil
}

func markupButtonFromAPI(button apiInlineKeyboardButton) (domain.MarkupButton, error) {
	if button.URL != "" {
		return domain.MarkupButton{Type: domain.MarkupButtonURL, Text: button.Text, URL: button.URL}, nil
	}
	if button.CallbackData != nil {
		if *button.CallbackData == "" || len([]byte(*button.CallbackData)) > domain.MaxCallbackDataLen {
			return domain.MarkupButton{}, errors.New("BUTTON_DATA_INVALID")
		}
		return domain.MarkupButton{Type: domain.MarkupButtonCallback, Text: button.Text, Data: []byte(*button.CallbackData)}, nil
	}
	return domain.MarkupButton{}, errors.New("BUTTON_INVALID")
}

func replyMarkupErrFromDomain(err error) error {
	switch {
	case errors.Is(err, domain.ErrButtonURLInvalid):
		return errors.New("BUTTON_URL_INVALID")
	case errors.Is(err, domain.ErrButtonDataInvalid):
		return errors.New("BUTTON_DATA_INVALID")
	default:
		return errors.New("BUTTON_INVALID")
	}
}

func preparedPeerTypesFromAPI(values map[string]string) []string {
	out := make([]string, 0, 5)
	if apiBool(values["allow_user_chats"]) {
		out = append(out, store.InlineQueryPeerTypePM)
	}
	if apiBool(values["allow_bot_chats"]) {
		out = append(out, store.InlineQueryPeerTypeBotPM)
	}
	if apiBool(values["allow_group_chats"]) {
		out = append(out, store.InlineQueryPeerTypeChat, store.InlineQueryPeerTypeMegagroup)
	}
	if apiBool(values["allow_channel_chats"]) {
		out = append(out, store.InlineQueryPeerTypeBroadcast)
	}
	return out
}

func apiBool(raw string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	return err == nil && v
}

func validBotAPIHTTPSURL(raw string) bool {
	if raw == "" || len(raw) > domain.MaxBotInlineWebURLLen || strings.TrimSpace(raw) != raw {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" || strings.ToLower(parsed.Scheme) != "https" {
		return false
	}
	return !strings.ContainsAny(parsed.Host, " \t\r\n")
}

type apiInlineResult struct {
	Type                string          `json:"type"`
	ID                  string          `json:"id"`
	Title               string          `json:"title"`
	Description         string          `json:"description"`
	URL                 string          `json:"url"`
	MessageText         string          `json:"message_text"`
	InputMessageContent json.RawMessage `json:"input_message_content"`
	ReplyMarkup         json.RawMessage `json:"reply_markup"`
}

type apiInputTextMessageContent struct {
	MessageText           string             `json:"message_text"`
	ParseMode             string             `json:"parse_mode"`
	Entities              []apiMessageEntity `json:"entities"`
	DisableWebPagePreview bool               `json:"disable_web_page_preview"`
	LinkPreviewOptions    struct {
		IsDisabled bool `json:"is_disabled"`
	} `json:"link_preview_options"`
}

type apiMessageEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url"`
	User   *struct {
		ID int64 `json:"id"`
	} `json:"user"`
	Language      string `json:"language"`
	CustomEmojiID string `json:"custom_emoji_id"`
}

type apiInlineKeyboardMarkup struct {
	InlineKeyboard [][]apiInlineKeyboardButton `json:"inline_keyboard"`
}

type apiInlineKeyboardButton struct {
	Text         string  `json:"text"`
	URL          string  `json:"url"`
	CallbackData *string `json:"callback_data"`
}
