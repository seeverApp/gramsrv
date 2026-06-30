package postgres

import (
	"context"
	"errors"
	"testing"

	"telesrv/internal/domain"
)

// TestChannelPaidReactionPostgres 回归迁移 0010：广播频道付费 reaction 对真实 PG 的累计 +
// 聚合（总星数 / viewer 自身 / top reactors 降序 / 同 reactor 多次累加）。
func TestChannelPaidReactionPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	suffix := randomSuffix(t)

	users := NewUserStore(pool)
	owner, err := users.Create(ctx, domain.User{AccessHash: 521, Phone: "+1778" + suffix + "41", FirstName: "PaidRxOwner"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	member, err := users.Create(ctx, domain.User{AccessHash: 522, Phone: "+1778" + suffix + "42", FirstName: "PaidRxMember"})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	var channelID int64
	t.Cleanup(func() {
		if channelID != 0 {
			_, _ = pool.Exec(ctx, "DELETE FROM channel_message_paid_reactions WHERE channel_id = $1", channelID)
			_, _ = pool.Exec(ctx, "DELETE FROM channels WHERE id = $1", channelID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = ANY($1::bigint[])", []int64{owner.ID, member.ID})
	})

	channels := NewChannelStore(pool)
	created, err := channels.CreateChannel(ctx, domain.CreateChannelRequest{
		CreatorUserID: owner.ID,
		Title:         "Paid Reaction " + suffix,
		Broadcast:     true,
		MemberUserIDs: []int64{member.ID},
		Date:          1700000400,
	})
	if err != nil {
		t.Fatalf("create broadcast channel: %v", err)
	}
	channelID = created.Channel.ID
	sent, err := channels.SendChannelMessage(ctx, domain.SendChannelMessageRequest{
		UserID:    owner.ID,
		ChannelID: channelID,
		RandomID:  9401,
		Message:   "paid reaction target",
		Date:      1700000401,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	msgID := sent.Message.ID

	// owner 投 100。
	res, err := channels.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: owner.ID, ChannelID: channelID, MessageID: msgID, Stars: 100, Date: 1700000402,
	})
	if err != nil {
		t.Fatalf("owner paid reaction: %v", err)
	}
	if res.Paid.TotalStars != 100 || res.Paid.MyStars != 100 {
		t.Fatalf("after owner 100 = total %d my %d, want 100/100", res.Paid.TotalStars, res.Paid.MyStars)
	}

	// member 投 250 → 总 350，member 视角 my=250，top reactors 降序 member(250)/owner(100)。
	res, err = channels.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: member.ID, ChannelID: channelID, MessageID: msgID, Stars: 250, Date: 1700000403,
	})
	if err != nil {
		t.Fatalf("member paid reaction: %v", err)
	}
	if res.Paid.TotalStars != 350 || res.Paid.MyStars != 250 {
		t.Fatalf("after member 250 = total %d my %d, want 350/250", res.Paid.TotalStars, res.Paid.MyStars)
	}
	if len(res.Paid.TopReactors) != 2 || res.Paid.TopReactors[0].Stars != 250 || !res.Paid.TopReactors[0].My || res.Paid.TopReactors[1].Stars != 100 {
		t.Fatalf("top reactors = %+v, want member(250,My)/owner(100)", res.Paid.TopReactors)
	}

	// owner 再投 50 → 累加到 150，总 400。
	res, err = channels.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: owner.ID, ChannelID: channelID, MessageID: msgID, Stars: 50, Date: 1700000404,
	})
	if err != nil {
		t.Fatalf("owner re-invest: %v", err)
	}
	if res.Paid.TotalStars != 400 || res.Paid.MyStars != 150 {
		t.Fatalf("after owner +50 = total %d my %d, want 400/150 (accumulated)", res.Paid.TotalStars, res.Paid.MyStars)
	}

	// 关键回归（读路径修复）：fresh 读回该消息必须携带付费 reaction（不止实时推送）。
	// owner 视角：Paid.TotalStars=400, MyStars=150。
	read, err := channels.GetChannelMessageReactions(ctx, domain.ChannelMessageReactionsRequest{
		UserID: owner.ID, ChannelID: channelID, IDs: []int{msgID},
	})
	if err != nil {
		t.Fatalf("get message reactions: %v", err)
	}
	if len(read.Messages) != 1 || read.Messages[0].Reactions == nil || read.Messages[0].Reactions.Paid == nil {
		t.Fatalf("fresh read messages=%d reactions/paid missing: %+v", len(read.Messages), read.Messages)
	}
	paid := read.Messages[0].Reactions.Paid
	if paid.TotalStars != 400 || paid.MyStars != 150 {
		t.Fatalf("fresh read paid = total %d my %d, want 400/150 (读路径须回显付费 reaction)", paid.TotalStars, paid.MyStars)
	}
	// member 视角读同一条：MyStars=250、TopReactors 含 member 自己带 My。
	readMember, err := channels.GetChannelMessageReactions(ctx, domain.ChannelMessageReactionsRequest{
		UserID: member.ID, ChannelID: channelID, IDs: []int{msgID},
	})
	if err != nil {
		t.Fatalf("get message reactions (member): %v", err)
	}
	mp := readMember.Messages[0].Reactions.Paid
	if mp == nil || mp.TotalStars != 400 || mp.MyStars != 250 {
		t.Fatalf("member fresh read paid = %+v, want total 400 my 250", mp)
	}

	// 非法星数被拒。
	if _, err := channels.AddChannelMessagePaidReaction(ctx, domain.SendChannelPaidReactionRequest{
		UserID: owner.ID, ChannelID: channelID, MessageID: msgID, Stars: 0, Date: 1700000405,
	}); !errors.Is(err, domain.ErrChannelInvalid) {
		t.Fatalf("zero-stars err = %v, want ErrChannelInvalid", err)
	}
}
