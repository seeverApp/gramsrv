export type Navigate = (href: string) => void;

export type RouteState = {
  href: string;
  path: string;
  search: URLSearchParams;
};

export function currentRoute(): RouteState {
  return {
    href: `${window.location.pathname}${window.location.search}`,
    path: window.location.pathname,
    search: new URLSearchParams(window.location.search)
  };
}

export function routeTitle(pathname: string): string {
  if (pathname.startsWith("/accounts")) return "账号管理";
  if (pathname.startsWith("/channels")) return "超级群与频道";
  if (pathname.startsWith("/messages")) return "消息审计";
  return "运维控制台";
}

export function routeSubtitle(pathname: string): string {
  if (pathname.startsWith("/accounts")) return "控制台 / 账号";
  if (pathname.startsWith("/channels")) return "控制台 / 频道";
  if (pathname.startsWith("/messages")) return "控制台 / 消息";
  return "控制台 / 总览";
}
