package domain

import (
	"errors"
	"strings"
	"testing"
)

func cb(text string, data []byte) MarkupButton {
	return MarkupButton{Type: MarkupButtonCallback, Text: text, Data: data}
}

func TestValidateReplyMarkup(t *testing.T) {
	tests := []struct {
		name string
		m    *MessageReplyMarkup
		want error
	}{
		{"nil ok", nil, nil},
		{"empty ok", &MessageReplyMarkup{}, nil},
		{"callback ok", &MessageReplyMarkup{Inline: [][]MarkupButton{{cb("ok", []byte("d"))}}}, nil},
		{"callback 64B ok", &MessageReplyMarkup{Inline: [][]MarkupButton{{cb("ok", make([]byte, 64))}}}, nil},
		{"callback 65B bad", &MessageReplyMarkup{Inline: [][]MarkupButton{{cb("ok", make([]byte, 65))}}}, ErrButtonDataInvalid},
		{"empty text bad", &MessageReplyMarkup{Inline: [][]MarkupButton{{cb("", []byte("d"))}}}, ErrButtonInvalid},
		{"url https ok", &MessageReplyMarkup{Inline: [][]MarkupButton{{{Type: MarkupButtonURL, Text: "go", URL: "https://example.com/x"}}}}, nil},
		{"url http bad", &MessageReplyMarkup{Inline: [][]MarkupButton{{{Type: MarkupButtonURL, Text: "go", URL: "http://example.com"}}}}, ErrButtonURLInvalid},
		{"url javascript bad", &MessageReplyMarkup{Inline: [][]MarkupButton{{{Type: MarkupButtonURL, Text: "go", URL: "javascript:alert(1)"}}}}, ErrButtonURLInvalid},
		{"url empty bad", &MessageReplyMarkup{Inline: [][]MarkupButton{{{Type: MarkupButtonURL, Text: "go", URL: ""}}}}, ErrButtonURLInvalid},
		{"unknown type bad", &MessageReplyMarkup{Inline: [][]MarkupButton{{{Type: "webview", Text: "x"}}}}, ErrButtonTypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateReplyMarkup(tt.m); !errors.Is(err, tt.want) {
				t.Fatalf("ValidateReplyMarkup = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateReplyMarkupLimits(t *testing.T) {
	// 行数上限。
	tooManyRows := &MessageReplyMarkup{Inline: make([][]MarkupButton, MaxMarkupRows+1)}
	for i := range tooManyRows.Inline {
		tooManyRows.Inline[i] = []MarkupButton{cb("x", []byte("d"))}
	}
	if err := ValidateReplyMarkup(tooManyRows); !errors.Is(err, ErrButtonInvalid) {
		t.Fatalf("rows over limit = %v, want ErrButtonInvalid", err)
	}
	// 单行按钮数上限。
	wideRow := make([]MarkupButton, MaxMarkupButtonsPerRow+1)
	for i := range wideRow {
		wideRow[i] = cb("x", []byte("d"))
	}
	if err := ValidateReplyMarkup(&MessageReplyMarkup{Inline: [][]MarkupButton{wideRow}}); !errors.Is(err, ErrButtonInvalid) {
		t.Fatalf("row width over limit = %v, want ErrButtonInvalid", err)
	}
	// 文本长度上限。
	longText := strings.Repeat("a", MaxMarkupButtonTextLen+1)
	if err := ValidateReplyMarkup(&MessageReplyMarkup{Inline: [][]MarkupButton{{cb(longText, []byte("d"))}}}); !errors.Is(err, ErrButtonInvalid) {
		t.Fatalf("text over limit = %v, want ErrButtonInvalid", err)
	}
}

func TestMessageReplyMarkupIsZero(t *testing.T) {
	if !(*MessageReplyMarkup)(nil).IsZero() {
		t.Fatal("nil markup must be zero")
	}
	if !(&MessageReplyMarkup{}).IsZero() {
		t.Fatal("empty markup must be zero")
	}
	if !(&MessageReplyMarkup{Inline: [][]MarkupButton{{}}}).IsZero() {
		t.Fatal("markup with only empty rows must be zero")
	}
	if (&MessageReplyMarkup{Inline: [][]MarkupButton{{cb("x", nil)}}}).IsZero() {
		t.Fatal("markup with a button must not be zero")
	}
}
