package rpc

import "context"

// withAndroidCompatMetadata 为「客户端构造器漂移」请求**仅在当前 ctx 内**兜底 client 层/类型。
// 这类请求多来自未完整走 initConnection 的 DrKLO Android；当 layer/ClientType 仍未知时
// 按 android 处理，使下游（createChat 的 legacy 响应、langpack 的 lang_pack 派生）行为正确。
//
// 关键：**绝不把这个兜底值写回持久缓存**（不调 rememberClientLayer/rememberClientInfo）。
// 缓存是 invokeWithLayer/initConnection 的权威产物；条目被驱逐时该兜底会拿不到真实值而误判
// 227/android，若写回就把长连接老客户端的真实 layer/类型永久覆盖（与 NegotiatedLayer 的
// 「驱逐时不覆盖」契约矛盾）。出站 layer 由 Conn.clientLayer 承载（非覆盖），与本兜底无关。
func (r *Router) withAndroidCompatMetadata(ctx context.Context) context.Context {
	if LayerFrom(ctx) == 0 {
		ctx = WithLayer(ctx, currentClientLayer)
	}
	if ClientTypeFrom(ctx) == ClientTypeUnknown {
		ctx = WithClientInfo(ctx, ClientInfo{LangPack: string(ClientTypeAndroid), Type: ClientTypeAndroid})
	}
	return ctx
}
