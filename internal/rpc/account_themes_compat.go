package rpc

import (
	"context"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// DrKLO 12.8.1(Layer 227)发出的 theme 方法构造器比 gotd v0.158 的 schema 更新:
// gotd 的 ServerDispatcher 按构造器 id 匹配,这些「新」id 匹配不上而落到 fallback。
// 字段语义与 gotd 请求结构相同(仅 createTheme/updateTheme 的 settings 在线为单个对象、
// gotd 为 Vector;getTheme 多一个被忽略的 document_id),故这里按 DrKLO 字段序手动解码、
// 复用现有 handler。uploadTheme(0x1c3db333)/saveTheme(0xf257106c) 构造器与 gotd 一致,
// 无需 compat。
const (
	legacyCreateThemeID  = 0x8432c21f
	legacyUpdateThemeID  = 0x5cb367d5
	legacyInstallThemeID = 0x7ae43737
	legacyGetThemeID     = 0x8d9d742b
)

// tryLegacyThemeRPC 在 fallback 中尝试处理 DrKLO 的新版 theme 构造器。
// handled=false 表示不是这些构造器,调用方继续原 fallback 流程。
func (r *Router) tryLegacyThemeRPC(ctx context.Context, b *bin.Buffer) (enc bin.Encoder, handled bool, err error) {
	id, perr := b.PeekID()
	if perr != nil {
		return nil, false, nil
	}
	switch id {
	case legacyCreateThemeID:
		enc, err = r.legacyCreateTheme(ctx, b)
	case legacyUpdateThemeID:
		enc, err = r.legacyUpdateTheme(ctx, b)
	case legacyInstallThemeID:
		enc, err = r.legacyInstallTheme(ctx, b)
	case legacyGetThemeID:
		enc, err = r.legacyGetTheme(ctx, b)
	default:
		return nil, false, nil
	}
	return enc, true, err
}

func boolEncoder(v bool) bin.Encoder {
	if v {
		return &tg.BoolTrue{}
	}
	return &tg.BoolFalse{}
}

func (r *Router) legacyCreateTheme(ctx context.Context, b *bin.Buffer) (bin.Encoder, error) {
	if _, err := b.ID(); err != nil {
		return nil, inputConstructorInvalidErr()
	}
	flags, err := b.Int32()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	slug, err := b.String()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	title, err := b.String()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	req := &tg.AccountCreateThemeRequest{Slug: slug, Title: title}
	if flags&(1<<2) != 0 {
		doc, err := tg.DecodeInputDocument(b)
		if err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetDocument(doc)
	}
	if flags&(1<<3) != 0 {
		var s tg.InputThemeSettings
		if err := s.Decode(b); err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetSettings([]tg.InputThemeSettings{s})
	}
	return r.onAccountCreateTheme(ctx, req)
}

func (r *Router) legacyUpdateTheme(ctx context.Context, b *bin.Buffer) (bin.Encoder, error) {
	if _, err := b.ID(); err != nil {
		return nil, inputConstructorInvalidErr()
	}
	flags, err := b.Int32()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	format, err := b.String()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	theme, err := tg.DecodeInputTheme(b)
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	req := &tg.AccountUpdateThemeRequest{Format: format, Theme: theme}
	if flags&(1<<0) != 0 {
		slug, err := b.String()
		if err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetSlug(slug)
	}
	if flags&(1<<1) != 0 {
		title, err := b.String()
		if err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetTitle(title)
	}
	if flags&(1<<2) != 0 {
		doc, err := tg.DecodeInputDocument(b)
		if err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetDocument(doc)
	}
	if flags&(1<<3) != 0 {
		var s tg.InputThemeSettings
		if err := s.Decode(b); err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetSettings([]tg.InputThemeSettings{s})
	}
	return r.onAccountUpdateTheme(ctx, req)
}

func (r *Router) legacyInstallTheme(ctx context.Context, b *bin.Buffer) (bin.Encoder, error) {
	if _, err := b.ID(); err != nil {
		return nil, inputConstructorInvalidErr()
	}
	flags, err := b.Int32()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	req := &tg.AccountInstallThemeRequest{}
	if flags&(1<<0) != 0 {
		req.SetDark(true)
	}
	// bit1 同时门控 format 与 theme(DrKLO 布局)。
	if flags&(1<<1) != 0 {
		format, err := b.String()
		if err != nil {
			return nil, inputConstructorInvalidErr()
		}
		theme, err := tg.DecodeInputTheme(b)
		if err != nil {
			return nil, inputConstructorInvalidErr()
		}
		req.SetFormat(format)
		req.SetTheme(theme)
	}
	ok, err := r.onAccountInstallTheme(ctx, req)
	if err != nil {
		return nil, err
	}
	return boolEncoder(ok), nil
}

func (r *Router) legacyGetTheme(ctx context.Context, b *bin.Buffer) (bin.Encoder, error) {
	if _, err := b.ID(); err != nil {
		return nil, inputConstructorInvalidErr()
	}
	format, err := b.String()
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	theme, err := tg.DecodeInputTheme(b)
	if err != nil {
		return nil, inputConstructorInvalidErr()
	}
	if _, err := b.Long(); err != nil { // document_id:被服务端忽略
		return nil, inputConstructorInvalidErr()
	}
	req := &tg.AccountGetThemeRequest{Format: format, Theme: theme}
	return r.onAccountGetTheme(ctx, req)
}
