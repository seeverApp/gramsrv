package rpc

import (
	"context"
	"testing"

	"github.com/gotd/td/clock"
	"github.com/gotd/td/tg"
	"go.uber.org/zap/zaptest"

	"telesrv/internal/domain"
)

func TestAccountGetDefaultProfilePhotoEmojisUsesSeededEmojiSets(t *testing.T) {
	files := &fakeFiles{sets: map[domain.StickerSetKind][]domain.StickerSet{
		domain.StickerSetKindEmoji: {
			{DocumentIDs: []int64{1001, 0, 1002, 1001}},
			{DocumentIDs: []int64{1003}},
		},
		domain.StickerSetKindSystem: {
			{SystemKey: "animated_emoji", DocumentIDs: []int64{2001}},
		},
	}}
	r := New(Config{}, Deps{Files: files}, zaptest.NewLogger(t), clock.System)

	got, err := r.onAccountGetDefaultProfilePhotoEmojis(context.Background(), 0)
	if err != nil {
		t.Fatalf("get default profile photo emojis: %v", err)
	}
	list, ok := got.(*tg.EmojiList)
	if !ok {
		t.Fatalf("default profile photo emojis = %T, want *tg.EmojiList", got)
	}
	if len(list.DocumentID) != 3 || list.DocumentID[0] != 1001 || list.DocumentID[1] != 1002 || list.DocumentID[2] != 1003 {
		t.Fatalf("document ids = %v, want deduped seeded emoji ids", list.DocumentID)
	}
	if list.Hash == 0 {
		t.Fatal("emoji list hash = 0, want stable non-zero hash")
	}

	cached, err := r.onAccountGetDefaultProfilePhotoEmojis(context.Background(), list.Hash)
	if err != nil {
		t.Fatalf("get cached default profile photo emojis: %v", err)
	}
	if _, ok := cached.(*tg.EmojiListNotModified); !ok {
		t.Fatalf("cached default profile photo emojis = %T, want notModified", cached)
	}
}

func TestAccountGetDefaultProfilePhotoEmojisFallsBackToSystemAnimatedEmoji(t *testing.T) {
	files := &fakeFiles{sets: map[domain.StickerSetKind][]domain.StickerSet{
		domain.StickerSetKindSystem: {
			{SystemKey: "dice:🎲", DocumentIDs: []int64{3001}},
			{SystemKey: "animated_emoji", DocumentIDs: []int64{4001, 4002, 4001}},
			{SystemKey: "emoji_generic_animations", DocumentIDs: []int64{5001}},
		},
	}}
	r := New(Config{}, Deps{Files: files}, zaptest.NewLogger(t), clock.System)

	got, err := r.onAccountGetDefaultProfilePhotoEmojis(context.Background(), 0)
	if err != nil {
		t.Fatalf("get default profile photo emojis: %v", err)
	}
	list, ok := got.(*tg.EmojiList)
	if !ok {
		t.Fatalf("default profile photo emojis = %T, want *tg.EmojiList", got)
	}
	if len(list.DocumentID) != 2 || list.DocumentID[0] != 4001 || list.DocumentID[1] != 4002 {
		t.Fatalf("document ids = %v, want animated_emoji fallback only", list.DocumentID)
	}
}

func TestAccountGetDefaultBackgroundEmojisUsesStatusPack(t *testing.T) {
	files := &fakeFiles{sets: map[domain.StickerSetKind][]domain.StickerSet{
		domain.StickerSetKindEmoji: {
			{ShortName: "OtherPack", DocumentIDs: []int64{9001}},
			{ShortName: "StatusPack", DocumentIDs: []int64{1001, 0, 1002, 1001}},
		},
	}}
	r := New(Config{}, Deps{Files: files}, zaptest.NewLogger(t), clock.System)

	got, err := r.onAccountGetDefaultBackgroundEmojis(context.Background(), 0)
	if err != nil {
		t.Fatalf("get default background emojis: %v", err)
	}
	list, ok := got.(*tg.EmojiList)
	if !ok {
		t.Fatalf("default background emojis = %T, want *tg.EmojiList", got)
	}
	if len(list.DocumentID) != 2 || list.DocumentID[0] != 1001 || list.DocumentID[1] != 1002 {
		t.Fatalf("document ids = %v, want deduped StatusPack ids", list.DocumentID)
	}
	if list.Hash == 0 {
		t.Fatal("emoji list hash = 0, want stable non-zero hash")
	}

	cached, err := r.onAccountGetDefaultBackgroundEmojis(context.Background(), list.Hash)
	if err != nil {
		t.Fatalf("get cached default background emojis: %v", err)
	}
	if _, ok := cached.(*tg.EmojiListNotModified); !ok {
		t.Fatalf("cached default background emojis = %T, want notModified", cached)
	}
}

func TestAccountGetDefaultBackgroundEmojisFallsBackWhenStatusPackMissing(t *testing.T) {
	r := New(Config{}, Deps{Files: &fakeFiles{}}, zaptest.NewLogger(t), clock.System)

	got, err := r.onAccountGetDefaultBackgroundEmojis(context.Background(), 0)
	if err != nil {
		t.Fatalf("get default background emojis without StatusPack: %v", err)
	}
	list, ok := got.(*tg.EmojiList)
	if !ok {
		t.Fatalf("fallback default background emojis = %T, want *tg.EmojiList", got)
	}
	if list.Hash != 0 || len(list.DocumentID) != 0 {
		t.Fatalf("fallback list = hash %d ids %v, want empty compat stub", list.Hash, list.DocumentID)
	}
}
