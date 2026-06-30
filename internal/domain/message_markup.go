package domain

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	// MaxMarkupRows 限制 inline keyboard 行数。
	MaxMarkupRows = 100
	// MaxMarkupButtonsPerRow 限制单行按钮数。
	MaxMarkupButtonsPerRow = 8
	// MaxMarkupButtonsTotal 限制 inline keyboard 总按钮数（对齐官方约 100）。
	MaxMarkupButtonsTotal = 100
	// MaxCallbackDataLen 是 callback 按钮 data 的字节上限（对齐 Bot API 1-64）。
	MaxCallbackDataLen = 64
	// MaxMarkupButtonTextLen 是按钮文本长度上限（rune 计数）。
	MaxMarkupButtonTextLen = 256
	// MaxBotCallbackAnswerLen 是 callback answer 弹窗/toast 文本上限。
	MaxBotCallbackAnswerLen = 200
	// MaxStartParamLen 是 messages.startBot 深链 payload 上限（对齐官方 64）。
	MaxStartParamLen = 64
)

// markup / callback 业务错误。
var (
	// ErrButtonDataInvalid 表示 callback data 越界（>64 字节）。
	ErrButtonDataInvalid = errors.New("button data invalid")
	// ErrButtonInvalid 表示键盘结构非法（行/按钮数超限、文本空/过长）。
	ErrButtonInvalid = errors.New("button invalid")
	// ErrButtonURLInvalid 表示 url 按钮的链接非法（非 https）。
	ErrButtonURLInvalid = errors.New("button url invalid")
	// ErrButtonTypeInvalid 表示按钮类型 P3 不支持（webview/game/url_auth/request_* 等）。
	ErrButtonTypeInvalid = errors.New("button type invalid")
	// ErrStartParamInvalid 表示 startBot 的 start_param 越界。
	ErrStartParamInvalid = errors.New("start param invalid")
)

// MarkupButtonType 标识 P3 支持的 inline 按钮类型。
type MarkupButtonType string

const (
	// MarkupButtonCallback 是 keyboardButtonCallback（点击触发 getBotCallbackAnswer）。
	MarkupButtonCallback MarkupButtonType = "callback"
	// MarkupButtonURL 是 keyboardButtonUrl（点击打开链接）。
	MarkupButtonURL MarkupButtonType = "url"
)

// MarkupButton 是一颗 inline keyboard 按钮（P3 仅 callback/url）。
type MarkupButton struct {
	Type MarkupButtonType `json:"type"`
	Text string           `json:"text"`
	// Data 仅 callback 使用：原始字节（含 0x00/非 UTF-8/高位）。json 自动 base64
	// 编解码，保证经 JSONB 列字节级 round-trip（updateBotCallbackQuery.data 须原样）。
	Data []byte `json:"data,omitempty"`
	// URL 仅 url 使用。
	URL string `json:"url,omitempty"`
	// RequiresPassword 仅 callback 使用（keyboardButtonCallback.requires_password，
	// 2FA SRP 校验 P3 stub）。
	RequiresPassword bool `json:"requires_password,omitempty"`
}

// MessageReplyMarkup 是消息携带的 inline keyboard 快照（P3 仅 ReplyInlineMarkup）。
type MessageReplyMarkup struct {
	Inline [][]MarkupButton `json:"inline,omitempty"`
}

// IsZero 报告 markup 是否为空（无任何按钮）。空 markup 不写 wire flag、不入库。
func (m *MessageReplyMarkup) IsZero() bool {
	if m == nil {
		return true
	}
	for _, row := range m.Inline {
		if len(row) > 0 {
			return false
		}
	}
	return true
}

// ValidateReplyMarkup 校验 inline keyboard 结构与各按钮，校验须先于落库（I9）。
// 空 markup 合法（视为清空/无键盘）。
func ValidateReplyMarkup(m *MessageReplyMarkup) error {
	if m == nil {
		return nil
	}
	if len(m.Inline) > MaxMarkupRows {
		return ErrButtonInvalid
	}
	total := 0
	for _, row := range m.Inline {
		if len(row) > MaxMarkupButtonsPerRow {
			return ErrButtonInvalid
		}
		total += len(row)
		if total > MaxMarkupButtonsTotal {
			return ErrButtonInvalid
		}
		for i := range row {
			if err := validateMarkupButton(row[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMarkupButton(b MarkupButton) error {
	text := strings.TrimSpace(b.Text)
	if text == "" || utf8.RuneCountInString(b.Text) > MaxMarkupButtonTextLen {
		return ErrButtonInvalid
	}
	switch b.Type {
	case MarkupButtonCallback:
		if len(b.Data) > MaxCallbackDataLen {
			return ErrButtonDataInvalid
		}
	case MarkupButtonURL:
		if err := validateButtonURL(b.URL); err != nil {
			return err
		}
	default:
		// webview/game/url_auth/request_* 等 P3 未实现类型：拒绝，绝不半实现下发。
		return ErrButtonTypeInvalid
	}
	return nil
}

func validateButtonURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > MaxBotMenuButtonURLLen {
		return ErrButtonURLInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ErrButtonURLInvalid
	}
	return nil
}

// BotCallbackAnswer 是 bot 对一次 callback query 的应答（setBotCallbackAnswer →
// 解挂等待中的 getBotCallbackAnswer）。
type BotCallbackAnswer struct {
	Alert     bool
	Message   string
	URL       string
	CacheTime int
}
