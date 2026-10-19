import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { formatJalali, formatJalaliTime, formatJalaliLong, JALALI_MONTHS } from "./jalali";
import { copy } from "./copy.fa";

export { formatJalali, formatJalaliTime, formatJalaliLong, JALALI_MONTHS };

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

export function formatNumber(n: number | undefined | null): string {
  if (n === undefined || n === null || isNaN(n)) return new Intl.NumberFormat("fa-IR").format(0);
  return new Intl.NumberFormat("fa-IR").format(n);
}

function withCount(template: string, n: number): string {
  return template.replace("{n}", String(n));
}

export function formatDuration(seconds: number | undefined | null): string {
  if (!seconds || seconds <= 0) return copy.misc.dash;
  if (seconds < 60) return `${Math.round(seconds)} ${copy.time.seconds}`;
  const minutes = seconds / 60;
  if (minutes < 60) return `${Math.round(minutes)} ${copy.time.minutes}`;
  const hours = minutes / 60;
  if (hours < 24) return `${hours.toFixed(1)} ${copy.time.hours}`;
  return `${(hours / 24).toFixed(1)} ${copy.time.days}`;
}

export function formatDateTime(raw?: string | null): string {
  if (!raw) return copy.misc.dash;
  const d = new Date(raw);
  if (isNaN(d.getTime())) return copy.misc.dash;
  return `${formatJalali(d)} ${formatJalaliTime(d)}`;
}

export function timeAgo(raw?: string | null): string {
  if (!raw) return "";
  const then = new Date(raw).getTime();
  if (isNaN(then)) return "";
  const diff = Math.max(0, Date.now() - then);
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return copy.time.justNow;
  const min = Math.floor(sec / 60);
  if (min < 60) return withCount(copy.time.minutesAgo, min);
  const hour = Math.floor(min / 60);
  if (hour < 24) return withCount(copy.time.hoursAgo, hour);
  const day = Math.floor(hour / 24);
  if (day < 30) return withCount(copy.time.daysAgo, day);
  return formatJalali(new Date(then));
}

export function initials(name?: string | null): string {
  const n = (name || "").trim();
  if (!n) return copy.misc.unknownInitial;
  const parts = n.split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2);
  return (parts[0][0] || "") + (parts[1][0] || "");
}

export function statusLabel(status: string): string {
  const labels: Record<string, string> = { ...copy.status };
  return labels[status] || status;
}

export function statusColor(status: string): string {
  switch (status) {
    case "pending":
      return "bg-amber-100 text-amber-700";
    case "active":
      return "bg-emerald-100 text-emerald-700";
    case "resolved":
      return "bg-slate-200 text-slate-700";
    case "closed":
      return "bg-slate-100 text-slate-500";
    default:
      return "bg-slate-100 text-slate-600";
  }
}

export function priorityColor(priority: string): string {
  switch (priority) {
    case "urgent":
      return "bg-red-100 text-red-700";
    case "high":
      return "bg-orange-100 text-orange-700";
    case "low":
      return "bg-slate-100 text-slate-600";
    default:
      return "bg-blue-100 text-blue-700";
  }
}

export function maskPhone(phone?: string | null): string {
  if (!phone) return "";
  if (phone.length <= 8) return phone;
  return phone.slice(0, 5) + "***" + phone.slice(-4);
}

export function download(filename: string, content: string, type = "application/json"): void {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
