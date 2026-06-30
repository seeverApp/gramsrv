import { ArrowLeft } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { Alert, Badge, EmptyRow, JsonBlock, LoadingSurface, PageFrame, SectionHead, Summary } from "../components/ui";
import { formatUnix } from "../lib/format";
import type { Navigate } from "../routing";
import type { GroupMessageDetail } from "../types";

export function GroupMessageDetailPage({ channelID, msgID, navigate }: { channelID: number; msgID: number; navigate: Navigate }) {
  const [detail, setDetail] = useState<GroupMessageDetail | null>(null);
  const [error, setError] = useState("");

  async function load() {
    setError("");
    try {
      setDetail(await api.groupMessage(channelID, msgID));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  useEffect(() => {
    void load();
  }, [channelID, msgID]);

  if (error) {
    return <Alert>{error}</Alert>;
  }
  if (!detail) {
    return <LoadingSurface label="加载群聊消息详情" />;
  }

  const msg = detail.Message;
  return (
    <PageFrame
      title={`群聊消息 #${msg.ID}`}
      eyebrow="消息详情"
      actions={<button className="btn icon-text" onClick={() => navigate("/messages/groups")}><ArrowLeft size={15} /> 返回群聊消息</button>}
    >
      <div className="stacked-sections">
        <section className="entity-head">
          <div>
            <div className="entity-title">频道/群 {msg.ChannelID}</div>
            <div className="entity-subtitle">发送方 {msg.SenderUserID} · {formatUnix(msg.Date)}</div>
          </div>
          <div className="entity-badges">
            {msg.Deleted ? <Badge tone="danger">已删除</Badge> : <Badge>存活</Badge>}
            {msg.Pinned && <Badge tone="warn">置顶</Badge>}
            {msg.Post && <Badge>频道帖子</Badge>}
            <Badge>pts {msg.PTS}</Badge>
          </div>
        </section>
        <div className="summary-grid">
          <Summary label="消息 ID" value={String(msg.ID)} mono />
          <Summary label="频道 / 群" value={String(msg.ChannelID)} mono />
          <Summary label="From Peer" value={`${msg.FromPeerType}:${msg.FromPeerID}`} mono />
          <Summary label="浏览" value={String(msg.ViewsCount)} />
        </div>
        <section className="section-block">
          <SectionHead title="消息行" text="channel_messages 只读快照" />
          <JsonBlock value={detail.MessageJSON} />
        </section>
        <section className="section-block">
          <SectionHead title="频道行" text="channels 只读快照" />
          <JsonBlock value={detail.ChannelJSON} />
        </section>
        <section className="section-block">
          <SectionHead title="频道更新事件" text="durable channel_update_events" />
          <div className="table-wrap">
            <table className="data-table">
              <thead><tr><th>PTS</th><th>数量</th><th>类型</th><th>消息 ID</th><th>发送方</th><th>时间</th></tr></thead>
              <tbody>
                {detail.UpdateEvents.map((row) => (
                  <tr key={`${row.PTS}-${row.Type}-${row.MessageID}`}>
                    <td>{row.PTS}</td>
                    <td>{row.PTSCount}</td>
                    <td>{row.Type}</td>
                    <td>{row.MessageID}</td>
                    <td>{row.SenderUserID}</td>
                    <td>{formatUnix(row.Date)}</td>
                  </tr>
                ))}
                {detail.UpdateEvents.length === 0 && <EmptyRow colSpan={6} />}
              </tbody>
            </table>
          </div>
        </section>
        <section className="section-block">
          <SectionHead title="事件 JSON" />
          <div className="raw-grid">
            {detail.UpdateEvents.map((row) => (
              <JsonBlock key={`${row.PTS}-${row.Type}-json`} value={row.JSON} />
            ))}
            {detail.UpdateEvents.length === 0 && <div className="empty-panel">无结果</div>}
          </div>
        </section>
      </div>
    </PageFrame>
  );
}
