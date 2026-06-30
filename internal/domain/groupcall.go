package domain

import "errors"

// 群通话（超级群语音聊天）领域模型。信令真值在 GroupCallStore（memory/postgres
// 双实现），版本协议要求 version 必须持久化：客户端忽略 version 小于本地缓存的
// updateGroupCallParticipants，重启回卷会让整个房间静默失联。

// GroupCallState 是群通话状态。
type GroupCallState string

const (
	GroupCallStateActive    GroupCallState = "active"
	GroupCallStateDiscarded GroupCallState = "discarded"
)

// 群通话业务错误；rpc 层映射为 GROUPCALL_* RPC_ERROR。
var (
	ErrGroupCallInvalid        = errors.New("group call invalid")
	ErrGroupCallDiscarded      = errors.New("group call already discarded")
	ErrGroupCallAlreadyStarted = errors.New("group call already started")
	ErrGroupCallSSRCDuplicate  = errors.New("group call ssrc duplicate")
	ErrGroupCallNotJoined      = errors.New("group call participant missing")
)

// GroupCall 是一场群通话的权威态。
type GroupCall struct {
	ID            int64
	AccessHash    int64
	ChannelID     int64
	CreatorUserID int64
	State         GroupCallState
	Title         string
	JoinMuted     bool
	// Version 是参与者协议版本：所有参与者变更事务内 +1，单调且持久。
	Version           int
	ParticipantsCount int
	CreatedAt         int
	DiscardedAt       int
	Duration          int
	// StartedMsgID 是 messageActionGroupCall(started) 的频道消息 id（discard 时
	// 客户端用它定位起始服务消息，当前仅记录）。
	StartedMsgID int
}

// Active 报告通话是否仍在进行。
func (c GroupCall) Active() bool {
	return c.State == GroupCallStateActive
}

// GroupCallParticipant 是房间内一名参与者。
type GroupCallParticipant struct {
	CallID int64
	UserID int64
	// SSRC 是客户端在 join JSON 里自报的 audio ssrc（uint32 值域，存 int64 防符号坑）。
	SSRC       int64
	JoinDate   int
	ActiveDate int
	Muted      bool
	// MutedByAdmin=true 时 can_self_unmute=false（管理员禁言/默认静音策略）。
	MutedByAdmin bool
	// VolumeByAdmin 是管理员设定的全局音量（1..20000），0=未设。
	VolumeByAdmin int
	// RaiseHandRating 非零表示举手中，值单调递增用于排序。
	RaiseHandRating int64
	// VideoJSON / PresentationJSON 是 tg.GroupCallParticipantVideo 的原始 JSON
	// 快照（M3/M4 启用；M0/M1 仅透明保存 self-edit，不转发）。
	VideoJSON        []byte
	PresentationJSON []byte
	Left             bool
	// LastCheckDate 是 checkGroupCall 保活水位。注意：客户端只在 Connecting 态
	// 发 checkGroupCall（媒体连通后心跳停止），掉线判定必须与 SFU 媒体面活性
	// 取双过期（见 sweeper），绝不能单凭此字段。
	LastCheckDate int
}

// CreateGroupCallRequest 创建群通话。
type CreateGroupCallRequest struct {
	ChannelID     int64
	CreatorUserID int64
	RandomID      int64
	Title         string
	Now           int
}

// JoinGroupCallRequest 加入/重进群通话（rejoin 同主键换新 ssrc）。
type JoinGroupCallRequest struct {
	CallID  int64
	UserID  int64
	SSRC    int64
	Muted   bool
	IsAdmin bool
	// VideoJSON 是本次 join 铸造的视频内部状态（endpoint+源组+active）；rejoin
	// 整体替换并**清空旧 PresentationJSON**（客户端主连接 rejoin 后会重发
	// joinGroupCallPresentation，旧屏幕登记必须作废）。
	VideoJSON []byte
	Now       int
}

// GroupCallMutation 是一次参与者维度变更的结果：变更后的 call 行（含新 version）
// 与受影响的参与者行（推送 updateGroupCallParticipants 用）。
type GroupCallMutation struct {
	Call        GroupCall
	Participant GroupCallParticipant
}

// GroupCallParticipantUpdate 是 editGroupCallParticipant 的字段级更新（nil=不动）。
type GroupCallParticipantUpdate struct {
	Muted            *bool
	MutedByAdmin     *bool
	VolumeByAdmin    *int
	RaiseHandRating  *int64
	VideoJSON        *[]byte
	PresentationJSON *[]byte
	Now              int
}

// GroupCallParticipantOverride 是 per-viewer 视图覆盖（setter→target）：
// 本地静音/本地音量，仅 setter 自己可见，不进全房间 version。
type GroupCallParticipantOverride struct {
	MutedByYou bool
	Volume     int // 0=未设
}

// GroupCallParticipantPage 是 getGroupParticipants 的分页结果。
type GroupCallParticipantPage struct {
	Count        int
	Participants []GroupCallParticipant
	NextOffset   string
	Version      int
}
