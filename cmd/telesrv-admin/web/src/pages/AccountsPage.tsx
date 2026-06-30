import { ChevronRight, Loader2, RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { Alert, Badge, EmptyRow, Metric, PageFrame, QueryPanel } from "../components/ui";
import { displayName, displayPhone, displayUsername, formatDate, formatUnix } from "../lib/format";
import { accountMetrics } from "../lib/metrics";
import type { Navigate } from "../routing";
import type { AccountListResponse } from "../types";

export function AccountsPage({ navigate }: { navigate: Navigate }) {
  const [q, setQ] = useState("");
  const [limit, setLimit] = useState("50");
  const [data, setData] = useState<AccountListResponse | null>(null);
  const [cursor, setCursor] = useState({ beforeID: 0, beforeActiveUS: 0 });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function load(next = false) {
    setBusy(true);
    setError("");
    const params = new URLSearchParams({ limit });
    if (q.trim()) {
      params.set("q", q.trim());
    } else if (next) {
      params.set("before_id", String(cursor.beforeID));
      params.set("before_active_us", String(cursor.beforeActiveUS));
    }
    try {
      const result = await api.accounts(params);
      setData(result);
      setCursor({
        beforeID: result.next_before_id,
        beforeActiveUS: result.next_before_active_us
      });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    void load(false);
  }, []);

  const metrics = accountMetrics(data?.rows ?? []);

  return (
    <PageFrame
      title="账号"
      eyebrow={data?.listing === false ? "查询结果" : "最近活跃账号"}
      actions={
        <button className="btn" type="button" onClick={() => load(false)} disabled={busy}>
          <RefreshCw size={15} /> 刷新
        </button>
      }
    >
      {error && <Alert>{error}</Alert>}
      <div className="metric-row">
        <Metric label="当前页账号" value={String(data?.rows.length ?? 0)} />
        <Metric label="在线设备记录" value={String(metrics.devices)} />
        <Metric label="会员" value={String(metrics.premium)} tone="good" />
        <Metric label="冻结" value={String(metrics.frozen)} tone={metrics.frozen > 0 ? "danger" : "neutral"} />
      </div>
      <QueryPanel>
        <form className="toolbar" onSubmit={(event) => { event.preventDefault(); void load(false); }}>
          <label className="searchbox">
            <Search size={15} />
            <input value={q} onChange={(event) => setQ(event.target.value)} placeholder="用户 ID / 手机号 / 用户名" />
          </label>
          <label className="field-inline">
            <span>条数</span>
            <input className="small-input" value={limit} onChange={(event) => setLimit(event.target.value)} type="number" min="1" max="100" />
          </label>
          <button className="btn primary icon-text" type="submit" disabled={busy}>
            {busy ? <Loader2 size={15} className="spin" /> : <Search size={15} />} 查询
          </button>
          {data?.listing && data.has_more && (
            <button className="btn icon-text" type="button" onClick={() => load(true)} disabled={busy}>
              <ChevronRight size={15} /> 下一页
            </button>
          )}
        </form>
      </QueryPanel>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>用户 ID</th>
              <th>手机号</th>
              <th>用户名</th>
              <th>姓名</th>
              <th>设备</th>
              <th>最近活跃</th>
              <th>会员</th>
              <th>认证</th>
              <th>冻结</th>
              <th>更新时间</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {data?.rows.map((row) => (
              <tr key={row.ID}>
                <td className="mono">{row.ID}</td>
                <td>{displayPhone(row.Phone)}</td>
                <td>{displayUsername(row.Username)}</td>
                <td>{displayName(row)}</td>
                <td>{row.DeviceCount}</td>
                <td>{formatDate(row.LastActiveAt)}</td>
                <td>{row.PremiumUntil > 0 ? <Badge tone="good">会员 {formatUnix(row.PremiumUntil)}</Badge> : <Badge>无</Badge>}</td>
                <td>{row.Verified ? <Badge tone="good">已认证</Badge> : <Badge>未认证</Badge>}</td>
                <td>{row.Frozen ? <Badge tone="danger">冻结</Badge> : <Badge>正常</Badge>}</td>
                <td>{formatDate(row.UpdatedAt)}</td>
                <td><button className="row-link" onClick={() => navigate(`/accounts/${row.ID}`)}>详情 <ChevronRight size={14} /></button></td>
              </tr>
            ))}
            {(!data || data.rows.length === 0) && <EmptyRow colSpan={11} />}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}
