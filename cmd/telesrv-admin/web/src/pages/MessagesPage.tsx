import { ChevronRight, History, Search, Trash2 } from "lucide-react";
import { useState } from "react";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { UserPicker } from "../components/EntityPicker";
import { Alert, Badge, EmptyRow, Metric, PageFrame, QueryPanel } from "../components/ui";
import { displayName, formatUnix, parseIDs, toInt } from "../lib/format";
import type { Navigate } from "../routing";
import type { AccountRow, MessageListResponse } from "../types";

export function MessagesPage({ navigate }: { navigate: Navigate }) {
  const [owner, setOwner] = useState<AccountRow | null>(null);
  const [peer, setPeer] = useState<AccountRow | null>(null);
  const [beforeDate, setBeforeDate] = useState("");
  const [beforeID, setBeforeID] = useState("");
  const [limit, setLimit] = useState("100");
  const [ids, setIDs] = useState("");
  const [revoke, setRevoke] = useState(true);
  const [justClear, setJustClear] = useState(false);
  const [maxID, setMaxID] = useState("");
  const [maxBatches, setMaxBatches] = useState("1");
  const [data, setData] = useState<MessageListResponse | null>(null);
  const [error, setError] = useState("");

  async function load(next = false) {
    setError("");
    if (!owner || !peer) {
      setError("请先搜索并选择所属用户和对端用户");
      return;
    }
    const params = new URLSearchParams({
      owner_user_id: String(owner.ID),
      peer_id: String(peer.ID),
      limit
    });
    if (next && data?.rows.length) {
      const last = data.rows[data.rows.length - 1];
      params.set("before_date", String(last.Date));
      params.set("before_id", String(last.BoxID));
      setBeforeDate(String(last.Date));
      setBeforeID(String(last.BoxID));
    } else {
      if (beforeDate) params.set("before_date", beforeDate);
      if (beforeID) params.set("before_id", beforeID);
    }
    try {
      setData(await api.messages(params));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  function changeOwner(row: AccountRow | null) {
    setOwner(row);
    setBeforeDate("");
    setBeforeID("");
    setData(null);
  }

  function changePeer(row: AccountRow | null) {
    setPeer(row);
    setBeforeDate("");
    setBeforeID("");
    setData(null);
  }

  return (
    <PageFrame title="私聊消息" eyebrow="私聊消息盒">
      {error && <Alert>{error}</Alert>}
      <QueryPanel>
        <div className="message-selector-grid">
          <UserPicker label="所属用户" value={owner} onChange={changeOwner} />
          <UserPicker label="对端用户" value={peer} onChange={changePeer} />
        </div>
        <form className="toolbar message-query" onSubmit={(event) => { event.preventDefault(); void load(false); }}>
          <input value={beforeDate} onChange={(event) => setBeforeDate(event.target.value)} placeholder="before_date 游标" />
          <input value={beforeID} onChange={(event) => setBeforeID(event.target.value)} placeholder="before_msg_id 游标" />
          <input className="small-input" value={limit} onChange={(event) => setLimit(event.target.value)} placeholder="条数 <= 100" />
          <button className="btn primary icon-text" type="submit"><Search size={15} /> 查询消息</button>
          {data?.rows.length ? <button className="btn icon-text" type="button" onClick={() => load(true)}><ChevronRight size={15} /> 下一页</button> : null}
        </form>
      </QueryPanel>
      <div className="metric-row">
        <Metric label="当前页消息" value={String(data?.rows.length ?? 0)} />
        <Metric label="已删除" value={String((data?.rows ?? []).filter((row) => row.Deleted).length)} tone="danger" />
        <Metric label="发出消息" value={String((data?.rows ?? []).filter((row) => row.Outgoing).length)} />
        <Metric label="所属 / 对端" value={owner && peer ? `${displayName(owner)} / ${displayName(peer)}` : "-"} />
      </div>
      <div className="operation-row">
        <div className="operation-box">
          <div className="operation-title"><Trash2 size={15} /> 删除指定消息</div>
          <input value={ids} onChange={(event) => setIDs(event.target.value)} placeholder="消息 ID，逗号分隔" />
          <label className="checkline"><input type="checkbox" checked={revoke} onChange={(event) => setRevoke(event.target.checked)} /> 同步撤回</label>
          <ActionButton path="/api/actions/delete-messages" label="预演删除" payload={() => ({
            owner_user_id: owner?.ID ?? 0,
            peer_id: peer?.ID ?? 0,
            ids: parseIDs(ids),
            revoke
          })} />
        </div>
        <div className="operation-box">
          <div className="operation-title"><History size={15} /> 清空私聊历史</div>
          <input value={maxID} onChange={(event) => setMaxID(event.target.value)} placeholder="max_id 截止消息" />
          <input value={maxBatches} onChange={(event) => setMaxBatches(event.target.value)} placeholder="max_batches 批次数" />
          <label className="checkline"><input type="checkbox" checked={revoke} onChange={(event) => setRevoke(event.target.checked)} /> 同步撤回</label>
          <label className="checkline"><input type="checkbox" checked={justClear} onChange={(event) => setJustClear(event.target.checked)} /> 仅清本侧</label>
          <ActionButton path="/api/actions/delete-history" label="预演清历史" payload={() => ({
            owner_user_id: owner?.ID ?? 0,
            peer_id: peer?.ID ?? 0,
            max_id: toInt(maxID),
            max_batches: toInt(maxBatches),
            just_clear: justClear,
            revoke
          })} />
        </div>
      </div>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>消息 ID</th>
              <th>时间</th>
              <th>发送方</th>
              <th>方向</th>
              <th>PTS</th>
              <th>状态</th>
              <th>正文</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {data?.rows.map((row) => (
              <tr key={`${row.OwnerUserID}-${row.BoxID}`}>
                <td className="mono">{row.BoxID}</td>
                <td>{formatUnix(row.Date)}</td>
                <td className="mono">{row.FromUserID}</td>
                <td>{row.Outgoing ? "发出" : "收到"}</td>
                <td>{row.PTS}</td>
                <td>{row.Deleted ? <Badge tone="danger">已删除</Badge> : <Badge>存活</Badge>}</td>
                <td className="truncate">{row.Body}</td>
                <td>
                  <button
                    className="row-link"
                    onClick={() => navigate(`/messages/private/detail?owner_user_id=${row.OwnerUserID}&msg_id=${row.BoxID}`)}
                  >
                    详情 <ChevronRight size={14} />
                  </button>
                </td>
              </tr>
            ))}
            {(!data || data.rows.length === 0) && <EmptyRow colSpan={8} />}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
