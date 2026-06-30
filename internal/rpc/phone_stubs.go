package rpc

import (
	"context"

	"github.com/gotd/td/tg"
)

// 群通话范围外入口的被动 stub（防点崩）。未列出的 phone.*（conference 族 /
// 通话内消息族 / scheduled / RTMP）走 router fallback：400/500 NOT_IMPLEMENTED +
// 兼容矩阵日志，客户端不断连。
func (r *Router) registerPhoneStubs(d *tg.ServerDispatcher) {
	// 入会面板前置调用：返回 self 一个候选身份（空返回会卡 UI）。
	d.OnPhoneGetGroupCallJoinAs(func(ctx context.Context, peer tg.InputPeerClass) (*tg.PhoneJoinAsPeers, error) {
		userID, err := r.phoneRequireUser(ctx)
		if err != nil {
			return nil, err
		}
		return &tg.PhoneJoinAsPeers{
			Peers: []tg.PeerClass{&tg.PeerUser{UserID: userID}},
			Chats: []tg.ChatClass{},
			Users: r.tgUsersForIDs(ctx, userID, []int64{userID}),
		}, nil
	})
	d.OnPhoneSaveDefaultGroupCallJoinAs(func(ctx context.Context, req *tg.PhoneSaveDefaultGroupCallJoinAsRequest) (bool, error) {
		return true, nil
	})
	// 录制范围外：客户端只看 record_start_date（恒不下发），打发掉即可。
	d.OnPhoneToggleGroupCallRecord(func(ctx context.Context, req *tg.PhoneToggleGroupCallRecordRequest) (tg.UpdatesClass, error) {
		return tgEmptyUpdates(int(r.clock.Now().Unix())), nil
	})
	// RTMP 直播范围外。
	d.OnPhoneGetGroupCallStreamChannels(func(ctx context.Context, call tg.InputGroupCallClass) (*tg.PhoneGroupCallStreamChannels, error) {
		return &tg.PhoneGroupCallStreamChannels{Channels: []tg.GroupCallStreamChannel{}}, nil
	})
}
