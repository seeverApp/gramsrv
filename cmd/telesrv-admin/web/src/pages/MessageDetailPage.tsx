import { ArrowLeft, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { Alert, Badge, EmptyRow, JsonBlock, LoadingSurface, PageFrame, SectionHead, SplitLayout, Summary } from "../components/ui";
import { formatDate, formatUnix } from "../lib/format";
import type { Navigate } from "../routing";
import type { MessageDetail } from "../types";

export function MessageDetailPage({ ownerUserID, msgID, navigate }: { ownerUserID: number; msgID: number; navigate: Navigate }) {
  const [detail, setDetail] = useState<MessageDetail | null>(null);
  const [error, setError] = useState("");

  async function load() {
    setError("");
    try {
      setDetail(await api.message(ownerUserID, msgID));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  useEffect(() => {
    void load();
  }, [ownerUserID, msgID]);

  if (error) {
    return <Alert>{error}</Alert>;
  }
  if (!detail) {
    return <LoadingSurface label="加载消息详情" />;
  }

  const msg = detail.Message;
  return (
    <PageFrame
      title={`消息 #${msg.BoxID}`}
      eyebrow="消息详情"
      actions={<button className="btn icon-text" onClick={() => navigate("/messages/private")}><ArrowLeft size={15} /> 返回私聊消息</button>}
    >
      <SplitLayout
        main={
          <div className="stacked-sections">
            <section className="entity-head">
              <div>
                <div className="entity-title">所属 {msg.OwnerUserID} · 对端 {msg.PeerID}</div>
                <div className="entity-subtitle">发送方 {msg.FromUserID} · {formatUnix(msg.Date)}</div>
              </div>
              <div className="entity-badges">
                {msg.Deleted ? <Badge tone="danger">已删除</Badge> : <Badge>存活</Badge>}
                <Badge>pts {msg.PTS}</Badge>
                <Badge>{msg.Outgoing ? "发出" : "收到"}</Badge>
              </div>
            </section>
            <div className="summary-grid">
              <Summary label="消息盒 ID" value={String(msg.BoxID)} mono />
              <Summary label="私聊消息 ID" value={String(msg.PrivateMessageID)} mono />
              <Summary label="发送方" value={String(msg.MessageSenderID)} mono />
              <Summary label="时间" value={formatUnix(msg.Date)} />
            </div>
            <section className="section-block">
              <SectionHead title="消息盒" text="message_boxes 只读快照" />
              <JsonBlock value={detail.MessageJSON} />
            </section>
            <div className="raw-grid">
              <section className="section-block">
                <SectionHead title="会话行" text="dialogs 只读快照" />
                <JsonBlock value={detail.DialogJSON} />
              </section>
              <section className="section-block">
                <SectionHead title="私聊消息行" text="private_messages 只读快照" />
                <JsonBlock value={detail.PrivateJSON} />
              </section>
            </div>
            <section className="section-block">
              <SectionHead title="更新事件" text="durable user_update_events" />
              <div className="table-wrap">
                <table className="data-table">
                  <thead><tr><th>PTS</th><th>数量</th><th>类型</th><th>时间</th></tr></thead>
                  <tbody>
                    {detail.UpdateEvents.map((row) => <tr key={`${row.PTS}-${row.Type}`}><td>{row.PTS}</td><td>{row.PTSCount}</td><td>{row.Type}</td><td>{formatUnix(row.Date)}</td></tr>)}
                    {detail.UpdateEvents.length === 0 && <EmptyRow colSpan={4} />}
                  </tbody>
                </table>
              </div>
            </section>
            <section className="section-block">
              <SectionHead title="分发队列" text="在线/离线 dispatch_outbox" />
              <div className="table-wrap">
                <table className="data-table">
                  <thead><tr><th>ID</th><th>用户</th><th>PTS</th><th>类型</th><th>状态</th><th>尝试</th><th>更新时间</th></tr></thead>
                  <tbody>
                    {detail.Outbox.map((row) => <tr key={row.ID}><td>{row.ID}</td><td>{row.TargetUserID}</td><td>{row.PTS}</td><td>{row.EventType}</td><td>{row.Status}</td><td>{row.Attempts}</td><td>{formatDate(row.UpdatedAt)}</td></tr>)}
                    {detail.Outbox.length === 0 && <EmptyRow colSpan={7} />}
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        }
        side={
          <section className="action-dock">
            <div className="dock-title">消息操作</div>
            <ActionButton
              label="删除此消息"
              icon={<Trash2 size={15} />}
              path="/api/actions/delete-messages"
              payload={() => ({ owner_user_id: msg.OwnerUserID, peer_id: msg.PeerID, ids: [msg.BoxID], revoke: true })}
              onDone={load}
            />
          </section>
        }
      />
    </PageFrame>
  );
}
