package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"

	"telesrv/internal/domain"
)

// TestWebPagePreviewMedia 验证 getWebPagePreview 的 media 决策：done 卡片→messageMediaWebPage，
// 无链接/解析失败/非 done 一律 messageMediaEmpty（绝不返回 pending、绝不报错）。
func TestWebPagePreviewMedia(t *testing.T) {
	ctx := context.Background()
	urlEntities := []tg.MessageEntityClass{&tg.MessageEntityURL{Offset: 0, Length: 9}} // "https://x"

	doneCard := domain.MessageWebPage{
		State:      domain.MessageWebPageStateDone,
		ID:         42,
		URL:        "https://x",
		DisplayURL: "x",
		Title:      "Hello",
		Photo: &domain.Photo{
			ID: 7, AccessHash: 8, DCID: 2,
			Sizes: []domain.PhotoSize{{Kind: domain.PhotoSizeKindDefault, Type: "x", W: 320, H: 200, Size: 1024}},
		},
	}

	files := &fakeFiles{resolveWebPageFn: func(string) (domain.MessageWebPage, error) { return doneCard, nil }}
	r := &Router{deps: Deps{Files: files}}

	t.Run("done-card", func(t *testing.T) {
		media := r.webPagePreviewMedia(ctx, "https://x", urlEntities)
		wrap, ok := media.(*tg.MessageMediaWebPage)
		if !ok {
			t.Fatalf("media = %T, want *tg.MessageMediaWebPage", media)
		}
		page, ok := wrap.Webpage.(*tg.WebPage)
		if !ok {
			t.Fatalf("webpage = %T, want *tg.WebPage", wrap.Webpage)
		}
		if v, _ := page.GetTitle(); v != "Hello" {
			t.Fatalf("title = %q, want Hello", v)
		}
	})

	t.Run("no-url", func(t *testing.T) {
		if media := r.webPagePreviewMedia(ctx, "plain text", nil); !isEmptyMedia(media) {
			t.Fatalf("media = %T, want empty", media)
		}
	})

	t.Run("resolve-error-degrades", func(t *testing.T) {
		files.resolveWebPageFn = func(string) (domain.MessageWebPage, error) {
			return domain.MessageWebPage{}, errors.New("boom")
		}
		if media := r.webPagePreviewMedia(ctx, "https://x", urlEntities); !isEmptyMedia(media) {
			t.Fatalf("media = %T, want empty on error", media)
		}
	})

	t.Run("non-done-state-degrades", func(t *testing.T) {
		files.resolveWebPageFn = func(string) (domain.MessageWebPage, error) {
			return domain.MessageWebPage{State: domain.MessageWebPageStateEmpty, ID: 1, URL: "https://x"}, nil
		}
		if media := r.webPagePreviewMedia(ctx, "https://x", urlEntities); !isEmptyMedia(media) {
			t.Fatalf("media = %T, want empty for empty-state webpage", media)
		}
	})
}

func isEmptyMedia(m tg.MessageMediaClass) bool {
	_, ok := m.(*tg.MessageMediaEmpty)
	return ok
}
