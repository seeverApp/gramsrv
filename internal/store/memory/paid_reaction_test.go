package memory

import (
	"context"
	"errors"
	"testing"

	"telesrv/internal/domain"
)

func seedBroadcastPost(t *testing.T, st *ChannelStore, creator int64, broadcast bool) (channelID int64, msgID int) {
	t.Helper()
	ctx := context.Background()
	created, err := st.CreateChannel(ctx, domain.CreateChannelRequest{
		CreatorUserID: creator,
		Title:         "Paid",
		Broadcast:     broadcast,
		Megagroup:     !broadcast,
		Date:          1700000000,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	sent, err := st.SendChannelMessage(ctx, domain.SendChannelMessageRequest{
		UserID:    creator,
		ChannelID: created.Channel.ID,
		Message:   "post",
		Date:      1700000000,
	})
	if err != nil {
		t.Fatalf("send channel message: %v", err)
	}
	return created.Channel.ID, sent.Message.ID
}

// 付费 reaction 累计 + 聚合：同一 reactor 多次增投累加，TopReactors 含本人带 My。
func TestAddChannelMessagePaidReactionAccumulates(t *testing.T) {
	st := NewChannelStore()
	ctx := context.Background()
	const creator = int64(1000000001)
	channelID, msgID := seedBroadcastPost(t, st, creator, true)

	res, err := st.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: creator, ChannelID: channelID, MessageID: msgID, Stars: 100, Date: 1700000001,
	})
	if err != nil {
		t.Fatalf("first paid reaction: %v", err)
	}
	if res.Paid.TotalStars != 100 || res.Paid.MyStars != 100 {
		t.Fatalf("after 100 = total %d my %d, want 100/100", res.Paid.TotalStars, res.Paid.MyStars)
	}

	res, err = st.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: creator, ChannelID: channelID, MessageID: msgID, Stars: 50, Date: 1700000002,
	})
	if err != nil {
		t.Fatalf("second paid reaction: %v", err)
	}
	if res.Paid.TotalStars != 150 || res.Paid.MyStars != 150 {
		t.Fatalf("after +50 = total %d my %d, want 150/150 (accumulated)", res.Paid.TotalStars, res.Paid.MyStars)
	}
	if len(res.Paid.TopReactors) != 1 || res.Paid.TopReactors[0].Stars != 150 || !res.Paid.TopReactors[0].My {
		t.Fatalf("top reactors = %+v, want one My 150", res.Paid.TopReactors)
	}
}

// 多 reactor：TopReactors 按星数降序，本人始终在列。
func TestAddChannelMessagePaidReactionTopReactors(t *testing.T) {
	st := NewChannelStore()
	ctx := context.Background()
	const creator = int64(1000000001)
	channelID, msgID := seedBroadcastPost(t, st, creator, true)
	// 让另外两个用户成为成员并增投（直接写 store 累计，绕过成员校验仅测聚合）。
	for _, c := range []struct {
		user  int64
		stars int64
	}{{creator, 30}, {2000000002, 200}, {2000000003, 80}} {
		// 仅 creator 经正式路径；其他用户直接累计以构造排行。
		if c.user == creator {
			if _, err := st.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
				UserID: c.user, ChannelID: channelID, MessageID: msgID, Stars: c.stars, Date: 1700000010,
			}); err != nil {
				t.Fatalf("creator paid reaction: %v", err)
			}
			continue
		}
		st.mu.Lock()
		st.paidReactions[channelID][msgID][c.user] = memoryPaidReaction{stars: c.stars, date: 1700000010}
		st.mu.Unlock()
	}
	res, err := st.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: creator, ChannelID: channelID, MessageID: msgID, Stars: 0 + 1, Date: 1700000011,
	})
	// creator 现在 31+? 重新算：creator 30 + 这次 1 = 31。
	if err != nil {
		t.Fatalf("paid reaction: %v", err)
	}
	if res.Paid.TotalStars != 31+200+80 {
		t.Fatalf("total = %d, want %d", res.Paid.TotalStars, 31+200+80)
	}
	// 降序：200, 80, 31。
	if len(res.Paid.TopReactors) != 3 || res.Paid.TopReactors[0].Stars != 200 || res.Paid.TopReactors[1].Stars != 80 || res.Paid.TopReactors[2].Stars != 31 {
		t.Fatalf("top reactors = %+v, want 200/80/31 desc", res.Paid.TopReactors)
	}
	if !res.Paid.TopReactors[2].My {
		t.Fatalf("creator (31) must carry My flag, got %+v", res.Paid.TopReactors[2])
	}
}

// 非广播频道拒绝付费 reaction。
func TestAddChannelMessagePaidReactionRejectsMegagroup(t *testing.T) {
	st := NewChannelStore()
	ctx := context.Background()
	const creator = int64(1000000001)
	channelID, msgID := seedBroadcastPost(t, st, creator, false)
	_, err := st.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: creator, ChannelID: channelID, MessageID: msgID, Stars: 10, Date: 1700000001,
	})
	if !errors.Is(err, domain.ErrReactionInvalid) {
		t.Fatalf("megagroup paid reaction err = %v, want ErrReactionInvalid", err)
	}
}
