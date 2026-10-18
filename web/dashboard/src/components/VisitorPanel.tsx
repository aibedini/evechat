import { useState } from "react";
import type { Conversation, Visitor } from "@/lib/types";
import { t } from "@/lib/i18n";
import { formatDateTime, maskPhone, timeAgo } from "@/lib/utils";
import { Button } from "./ui/button";

function Row({ label, value }: { label: string; value?: string | null }) {
  return (
    <div className="flex items-start justify-between gap-3 py-1 text-xs">
      <span className="text-slate-400">{label}</span>
      <span className="max-w-[65%] truncate text-end text-slate-700">{value || "—"}</span>
    </div>
  );
}

function daysLeft(expiry?: string | null): number | null {
  if (!expiry) return null;
  const d = new Date(expiry);
  if (isNaN(d.getTime())) return null;
  return Math.ceil((d.getTime() - Date.now()) / 86400000);
}

export function VisitorPanel({
  visitor,
  conversation,
  history,
  onBan,
  onUnban,
  onConvertTicket,
}: {
  visitor?: Visitor;
  conversation?: Conversation;
  history: Conversation[];
  onBan: () => void;
  onUnban: () => void;
  onConvertTicket: () => void;
}) {
  const [tab, setTab] = useState<"info" | "history">("info");
  if (!visitor) {
    return (
      <div className="w-[300px] shrink-0 border-s border-slate-200 bg-white p-4 text-center text-xs text-slate-400">
        —
      </div>
    );
  }
  const sub = visitor.eve_subscription_info || {};
  const plan = (sub as any).plan || (sub as any).subscription_plan || "";
  const expiry = (sub as any).expiry || (sub as any).subscription_expiry || "";
  const left = daysLeft(expiry);

  return (
    <aside className="hidden w-[300px] shrink-0 flex-col overflow-y-auto border-s border-slate-200 bg-white xl:flex">
      <div className="border-b border-slate-100 p-4">
        <div className="flex items-center gap-2">
          <span className="grid h-10 w-10 place-items-center rounded-full bg-slate-100 text-sm font-bold text-slate-600">
            {(visitor.name || "؟").slice(0, 2)}
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold">{visitor.name || "کاربر ناشناس"}</div>
            <div className="text-[11px] text-slate-400">
              {visitor.is_banned ? "مسدود" : t("online")} · {timeAgo(visitor.last_seen_at)}
            </div>
          </div>
        </div>
        <div className="mt-3 grid grid-cols-2 gap-1">
          <Button variant={visitor.is_banned ? "ghost" : "danger"} size="sm" onClick={visitor.is_banned ? onUnban : onBan}>
            {visitor.is_banned ? t("unban") : t("ban")}
          </Button>
          <Button variant="ghost" size="sm" onClick={onConvertTicket}>
            {t("convertTicket")}
          </Button>
        </div>
      </div>

      <div className="flex border-b border-slate-100 text-xs">
        {(["info", "history"] as const).map((k) => (
          <button
            key={k}
            onClick={() => setTab(k)}
            className={
              "flex-1 py-2 font-medium " +
              (tab === k ? "border-b-2 border-brand-600 text-brand-700" : "text-slate-400")
            }
          >
            {k === "info" ? t("visitorInfo") : t("pastConversations")}
          </button>
        ))}
      </div>

      {tab === "info" ? (
        <div className="space-y-3 p-4">
          <div>
            <div className="mb-1 text-[10px] font-bold uppercase text-slate-400">{t("visitorInfo")}</div>
            <Row label="موبایل" value={maskPhone(visitor.phone || visitor.eve_mobile)} />
            <Row label="ایمیل" value={visitor.email} />
            <Row label="دستگاه" value={visitor.device_type} />
            <Row label="سیستم" value={visitor.os} />
            <Row label="مرورگر" value={visitor.browser} />
            <Row label="زبان" value={visitor.language} />
            <Row label="IP" value={visitor.ip} />
          </div>

          <div className="rounded-xl bg-slate-50 p-3">
            <div className="mb-1 text-[10px] font-bold uppercase text-slate-400">EVE</div>
            <Row label="شناسه کاربر" value={visitor.eve_user_id} />
            <Row label="نمایندگی" value={visitor.eve_reseller_id} />
            <Row label={t("plan")} value={plan || t("noSubscription")} />
            <Row label={t("expiry")} value={expiry ? formatDateTime(expiry) : undefined} />
            <Row label={t("daysLeft")} value={left !== null ? String(left) : undefined} />
          </div>

          <div className="rounded-xl bg-slate-50 p-3">
            <div className="mb-1 text-[10px] font-bold uppercase text-slate-400">صفحه فعلی</div>
            <div className="truncate text-xs text-brand-700">{conversation?.page_url || "—"}</div>
            <div className="mt-1 truncate text-[11px] text-slate-500">{conversation?.page_title || ""}</div>
          </div>

          {visitor.metadata && Object.keys(visitor.metadata).length > 0 && (
            <div className="rounded-xl bg-slate-50 p-3">
              <div className="mb-1 text-[10px] font-bold uppercase text-slate-400">متادیتا</div>
              {Object.entries(visitor.metadata).map(([k, v]) => (
                <Row key={k} label={k} value={String(v)} />
              ))}
            </div>
          )}
        </div>
      ) : (
        <div className="p-2">
          {history.length === 0 && <div className="p-4 text-center text-xs text-slate-400">—</div>}
          {history.map((c) => (
            <div key={c.id} className="rounded-xl px-3 py-2 text-xs hover:bg-slate-50">
              <div className="flex justify-between">
                <span className="font-medium">{c.subject || c.id.slice(-6)}</span>
                <span className="text-slate-400">{timeAgo(c.created_at)}</span>
              </div>
              <div className="text-[11px] text-slate-400">{c.status}</div>
            </div>
          ))}
        </div>
      )}
    </aside>
  );
}
