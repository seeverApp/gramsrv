package memory

import (
	"context"
	"sort"
	"time"

	"telesrv/internal/domain"
)

// PinPrivateMessage 与 PG 实现同语义：非 pm_oneside 的 pin 双侧置位，
// unpin 恒双侧清除，状态未变化时幂等 no-op；服务消息不可置顶。
func (s *MessageStore) PinPrivateMessage(_ context.Context, req domain.PinPrivateMessageRequest) (domain.PinPrivateMessageResult, error) {
	res := domain.PinPrivateMessageResult{OwnerUserID: req.OwnerUserID}
	if req.OwnerUserID == 0 || req.Peer.Type != domain.PeerTypeUser || req.Peer.ID == 0 {
		return res, domain.ErrMessageIDInvalid
	}
	if req.MessageID <= 0 || req.MessageID > domain.MaxMessageBoxID {
		return res, domain.ErrMessageIDInvalid
	}
	if req.Date == 0 {
		req.Date = int(time.Now().Unix())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ownIdx := -1
	for i, msg := range s.m[req.OwnerUserID] {
		if msg.Peer == req.Peer && msg.ID == req.MessageID {
			ownIdx = i
			break
		}
	}
	if ownIdx < 0 {
		return res, domain.ErrMessageIDInvalid
	}
	owned := s.m[req.OwnerUserID][ownIdx]
	if owned.Media != nil && owned.Media.Kind == domain.MessageMediaKindService {
		return res, domain.ErrMessageIDInvalid
	}
	if owned.Pinned == req.Pinned && (!req.Pinned || req.PmOneside) {
		// 与 PG 同语义：unpin/oneside pin 幂等短路；共享 pin 仍需检查
		// 对端传播。
		return res, nil
	}
	type pinSide struct {
		userID int64
		peer   domain.Peer
		idx    int
	}
	sides := []pinSide{{userID: req.OwnerUserID, peer: req.Peer, idx: ownIdx}}
	if req.Peer.ID != req.OwnerUserID && (!req.Pinned || !req.PmOneside) {
		for i, msg := range s.m[req.Peer.ID] {
			if msg.UID == owned.UID {
				sides = append(sides, pinSide{
					userID: req.Peer.ID,
					peer:   domain.Peer{Type: domain.PeerTypeUser, ID: req.OwnerUserID},
					idx:    i,
				})
				break
			}
		}
	}
	for _, side := range sides {
		if s.m[side.userID][side.idx].Pinned == req.Pinned {
			continue
		}
		s.m[side.userID][side.idx].Pinned = req.Pinned
		boxID := s.m[side.userID][side.idx].ID
		res.Updated = append(res.Updated, domain.PinnedMessagesForUser{
			UserID:     side.userID,
			Peer:       side.peer,
			MessageIDs: []int{boxID},
			Pinned:     req.Pinned,
			Event: domain.UpdateEvent{
				UserID:     side.userID,
				Type:       domain.UpdateEventPinnedMessages,
				Pts:        s.nextPtsLocked(side.userID),
				PtsCount:   1,
				Date:       req.Date,
				Peer:       side.peer,
				Bool:       req.Pinned,
				MessageIDs: []int{boxID},
			},
		})
	}
	return res, nil
}

// UnpinAllPrivateMessages 与 PG 实现同语义：本侧整批清除并经 UID 同步
// 清除对端共享置顶。
func (s *MessageStore) UnpinAllPrivateMessages(_ context.Context, req domain.UnpinAllPrivateMessagesRequest) (domain.PinPrivateMessageResult, error) {
	res := domain.PinPrivateMessageResult{OwnerUserID: req.OwnerUserID}
	if req.OwnerUserID == 0 || req.Peer.Type != domain.PeerTypeUser || req.Peer.ID == 0 {
		return res, domain.ErrMessageIDInvalid
	}
	if req.Date == 0 {
		req.Date = int(time.Now().Unix())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 与 PG 同语义：按 box_id 降序取一批清除，剩余批次以 Offset=1 续清。
	pinnedIdx := make([]int, 0)
	for i, msg := range s.m[req.OwnerUserID] {
		if msg.Peer == req.Peer && msg.Pinned {
			pinnedIdx = append(pinnedIdx, i)
		}
	}
	sort.Slice(pinnedIdx, func(a, b int) bool {
		return s.m[req.OwnerUserID][pinnedIdx[a]].ID > s.m[req.OwnerUserID][pinnedIdx[b]].ID
	})
	if len(pinnedIdx) > domain.MaxUnpinAllBatch {
		pinnedIdx = pinnedIdx[:domain.MaxUnpinAllBatch]
		res.Offset = 1
	}
	ownIDs := make([]int, 0, len(pinnedIdx))
	uids := make(map[int64]struct{})
	for _, i := range pinnedIdx {
		s.m[req.OwnerUserID][i].Pinned = false
		ownIDs = append(ownIDs, s.m[req.OwnerUserID][i].ID)
		uids[s.m[req.OwnerUserID][i].UID] = struct{}{}
	}
	if len(ownIDs) == 0 {
		return res, nil
	}
	appendSide := func(userID int64, peer domain.Peer, ids []int) {
		res.Updated = append(res.Updated, domain.PinnedMessagesForUser{
			UserID:     userID,
			Peer:       peer,
			MessageIDs: ids,
			Pinned:     false,
			Event: domain.UpdateEvent{
				UserID:     userID,
				Type:       domain.UpdateEventPinnedMessages,
				Pts:        s.nextPtsLocked(userID),
				PtsCount:   1,
				Date:       req.Date,
				Peer:       peer,
				Bool:       false,
				MessageIDs: ids,
			},
		})
	}
	appendSide(req.OwnerUserID, req.Peer, ownIDs)
	if req.Peer.ID != req.OwnerUserID {
		peerIDs := make([]int, 0)
		for i, msg := range s.m[req.Peer.ID] {
			if _, ok := uids[msg.UID]; ok && msg.Pinned {
				s.m[req.Peer.ID][i].Pinned = false
				peerIDs = append(peerIDs, msg.ID)
			}
		}
		if len(peerIDs) > 0 {
			appendSide(req.Peer.ID, domain.Peer{Type: domain.PeerTypeUser, ID: req.OwnerUserID}, peerIDs)
		}
	}
	return res, nil
}
