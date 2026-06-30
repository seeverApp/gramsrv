import { ArrowLeft, BadgeCheck, CircleAlert, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { AuthorizationTable } from "../components/AuthorizationTable";
import { Alert, AuditTable, Badge, LoadingSurface, PageFrame, SectionHead, SplitLayout, Summary } from "../components/ui";
import { displayName, displayPhone, displayUsername, formatDate, formatUnix, toInt } from "../lib/format";
import type { Navigate } from "../routing";
import type { AccountDetail } from "../types";

export function AccountDetailPage({ id, navigate }: { id: number; navigate: Navigate }) {
  const [detail, setDetail] = useState<AccountDetail | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [months, setMonths] = useState("1");

  async function load() {
    setBusy(true);
    setError("");
    try {
      setDetail(await api.account(id));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    void load();
  }, [id]);

  if (error) {
    return <Alert>{error}</Alert>;
  }
  if (!detail) {
    return <LoadingSurface label={busy ? "加载账号详情" : "等待数据"} />;
  }

  const account = detail.Account;
  return (
    <PageFrame
      title={`账号 #${account.ID}`}
      eyebrow="账号档案"
      actions={<button className="btn icon-text" onClick={() => navigate("/accounts")}><ArrowLeft size={15} /> 返回列表</button>}
    >
      <SplitLayout
        main={
          <div className="stacked-sections">
            <section className="entity-head">
              <div>
                <div className="entity-title">{displayName(account)}</div>
                <div className="entity-subtitle">{displayUsername(account.Username) || "无用户名"} · {displayPhone(account.Phone) || "无手机号"}</div>
              </div>
              <div className="entity-badges">
                {account.PremiumUntil > 0 ? <Badge tone="good">会员</Badge> : <Badge>非会员</Badge>}
                {detail.Verified ? <Badge tone="good">已认证</Badge> : <Badge>未认证</Badge>}
                {account.Frozen ? <Badge tone="danger">发消息冻结</Badge> : <Badge>发送正常</Badge>}
              </div>
            </section>
            <div className="summary-grid">
              <Summary label="用户 ID" value={String(account.ID)} mono />
              <Summary label="最后在线" value={formatUnix(detail.LastSeenAt) || "-"} />
              <Summary label="会员到期" value={account.PremiumUntil > 0 ? formatUnix(account.PremiumUntil) : "无"} />
              <Summary label="更新时间" value={formatDate(account.UpdatedAt) || "-"} />
              <Summary label="授权设备" value={String(detail.Authorizations.length)} />
              <Summary label="账号标记" value={`support=${detail.Support} bot=${detail.Bot}`} />
              <Summary label="限制状态" value={detail.HasRestriction ? detail.Restriction.Reason || "已限制" : "无"} />
              <Summary label="创建时间" value={formatDate(account.CreatedAt) || "-"} />
            </div>
            {detail.About && <p className="about-text">{detail.About}</p>}
            <section className="section-block">
              <SectionHead title="授权设备" text={`共 ${detail.Authorizations.length} 个授权`} />
              <AuthorizationTable rows={detail.Authorizations} userID={account.ID} onDone={load} />
            </section>
            <section className="section-block">
              <SectionHead title="最近后台操作" text="最近 30 条审计" />
              <AuditTable rows={detail.AuditLogs} />
            </section>
          </div>
        }
        side={
          <section className="action-dock">
            <div className="dock-title">账号操作</div>
            <ActionButton
              label={account.Frozen ? "解冻发消息" : "冻结发消息"}
              icon={<CircleAlert size={15} />}
              path="/api/actions/freeze-send"
              payload={() => ({ user_id: account.ID, frozen: !account.Frozen })}
              onDone={load}
            />
            <label className="duration-field">
              <span>会员时长（月）</span>
              <input
                aria-label="设置会员时长，单位月"
                value={months}
                onChange={(event) => setMonths(event.target.value)}
                type="number"
                min="1"
                max="120"
              />
            </label>
            <div className="action-stack">
              <ActionButton
                label="设置会员"
                icon={<Sparkles size={15} />}
                tone="warn"
                path="/api/actions/grant-premium"
                payload={() => ({ user_id: account.ID, months: toInt(months) })}
                onDone={load}
              />
              <ActionButton
                label="取消会员"
                icon={<Sparkles size={15} />}
                tone="warn"
                path="/api/actions/grant-premium"
                payload={() => ({ user_id: account.ID, months: 0 })}
                onDone={load}
              />
              <ActionButton
                label={detail.Verified ? "取消认证" : "设置认证"}
                icon={<BadgeCheck size={15} />}
                tone="warn"
                path="/api/actions/set-verified"
                payload={() => ({ user_id: account.ID, verified: !detail.Verified })}
                onDone={load}
              />
            </div>
          </section>
        }
      />
    </PageFrame>
  );
}
