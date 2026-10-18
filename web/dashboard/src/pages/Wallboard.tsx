import { useEffect, useState } from "react";

interface WallboardData {
  project: string;
  generated_at: string;
  window: string;
  agents: { online: number; total: number };
  pending: number;
  active: number;
  resolved: number;
  csat_avg: number;
  first_response_seconds: number;
}

/**
 * Read-only big screen. The token in the URL is the only credential and the
 * payload has aggregates only: no visitor names, no message content.
 */
export function WallboardPage() {
  const token = new URLSearchParams(location.search).get("token") || "";
  const [data, setData] = useState<WallboardData | null>(null);
  const [error, setError] = useState("");
  const [clock, setClock] = useState(new Date());

  useEffect(() => {
    const timer = setInterval(() => setClock(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const res = await fetch(`/api/v1/wallboard?token=${encodeURIComponent(token)}`, { credentials: "omit" });
        if (!res.ok) throw new Error("unavailable");
        const body = (await res.json()) as WallboardData;
        if (alive) {
          setData(body);
          setError("");
        }
      } catch {
        if (alive) setError("اتصال به سرور برقرار نیست");
      }
    };
    if (token) {
      load();
    } else {
      setError("توکن صفحه‌ی نمایش داده نشده است");
    }
    const timer = setInterval(load, 15000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [token]);

  const tiles = data
    ? [
        { label: "در صف", value: data.pending, tone: "text-amber-300" },
        { label: "در حال بررسی", value: data.active, tone: "text-emerald-300" },
        { label: "پاسخ امروز", value: data.resolved, tone: "text-sky-300" },
        { label: "کارشناسان آنلاین", value: `${data.agents.online}/${data.agents.total}`, tone: "text-violet-300" },
        { label: "میانگین پاسخ اول", value: `${Math.round(data.first_response_seconds)}s`, tone: "text-slate-200" },
        { label: "رضایت", value: data.csat_avg ? data.csat_avg.toFixed(1) : "—", tone: "text-slate-200" },
      ]
    : [];

  return (
    <div className="flex min-h-screen flex-col bg-slate-950 p-8 text-white" dir="rtl">
      <header className="mb-8 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="grid h-12 w-12 place-items-center rounded-2xl bg-brand-600 text-2xl">⚡</span>
          <div>
            <div className="text-2xl font-bold">{data?.project || "EveChat"}</div>
            <div className="text-xs text-slate-500">صفحه‌ی نمایش — فقط خواندنی</div>
          </div>
        </div>
        <div className="text-end">
          <div className="font-mono text-3xl tabular-nums">{clock.toLocaleTimeString("fa-IR")}</div>
          <div className="text-xs text-slate-500">{clock.toLocaleDateString("fa-IR")} · پنجره ۲۴ ساعت</div>
        </div>
      </header>

      {error && (
        <div className="mb-6 rounded-2xl border border-red-900 bg-red-950/60 px-4 py-3 text-sm text-red-200">{error}</div>
      )}

      <div className="grid flex-1 grid-cols-2 gap-4 lg:grid-cols-3">
        {tiles.map((tile) => (
          <div key={tile.label} className="flex flex-col justify-between rounded-3xl bg-slate-900 p-6">
            <div className="text-sm text-slate-400">{tile.label}</div>
            <div className={`mt-4 text-6xl font-black tabular-nums ${tile.tone}`}>{tile.value}</div>
          </div>
        ))}
      </div>

      <footer className="mt-6 text-center text-[11px] text-slate-600">
        به‌روزرسانی خودکار هر ۱۵ ثانیه · بدون محتوای پیام و بدون اطلاعات بازدیدکننده
      </footer>
    </div>
  );
}
