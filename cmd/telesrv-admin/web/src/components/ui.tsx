import { CircleAlert } from "lucide-react";
import type { ReactNode } from "react";
import { formatDate } from "../lib/format";
import type { AuditLogRow } from "../types";

type Tone = "neutral" | "good" | "danger" | "warn";

export function PageFrame({
  title,
  eyebrow,
  children,
  actions
}: {
  title: string;
  eyebrow?: string;
  children: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="page-frame">
      <div className="page-title-row">
        <div>
          {eyebrow && <div className="eyebrow">{eyebrow}</div>}
          <h2>{title}</h2>
        </div>
        {actions && <div className="page-actions">{actions}</div>}
      </div>
      {children}
    </div>
  );
}

export function QueryPanel({ children }: { children: ReactNode }) {
  return <div className="query-panel">{children}</div>;
}

export function SplitLayout({ main, side }: { main: ReactNode; side: ReactNode }) {
  return (
    <div className="split-layout">
      <div className="split-main">{main}</div>
      <aside className="split-side">{side}</aside>
    </div>
  );
}

export function SectionHead({ title, text, action }: { title: string; text?: string; action?: ReactNode }) {
  return (
    <div className="section-head">
      <div>
        <h2>{title}</h2>
        {text && <p>{text}</p>}
      </div>
      {action && <div className="section-action">{action}</div>}
    </div>
  );
}

export function Alert({ children }: { children: ReactNode }) {
  return <div className="alert"><CircleAlert size={16} /> <span>{children}</span></div>;
}

export function Badge({ children, tone = "neutral" }: { children: ReactNode; tone?: Tone }) {
  return <span className={`badge ${tone}`}>{children}</span>;
}

export function StatusItem({ label, value, tone }: { label: string; value: string; tone: "neutral" | "good" | "warn" }) {
  return (
    <div className={`status-item ${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function Metric({ label, value, tone = "neutral", mono = false }: { label: string; value: string; tone?: Tone; mono?: boolean }) {
  return (
    <div className={`metric ${tone}`}>
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>{value}</strong>
    </div>
  );
}

export function Summary({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="summary-item">
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>{value}</strong>
    </div>
  );
}

export function AuditTable({ rows }: { rows: AuditLogRow[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead><tr><th>ID</th><th>命令 ID</th><th>动作</th><th>操作者</th><th>状态</th><th>预演</th><th>原因</th><th>时间</th></tr></thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.ID}>
              <td>{row.ID}</td>
              <td className="mono">{row.CommandID}</td>
              <td>{row.Action}</td>
              <td>{row.Actor}</td>
              <td>{row.Status}</td>
              <td>{row.DryRun ? "是" : "否"}</td>
              <td className="truncate">{row.Reason}</td>
              <td>{formatDate(row.CreatedAt)}</td>
            </tr>
          ))}
          {rows.length === 0 && <EmptyRow colSpan={8} />}
        </tbody>
      </table>
    </div>
  );
}

export function EmptyRow({ colSpan }: { colSpan: number }) {
  return <tr><td colSpan={colSpan} className="empty-cell">无结果</td></tr>;
}

export function LoadingSurface({ label }: { label: string }) {
  return <section className="surface"><div className="loading-line">{label}</div></section>;
}

export function JsonBlock({ value }: { value: string }) {
  return <pre className="json-block">{value || "{}"}</pre>;
}
