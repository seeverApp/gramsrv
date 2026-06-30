import type { AccountRow, ChannelRow } from "../types";

export function displayPhone(value: string): string {
  const phone = value.trim();
  if (!phone || phone.startsWith("+")) return phone;
  return /^\d+$/.test(phone) ? `+${phone}` : phone;
}

export function displayUsername(value: string): string {
  const username = value.trim();
  if (!username) return "";
  return username.startsWith("@") ? username : `@${username}`;
}

export function displayName(row: Pick<AccountRow, "FirstName" | "LastName">): string {
  return `${row.FirstName || ""} ${row.LastName || ""}`.trim() || "-";
}

export function channelKind(ch: ChannelRow): string {
  if (ch.Broadcast && !ch.Megagroup) return "频道";
  if (ch.Megagroup && ch.Forum) return "超级群/论坛";
  if (ch.Megagroup) return "超级群";
  return "频道/群";
}

export function formatDate(value: string): string {
  if (!value || value.startsWith("0001-")) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString();
}

export function formatUnix(value: number): string {
  if (!value || value <= 0) return "";
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString();
}

export function toInt(value: string): number {
  if (!value.trim()) return 0;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function parseIDs(value: string): number[] {
  const ids = value
    .split(/[\s,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => Number.parseInt(item, 10));
  if (ids.length === 0 || ids.some((id) => !Number.isFinite(id) || id <= 0)) {
    throw new Error("msg ids invalid");
  }
  return ids;
}
