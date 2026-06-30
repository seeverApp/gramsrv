import { ArrowLeft, BadgeCheck } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { Alert, AuditTable, Badge, JsonBlock, LoadingSurface, PageFrame, SectionHead, SplitLayout, Summary } from "../components/ui";
import { channelKind, displayUsername, formatDate, formatUnix } from "../lib/format";
import type { Navigate } from "../routing";
import type { ChannelDetail } from "../types";

export function ChannelDetailPage({ id, navigate }: { id: number; navigate: Navigate }) {
  const [detail, setDetail] = useState<ChannelDetail | null>(null);
  const [error, setError] = useState("");

  async function load() {
    setError("");
    try {
      setDetail(await api.channel(id));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  useEffect(() => {
    void load();
  }, [id]);

  if (error) {
    return <Alert>{error}</Alert>;
  }
  if (!detail) {
    return <LoadingSurface label="加载频道详情" />;
  }

  const ch = detail.Channel;
  return (
    <PageFrame
      title={`${channelKind(ch)} #${ch.ID}`}
      eyebrow="频道档案"
      actions={<button className="btn icon-text" onClick={() => navigate("/channels")}><ArrowLeft size={15} /> 返回列表</button>}
    >
      <SplitLayout
        main={
          <div className="stacked-sections">
            <section className="entity-head">
              <div>
                <div className="entity-title">{ch.Title || "-"}</div>
                <div className="entity-subtitle">{displayUsername(ch.Username) || "无用户名"} · 创建者 {ch.CreatorUserID}</div>
              </div>
              <div className="entity-badges">
                <Badge>{channelKind(ch)}</Badge>
                {ch.Verified ? <Badge tone="good">已认证</Badge> : <Badge>未认证</Badge>}
                {ch.Deleted ? <Badge tone="danger">已删除</Badge> : <Badge>有效</Badge>}
              </div>
            </section>
            <div className="summary-grid">
              <Summary label="频道 ID" value={String(ch.ID)} mono />
              <Summary label="access_hash" value={String(ch.AccessHash)} mono />
              <Summary label="成员" value={`${ch.ParticipantsCount} / 管理员 ${ch.AdminsCount}`} />
              <Summary label="治理状态" value={`封禁 ${ch.BannedCount} / 踢出 ${ch.KickedCount}`} />
              <Summary label="频道标记" value={`broadcast=${ch.Broadcast} megagroup=${ch.Megagroup} forum=${ch.Forum}`} />
              <Summary label="top / pinned / PTS" value={`${ch.TopMessageID} / ${ch.PinnedMessageID} / ${ch.PTS}`} />
              <Summary label="创建时间" value={formatUnix(ch.Date) || "-"} />
              <Summary label="更新时间" value={formatDate(ch.UpdatedAt) || "-"} />
            </div>
            {ch.About && <p className="about-text">{ch.About}</p>}
            <section className="section-block">
              <SectionHead title="最近后台操作" text="最近 30 条审计" />
              <AuditTable rows={detail.AuditLogs} />
            </section>
            <section className="section-block">
              <SectionHead title="频道原始行" text="数据库只读快照" />
              <JsonBlock value={detail.ChannelJSON} />
            </section>
          </div>
        }
        side={
          <section className="action-dock">
            <div className="dock-title">频道操作</div>
            <ActionButton
              label={ch.Verified ? "取消认证" : "设置认证"}
              icon={<BadgeCheck size={15} />}
              tone="warn"
              path="/api/actions/set-channel-verified"
              payload={() => ({ channel_id: ch.ID, verified: !ch.Verified })}
              onDone={load}
            />
          </section>
        }
      />
    </PageFrame>
  );
}
