package store

import (
	"context"

	"telesrv/internal/domain"
)

// GroupCallStore 持久化群通话信令真值（memory/postgres 双实现，行为契约由
// 共享 contract test 钉死；version 单调性见 domain/groupcall.go 注释）。
type GroupCallStore interface {
	// CreateGroupCall 建会；同频道已有活跃通话返回 domain.ErrGroupCallAlreadyStarted。
	CreateGroupCall(ctx context.Context, call domain.GroupCall) (domain.GroupCall, error)
	GetGroupCall(ctx context.Context, callID int64) (domain.GroupCall, bool, error)
	// JoinGroupCall 加入/重进（同主键 upsert 换新 ssrc）；ssrc 与他人撞活跃唯一
	// 约束返回 domain.ErrGroupCallSSRCDuplicate；version++。
	JoinGroupCall(ctx context.Context, req domain.JoinGroupCallRequest) (domain.GroupCallMutation, error)
	// LeaveGroupCall 置 left+version++；未在会返回 domain.ErrGroupCallNotJoined。
	LeaveGroupCall(ctx context.Context, callID, userID int64, now int) (domain.GroupCallMutation, error)
	// DiscardGroupCall 终结通话并清空参与者，返回终态 call 与此前活跃的参与者。
	DiscardGroupCall(ctx context.Context, callID int64, now int) (domain.GroupCall, []domain.GroupCallParticipant, error)
	// TouchParticipant 刷新 checkGroupCall 保活水位，返回该用户当前活跃 ssrc 集合
	//（joined=false 表示未在会，客户端据空集自动 rejoin）。
	TouchParticipant(ctx context.Context, callID, userID int64, now int) (activeSSRCs []int64, joined bool, err error)
	GetParticipant(ctx context.Context, callID, userID int64) (domain.GroupCallParticipant, bool, error)
	// ListParticipants 按 (join_date, user_id) 游标分页；offset 为上次返回的
	// NextOffset（空=从头）。响应携带当前 version（客户端跳号全量 reload 依赖）。
	ListParticipants(ctx context.Context, callID int64, offset string, limit int) (domain.GroupCallParticipantPage, error)
	// UpdateParticipant 应用字段级更新；changed=false 表示无有效变化（version 不动）。
	UpdateParticipant(ctx context.Context, callID, userID int64, update domain.GroupCallParticipantUpdate) (domain.GroupCallMutation, bool, error)
	// SetGroupCallTitle / SetGroupCallJoinMuted 只动 call 行，不动参与者 version。
	SetGroupCallTitle(ctx context.Context, callID int64, title string) (domain.GroupCall, bool, error)
	SetGroupCallJoinMuted(ctx context.Context, callID int64, joinMuted bool) (domain.GroupCall, bool, error)
	SetStartedMessageID(ctx context.Context, callID int64, msgID int) error
	// SweepStaleParticipants 清理保活水位早于 checkOlderThan 的活跃参与者
	//（每清一人 version++）。注意调用方必须叠加 SFU 媒体面活性做双过期判定。
	SweepStaleParticipants(ctx context.Context, checkOlderThan, now int, limit int) ([]domain.GroupCallMutation, error)
	// ResetAllParticipants 服务端重启恢复：把全部活跃通话的参与者批量置 left
	//（每通话 version++），返回受影响的通话。
	ResetAllParticipants(ctx context.Context, now int) ([]domain.GroupCall, error)
	// NextRaiseHandRating 分配全局单调递增的举手序号（举手排序用）。
	NextRaiseHandRating(ctx context.Context, callID int64) (int64, error)
	// SetParticipantOverride 写入 setter 对 target 的 per-viewer 覆盖（本地静音/音量），
	// 仅影响 setter 自己的视图。clear=true 删除覆盖。
	SetParticipantOverride(ctx context.Context, callID, setterUserID, targetUserID int64, override domain.GroupCallParticipantOverride, clear bool) error
	// GetParticipantOverride 取某 setter 对某 target 的覆盖。
	GetParticipantOverride(ctx context.Context, callID, setterUserID, targetUserID int64) (domain.GroupCallParticipantOverride, bool, error)
}
