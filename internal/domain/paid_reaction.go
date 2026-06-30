package domain

// 频道帖子付费 reaction（messages.sendPaidReaction）：用户花 Stars 为一条频道消息「点赞」，
// 星数在 (channel,message,user) 上累计；消息展示 ReactionPaid 总星数 + top reactors 排行。
// 与 Stars 账本（stars.go）配合：rpc 先 Debit 再在此累计。

const (
	// MaxPaidReactionStarsPerRequest 是单次 sendPaidReaction 的星数上限（对齐官方 stars_paid_reaction_amount_max）。
	MaxPaidReactionStarsPerRequest = 10000
	// MaxPaidReactionTopReactors 是 top reactors 排行展示条数。
	MaxPaidReactionTopReactors = 3
)

// PaidReactor 是某 reactor 对一条消息累计投入的付费 reaction 星数。
type PaidReactor struct {
	UserID    int64
	Stars     int64
	Anonymous bool
	My        bool // 是否为当前 viewer（投影时按视角置位）
}

// ChannelMessagePaidReactions 是一条频道消息的付费 reaction 聚合（携带在消息上 / reaction 更新里）。
type ChannelMessagePaidReactions struct {
	TotalStars  int64         // 全体 reactor 投入星数之和
	MyStars     int64         // 当前 viewer 投入的星数（0 = 未投）
	MyAnonymous bool          // 当前 viewer 是否匿名投入
	TopReactors []PaidReactor // 按 Stars DESC，含当前 viewer
}

// SendChannelPaidReactionRequest 为一条频道消息增投付费 reaction 星数。
type SendChannelPaidReactionRequest struct {
	UserID    int64
	ChannelID int64
	MessageID int
	Stars     int64
	Anonymous bool // 隐私：是否匿名投入
	Date      int
}

// ChannelMessagePaidReactionResult 是增投后的结果，供 rpc 投影与扇出。
type ChannelMessagePaidReactionResult struct {
	Channel    Channel
	Message    ChannelMessage
	Paid       ChannelMessagePaidReactions
	Recipients []int64
}
