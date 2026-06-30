import { CheckCircle2, ChevronRight, Clock3, FileJson, KeyRound, MessageSquareText, ShieldCheck, Users } from "lucide-react";
import type { ReactNode } from "react";
import { AppLink } from "../components/AppLink";
import { StatusItem } from "../components/ui";
import type { Navigate } from "../routing";

export function Dashboard({ navigate }: { navigate: Navigate }) {
  return (
    <div className="dashboard-layout">
      <section className="overview-band">
        <div>
          <div className="eyebrow">运行总览</div>
          <h2>控制台总览</h2>
        </div>
        <div className="overview-metrics">
          <StatusItem label="读路径" value="PG 只读" tone="neutral" />
          <StatusItem label="写路径" value="Admin API" tone="good" />
          <StatusItem label="执行策略" value="先预演" tone="warn" />
        </div>
      </section>
      <div className="command-grid">
        <Launcher icon={<Users />} title="账号管理" text="账号状态、会员、认证、会话。" href="/accounts" navigate={navigate} />
        <Launcher icon={<ShieldCheck />} title="超级群与频道" text="公开实体、成员计数、认证状态。" href="/channels" navigate={navigate} />
        <Launcher icon={<MessageSquareText />} title="消息审计" text="消息盒、update、outbox 状态。" href="/messages" navigate={navigate} />
      </div>
      <section className="work-strip">
        <div className="strip-item"><CheckCircle2 size={16} /><span>所有危险操作先预演</span></div>
        <div className="strip-item"><KeyRound size={16} /><span>浏览器不持有内部 token</span></div>
        <div className="strip-item"><Clock3 size={16} /><span>列表使用游标分页</span></div>
        <div className="strip-item"><FileJson size={16} /><span>详情页保留原始状态快照</span></div>
      </section>
    </div>
  );
}

function Launcher({
  icon,
  title,
  text,
  href,
  navigate
}: {
  icon: ReactNode;
  title: string;
  text: string;
  href: string;
  navigate: Navigate;
}) {
  return (
    <AppLink className="launcher" href={href} navigate={navigate}>
      <span className="launcher-icon">{icon}</span>
      <span className="launcher-copy">
        <strong>{title}</strong>
        <span>{text}</span>
      </span>
      <ChevronRight size={16} />
    </AppLink>
  );
}
