package rpc

import (
	"context"
	"testing"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"

	"telesrv/internal/domain"
)

// TestLegacyThemeWireDecode 验证按 DrKLO 12.8.1 的 theme 构造器(比 gotd schema 新)
// 手写解码后能正确复用现有 handler。直接构造 DrKLO 的 wire 字节喂给 fallback compat 层。
func TestLegacyThemeWireDecode(t *testing.T) {
	const userID = 1000010
	ctx := WithUserID(context.Background(), userID)
	files := &fakeFiles{docs: map[int64]domain.Document{
		777: {ID: 777, AccessHash: 7, DCID: 2, MimeType: "application/x-tgtheme-android", Size: 4096},
	}}
	r := newThemeRouter(t, files)

	// createTheme 0x8432c21f:flags(=4,document) + slug + title + InputDocument。
	var cb bin.Buffer
	cb.PutID(legacyCreateThemeID)
	cb.PutInt32(1 << 2) // document present
	cb.PutString("")    // empty slug → auto
	cb.PutString("Legacy Theme")
	(&tg.InputDocument{ID: 777, AccessHash: 7}).Encode(&cb)

	enc, handled, err := r.tryLegacyThemeRPC(ctx, &cb)
	if !handled || err != nil {
		t.Fatalf("createTheme legacy = handled %v err %v", handled, err)
	}
	th, ok := enc.(*tg.Theme)
	if !ok {
		t.Fatalf("createTheme legacy result = %T, want *tg.Theme", enc)
	}
	if th.Slug == "" || !th.GetCreator() {
		t.Fatalf("created theme = slug %q creator %v, want auto slug + creator", th.Slug, th.GetCreator())
	}
	if doc, ok := th.GetDocument(); !ok {
		t.Fatalf("created theme missing document")
	} else if d, _ := doc.(*tg.Document); d == nil || d.ID != 777 {
		t.Fatalf("created theme document = %#v, want id 777", doc)
	}
	mustEncodeTheme(t, th)
	slug := th.Slug

	// getTheme 0x8d9d742b:format + InputThemeSlug + document_id(被忽略)。
	var gb bin.Buffer
	gb.PutID(legacyGetThemeID)
	gb.PutString("android")
	(&tg.InputThemeSlug{Slug: slug}).Encode(&gb)
	gb.PutLong(12345) // document_id ignored

	enc, handled, err = r.tryLegacyThemeRPC(ctx, &gb)
	if !handled || err != nil {
		t.Fatalf("getTheme legacy = handled %v err %v", handled, err)
	}
	got, ok := enc.(*tg.Theme)
	if !ok || got.Slug != slug {
		t.Fatalf("getTheme legacy result = %#v, want theme slug %q", enc, slug)
	}
	if _, ok := got.GetDocument(); !ok {
		t.Fatalf("getTheme by slug missing document → client ThemeNotSupported")
	}

	// installTheme 0x7ae43737:flags(dark@bit0 + bit1 gates format+theme)。
	var ib bin.Buffer
	ib.PutID(legacyInstallThemeID)
	ib.PutInt32((1 << 0) | (1 << 1)) // dark + has format/theme
	ib.PutString("android")
	(&tg.InputThemeSlug{Slug: slug}).Encode(&ib)

	enc, handled, err = r.tryLegacyThemeRPC(ctx, &ib)
	if !handled || err != nil {
		t.Fatalf("installTheme legacy = handled %v err %v", handled, err)
	}
	if _, ok := enc.(*tg.BoolTrue); !ok {
		t.Fatalf("installTheme legacy result = %T, want *tg.BoolTrue", enc)
	}

	// 非 theme 构造器 → 不处理。
	var ob bin.Buffer
	ob.PutID(0x12345678)
	if _, handled, _ := r.tryLegacyThemeRPC(ctx, &ob); handled {
		t.Fatalf("unrelated ctor should not be handled")
	}
}
