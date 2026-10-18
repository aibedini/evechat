import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { formatJalali, formatJalaliTime, formatJalaliLong, JALALI_MONTHS } from "./jalali";

export { formatJalali, formatJalaliTime, formatJalaliLong, JALALI_MONTHS };

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

export function formatNumber(n: number | undefined | null): string {
  if (n === undefined || n === null || isNaN(n)) return "۰";
  return new Intl.NumberFormat("fa-IR").format(n);
}

export function formatDuration(seconds: number | undefined | null): string {
  if (!seconds || seconds <= 0) return "—";
  if (seconds < 60) return `${Math.round(seconds)} ثانیه`;
  const minutes = seconds / 60;
  if (minutes < 60) return `${Math.round(minutes)} دقیقه`;
  const hours = minutes / 60;
  if (hours < 24) return `${hours.toFixed(1)} ساعت`;
  return `${(hours / 24).toFixed(1)} روز`;
}

export function formatDateTime(raw?: string | null): string {
  if (!raw) return "—";
  const d = new Date(raw);
  if (isNaN(d.getTime())) return "—";
  return `${formatJalali(d)} ${formatJalaliTime(d)}`;
}

export function timeAgo(raw?: string | null): string {
  if (!raw) return "";
  const then = new Date(raw).getTime();
  if (isNaN(then)) return "";
  const diff = Math.max(0, Date.now() - then);
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return "همین حالا";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min} دقیقه پیش`;
  const hour = Math.floor(min / 60);
  if (hour < 24) return `${hour} ساعت پیش`;
  const day = Math.floor(hour / 24);
  if (day < 30) return `${day} روز پیش`;
  return formatJalali(new Date(then));
}

export function initials(name?: string | null): string {
  const n = (name || "").trim();
  if (!n) return "؟";
  const parts = n.split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2);
  return (parts[0][0] || "") + (parts[1][0] || "");
}

export function statusLabel(status: string): string {
  switch (status) {
    case "pending":
      return "در انتظار";
    case "active":
      return "فعال";
    case "resolved":
      return "حل‌شده";
    case "closed":
      return "بسته";
    default:
      return status;
  }
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
