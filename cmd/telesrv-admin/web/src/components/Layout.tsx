import {
  ChevronDown,
  Database,
  LayoutDashboard,
  LogOut,
  MessageSquareText,
  Server,
  Shield,
  ShieldCheck,
  Users
} from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { api } from "../api";
import { type Navigate, type RouteState, routeSubtitle, routeTitle } from "../routing";
import { AppLink } from "./AppLink";

export function BootScreen() {
  return (
    <div className="boot-screen">
      <div className="brand compact brand-elevated">
        <span className="brand-mark">T</span>
        <span>
          <strong>telesrv</strong>
          <small>管理控制台</small>
        </span>
      </div>
      <div className="loader-bar" />
    </div>
  );
}

export function Shell({
  actor,
  route,
  navigate,
  onLogout,
  children
}: {
  actor: string;
  route: RouteState;
  navigate: Navigate;
  onLogout: () => void;
  children: ReactNode;
}) {
  const messagesActive = route.path.startsWith("/messages");
  const [messagesOpen, setMessagesOpen] = useState(messagesActive);

  useEffect(() => {
    if (messagesActive) {
      setMessagesOpen(true);
    }
  }, [messagesActive]);

  async function logout() {
    await api.logout().catch(() => undefined);
    onLogout();
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <AppLink className="brand" href="/" navigate={navigate}>
          <span className="brand-mark">T</span>
          <span>
            <strong>telesrv</strong>
            <small>管理控制台</small>
          </span>
        </AppLink>
        <div className="sidebar-label">导航</div>
        <nav className="nav-list" aria-label="主导航">
          <NavLink icon={<LayoutDashboard size={16} />} href="/" route={route} navigate={navigate}>总览</NavLink>
          <NavLink icon={<Users size={16} />} href="/accounts" route={route} navigate={navigate}>账号</NavLink>
          <NavLink icon={<ShieldCheck size={16} />} href="/channels" route={route} navigate={navigate}>超级群/频道</NavLink>
          <div className={`nav-section ${messagesActive ? "active" : ""} ${messagesOpen ? "open" : ""}`}>
            <button
              className="nav-section-toggle"
              type="button"
              aria-expanded={messagesOpen}
              onClick={() => setMessagesOpen((open) => !open)}
            >
              <MessageSquareText size={16} />
              <span>消息</span>
              <ChevronDown className="nav-section-chevron" size={15} />
            </button>
            {messagesOpen && (
              <div className="nav-children">
                <NavLink
                  href="/messages/private"
                  route={route}
                  navigate={navigate}
                  activeWhen={(path) => path === "/messages" || path === "/messages/detail" || path.startsWith("/messages/private")}
                >
                  私聊
                </NavLink>
                <NavLink
                  href="/messages/groups"
                  route={route}
                  navigate={navigate}
                  activeWhen={(path) => path.startsWith("/messages/groups")}
                >
                  群聊
                </NavLink>
              </div>
            )}
          </div>
        </nav>
        <div className="sidebar-status">
          <div className="sidebar-label">运行状态</div>
          <div className="runtime-row"><Server size={14} /><span>管理后台</span><strong>就绪</strong></div>
          <div className="runtime-row"><Database size={14} /><span>PG 读取</span><strong>只读</strong></div>
          <div className="runtime-row"><Shield size={14} /><span>写操作</span><strong>预演</strong></div>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <div>
            <div className="eyebrow">{routeSubtitle(route.path)}</div>
            <h1>{routeTitle(route.path)}</h1>
          </div>
          <div className="topbar-actions">
            <span className="actor-pill">操作者：{actor}</span>
            <button className="btn ghost icon-text" type="button" onClick={logout} title="退出">
              <LogOut size={16} /> 退出
            </button>
          </div>
        </header>
        <main className="content">{children}</main>
      </div>
    </div>
  );
}

function NavLink({
  href,
  route,
  navigate,
  icon,
  children,
  activeWhen
}: {
  href: string;
  route: RouteState;
  navigate: Navigate;
  icon?: ReactNode;
  children: ReactNode;
  activeWhen?: (path: string) => boolean;
}) {
  const active = activeWhen ? activeWhen(route.path) : href === "/" ? route.path === "/" : route.path.startsWith(href);
  return (
    <AppLink className={`nav-item ${active ? "active" : ""}`} href={href} navigate={navigate}>
      {icon ?? <span aria-hidden="true" className="nav-dot" />}
      <span>{children}</span>
    </AppLink>
  );
}
