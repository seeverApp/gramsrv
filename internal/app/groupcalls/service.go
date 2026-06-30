// Package groupcalls 实现超级群语音聊天（group call）的信令业务层：
// ID/access_hash 分配与 store 编排。权限（admin/成员资格）由 rpc 层校验，
// version 单调性与并发串行化由 store 层事务保证。
package groupcalls

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"telesrv/internal/domain"
	"telesrv/internal/store"
)

// Service 是群通话业务服务。
type Service struct {
	store store.GroupCallStore
}

// NewService 创建群通话服务。
func NewService(st store.GroupCallStore) *Service {
	return &Service{store: st}
}

// Create 分配 id/access_hash 并建会。
func (s *Service) Create(ctx context.Context, channelID, creatorUserID int64, title string, now int) (domain.GroupCall, error) {
	id, err := randomPositiveInt64()
	if err != nil {
		return domain.GroupCall{}, err
	}
	accessHash, err := randomPositiveInt64()
	if err != nil {
		return domain.GroupCall{}, err
	}
	return s.store.CreateGroupCall(ctx, domain.GroupCall{
		ID:            id,
		AccessHash:    accessHash,
		ChannelID:     channelID,
		CreatorUserID: creatorUserID,
		Title:         title,
		Version:       1,
		CreatedAt:     now,
	})
}

func (s *Service) Get(ctx context.Context, callID int64) (domain.GroupCall, bool, error) {
	return s.store.GetGroupCall(ctx, callID)
}

func (s *Service) Join(ctx context.Context, req domain.JoinGroupCallRequest) (domain.GroupCallMutation, error) {
	return s.store.JoinGroupCall(ctx, req)
}

func (s *Service) Leave(ctx context.Context, callID, userID int64, now int) (domain.GroupCallMutation, error) {
	return s.store.LeaveGroupCall(ctx, callID, userID, now)
}

func (s *Service) Discard(ctx context.Context, callID int64, now int) (domain.GroupCall, []domain.GroupCallParticipant, error) {
	return s.store.DiscardGroupCall(ctx, callID, now)
}

func (s *Service) Touch(ctx context.Context, callID, userID int64, now int) ([]int64, bool, error) {
	return s.store.TouchParticipant(ctx, callID, userID, now)
}

func (s *Service) Participant(ctx context.Context, callID, userID int64) (domain.GroupCallParticipant, bool, error) {
	return s.store.GetParticipant(ctx, callID, userID)
}

func (s *Service) Participants(ctx context.Context, callID int64, offset string, limit int) (domain.GroupCallParticipantPage, error) {
	return s.store.ListParticipants(ctx, callID, offset, limit)
}

func (s *Service) UpdateParticipant(ctx context.Context, callID, userID int64, update domain.GroupCallParticipantUpdate) (domain.GroupCallMutation, bool, error) {
	return s.store.UpdateParticipant(ctx, callID, userID, update)
}

func (s *Service) SetTitle(ctx context.Context, callID int64, title string) (domain.GroupCall, bool, error) {
	return s.store.SetGroupCallTitle(ctx, callID, title)
}

func (s *Service) SetJoinMuted(ctx context.Context, callID int64, joinMuted bool) (domain.GroupCall, bool, error) {
	return s.store.SetGroupCallJoinMuted(ctx, callID, joinMuted)
}

func (s *Service) SetStartedMessageID(ctx context.Context, callID int64, msgID int) error {
	return s.store.SetStartedMessageID(ctx, callID, msgID)
}

func (s *Service) SweepStale(ctx context.Context, checkOlderThan, now, limit int) ([]domain.GroupCallMutation, error) {
	return s.store.SweepStaleParticipants(ctx, checkOlderThan, now, limit)
}

func (s *Service) ResetAllParticipants(ctx context.Context, now int) ([]domain.GroupCall, error) {
	return s.store.ResetAllParticipants(ctx, now)
}

func (s *Service) NextRaiseHandRating(ctx context.Context, callID int64) (int64, error) {
	return s.store.NextRaiseHandRating(ctx, callID)
}

func (s *Service) SetParticipantOverride(ctx context.Context, callID, setterUserID, targetUserID int64, override domain.GroupCallParticipantOverride, clear bool) error {
	return s.store.SetParticipantOverride(ctx, callID, setterUserID, targetUserID, override, clear)
}

func (s *Service) ParticipantOverride(ctx context.Context, callID, setterUserID, targetUserID int64) (domain.GroupCallParticipantOverride, bool, error) {
	return s.store.GetParticipantOverride(ctx, callID, setterUserID, targetUserID)
}

func randomPositiveInt64() (int64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, fmt.Errorf("groupcalls: random id: %w", err)
	}
	v := int64(binary.BigEndian.Uint64(buf[:]) >> 1)
	if v == 0 {
		v = 1
	}
	return v, nil
}
