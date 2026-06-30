import { ChevronRight, Search } from "lucide-react";
import { useState } from "react";
import { api, errorMessage } from "../api";
import { ChannelPicker } from "../components/EntityPicker";
import { Alert, Badge, EmptyRow, Metric, PageFrame, QueryPanel } from "../components/ui";
import { channelKind, formatUnix } from "../lib/format";
import type { Navigate } from "../routing";
import type { ChannelRow, GroupMessageListResponse } from "../types";

export function GroupMessagesPage({ navigate }: { navigate: Navigate }) {
  const [channel, setChannel] = useState<ChannelRow | null>(null);
  const [beforeDate, setBeforeDate] = useState("");
  const [beforeID, setBeforeID] = useState("");
  const [limit, setLimit] = useState("100");
  const [data, setData] = useState<GroupMessageListResponse | null>(null);
  const [error, setError] = useState("");

  async function load(next = false) {
    setError("");
    if (!channel) {
      setError("请先搜索并选择超级群或频道");
      return;
    }
    const params = new URLSearchParams({
      channel_id: String(channel.ID),
      limit
    });
    if (next && data?.rows.length) {
      const last = data.rows[data.rows.length - 1];
      params.set("before_date", String(last.Date));
      params.set("before_id", String(last.ID));
      setBeforeDate(String(last.Date));
      setBeforeID(String(last.ID));
    } else {
      if (beforeDate) params.set("before_date", beforeDate);
      if (beforeID) params.set("before_id", beforeID);
    }
    try {
      setData(await api.groupMessages(params));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  function changeChannel(row: ChannelRow | null) {
    setChannel(row);
    setBeforeDate("");
    setBeforeID("");
    setData(null);
  }

  const rows = data?.rows ?? [];

  return (
    <PageFrame title="群聊消息" eyebrow="超级群 / 频道消息">
      {error && <Alert>{error}</Alert>}
      <QueryPanel>
        <div className="message-selector-grid single">
          <ChannelPicker label="超级群 / 频道" value={channel} onChange={changeChannel} />
        </div>
        <form className="toolbar message-query" onSubmit={(event) => { event.preventDefault(); void load(false); }}>
          <input value={beforeDate} onChange={(event) => setBeforeDate(event.target.value)} placeholder="before_date 游标" />
          <input value={beforeID} onChange={(event) => setBeforeID(event.target.value)} placeholder="before_msg_id 游标" />
          <input className="small-input" value={limit} onChange={(event) => setLimit(event.target.value)} placeholder="条数 <= 100" />
          <button className="btn primary icon-text" type="submit"><Search size={15} /> 查询消息</button>
          {rows.length ? <button className="btn icon-text" type="button" onClick={() => load(true)}><ChevronRight size={15} /> 下一页</button> : null}
        </form>
      </QueryPanel>
      <div className="metric-row">
        <Metric label="当前页消息" value={String(rows.length)} />
        <Metric label="有媒体" value={String(rows.filter((row) => row.Media && row.Media !== "{}").length)} />
        <Metric label="频道帖子" value={String(rows.filter((row) => row.Post).length)} />
        <Metric label="频道 / 群" value={channel ? `${channel.Title || channelKind(channel)} (${channel.ID})` : "-"} />
      </div>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>消息 ID</th>
              <th>时间</th>
              <th>发送方</th>
              <th>From Peer</th>
              <th>PTS</th>
              <th>浏览</th>
              <th>状态</th>
              <th>正文</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={`${row.ChannelID}-${row.ID}`}>
                <td className="mono">{row.ID}</td>
                <td>{formatUnix(row.Date)}</td>
                <td className="mono">{row.SenderUserID}</td>
                <td className="mono">{row.FromPeerType}:{row.FromPeerID}</td>
                <td>{row.PTS}</td>
                <td>{row.ViewsCount}</td>
                <td>
                  {row.Deleted ? <Badge tone="danger">已删除</Badge> : row.Pinned ? <Badge tone="warn">置顶</Badge> : <Badge>存活</Badge>}
                </td>
                <td className="truncate">{row.Body}</td>
                <td>
                  <button
                    className="row-link"
                    onClick={() => navigate(`/messages/groups/detail?channel_id=${row.ChannelID}&msg_id=${row.ID}`)}
                  >
                    详情 <ChevronRight size={14} />
                  </button>
                </td>
              </tr>
            ))}
            {rows.length === 0 && <EmptyRow colSpan={9} />}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
