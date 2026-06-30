package domain

import (
	"testing"
	"time"
)

func TestPremiumActiveAt(t *testing.T) {
	now := time.Now().Unix()
	cases := []struct {
		name string
		user User
		want bool
	}{
		{"zero until = 非会员", User{}, false},
		{"future until = 会员", User{PremiumUntil: int(now + 3600)}, true},
		{"past until = 已到期", User{PremiumUntil: int(now - 1)}, false},
		{"恰好 now = 已到期（边界闭区间外）", User{PremiumUntil: int(now)}, false},
		{"bot 永不会员（双保险）", User{Bot: true, BotInfoVersion: 1, PremiumUntil: int(now + 3600)}, false},
	}
	for _, tc := range cases {
		if got := tc.user.PremiumActiveAt(now); got != tc.want {
			t.Errorf("%s: PremiumActiveAt = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEmojiStatusActiveAt(t *testing.T) {
	now := time.Now().Unix()
	premium := int(now + 3600)
	cases := []struct {
		name string
		user User
		want bool
	}{
		{"未设置", User{PremiumUntil: premium}, false},
		{"永久状态", User{PremiumUntil: premium, EmojiStatusDocumentID: 7}, true},
		{"未过期状态", User{PremiumUntil: premium, EmojiStatusDocumentID: 7, EmojiStatusUntil: int(now + 60)}, true},
		{"已过期状态", User{PremiumUntil: premium, EmojiStatusDocumentID: 7, EmojiStatusUntil: int(now - 60)}, false},
		{"会员到期后残值不下发", User{PremiumUntil: int(now - 1), EmojiStatusDocumentID: 7}, false},
	}
	for _, tc := range cases {
		if got := tc.user.EmojiStatusActiveAt(now); got != tc.want {
			t.Errorf("%s: EmojiStatusActiveAt = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTrimMessageReactionsToUserMaxTiers(t *testing.T) {
	reactions := []MessageReaction{
		{Type: MessageReactionEmoji, Emoticon: "👍"},
		{Type: MessageReactionEmoji, Emoticon: "❤"},
		{Type: MessageReactionEmoji, Emoticon: "🔥"},
		{Type: MessageReactionEmoji, Emoticon: "🎉"},
	}
	// 默认档（含 0 = 旧调用方未填）裁到 1，保尾部最新。
	for _, max := range []int{0, -1, MaxMessageReactionsPerUser} {
		got := TrimMessageReactionsToUserMax(reactions, max)
		if len(got) != 1 || got[0].Emoticon != "🎉" {
			t.Fatalf("default tier (max=%d) = %+v, want [🎉]", max, got)
		}
	}
	// premium 档裁到 3，保尾部最新。
	got := TrimMessageReactionsToUserMax(reactions, MessageReactionsUserMax(true))
	if len(got) != 3 || got[0].Emoticon != "❤" || got[2].Emoticon != "🎉" {
		t.Fatalf("premium tier = %+v, want [❤ 🔥 🎉]", got)
	}
	// 超出 premium 档的请求值被封顶。
	if got := TrimMessageReactionsToUserMax(reactions, 99); len(got) != MaxMessageReactionsPerUserPremium {
		t.Fatalf("over-cap tier = %d entries, want %d", len(got), MaxMessageReactionsPerUserPremium)
	}
	if MessageReactionsUserMax(false) != MaxMessageReactionsPerUser {
		t.Fatalf("MessageReactionsUserMax(false) = %d", MessageReactionsUserMax(false))
	}
}

func TestPinnedDialogsLimitTiers(t *testing.T) {
	cases := []struct {
		folderID int
		premium  bool
		want     int
	}{
		{DialogMainFolderID, false, MaxPinnedDialogsMainFolder},
		{DialogMainFolderID, true, MaxPinnedDialogsMainFolderPremium},
		{DialogArchiveFolderID, false, MaxPinnedDialogsArchiveFolder},
		{DialogArchiveFolderID, true, MaxPinnedDialogsArchiveFolderPremium},
	}
	for _, tc := range cases {
		if got := PinnedDialogsLimit(tc.folderID, tc.premium); got != tc.want {
			t.Errorf("PinnedDialogsLimit(%d, %v) = %d, want %d", tc.folderID, tc.premium, got, tc.want)
		}
	}
}
