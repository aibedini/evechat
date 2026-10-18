import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AgentStat, OverviewStats, SeriesPoint } from "@/lib/types";
import { t } from "@/lib/i18n";
import { Card, Empty, Spinner } from "@/components/ui/card";
import { formatDuration, formatNumber } from "@/lib/utils";
import { formatJalaliLong } from "@/lib/jalali";

function StatCard({ label, value, hint, icon }: { label: string; value: string; hint?: string; icon: string }) {
  return (
    <Card className="flex items-center gap-4">
      <span className="grid h-11 w-11 shrink-0 place-items-center rounded-2xl bg-brand-50 text-xl">{icon}</span>
      <div className="min-w-0">
        <div className="text-[11px] text-slate-400">{label}</div>
        <div className="text-xl font-bold text-slate-800">{value}</div>
        {hint && <div className="text-[10px] text-slate-400">{hint}</div>}
      </div>
    </Card>
  );
}

function LineChart({ points }: { points: SeriesPoint[] }) {
  if (!points.length) return <Empty>داده‌ای برای نمایش وجود ندارد</Empty>;
  const w = 640;
  const h = 180;
  const pad = 24;
  const max = Math.max(1, ...points.map((p) => Math.max(p.conversations, p.messages)));
  const x = (i: number) => pad + (i * (w - pad * 2)) / Math.max(1, points.length - 1);
  const y = (v: number) => h - pad - (v * (h - pad * 2)) / max;
  const line = (key: "conversations" | "messages" | "resolved") =>
    points.map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)},${y(p[key]).toFixed(1)}`).join(" ");

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full" role="img" aria-label="نمودار ۱۴ روز گذشته">
      {[0, 0.25, 0.5, 0.75, 1].map((f) => (
        <line
          key={f}
          x1={pad}
          x2={w - pad}
          y1={pad + f * (h - pad * 2)}
          y2={pad + f * (h - pad * 2)}
          stroke="#e2e8f0"
          strokeWidth="1"
        />
      ))}
      <path d={line("messages")} fill="none" stroke="#94a3b8" strokeWidth="2" strokeDasharray="4 4" />
      <path d={line("resolved")} fill="none" stroke="#10b981" strokeWidth="2" />
      <path d={line("conversations")} fill="none" stroke="#2563eb" strokeWidth="2.5" />
      {points.map((p, i) => (
        <circle key={p.bucket} cx={x(i)} cy={y(p.conversations)} r="2.5" fill="#2563eb">
          <title>{`${p.bucket}: ${p.conversations}`}</title>
        </circle>
      ))}
    </svg>
  );
}

export function DashboardPage() {
  const overview = useQuery({
    queryKey: ["stats", "overview"],
    queryFn: () => api<{ data: OverviewStats; from: string; to: string }>("/api/v1/stats/overview"),
    refetchInterval: 60000,
  });
  const series = useQuery({
    queryKey: ["stats", "series"],
    queryFn: () => api<{ data: SeriesPoint[] }>("/api/v1/stats/series", { query: { bucket: "day" } }),
    refetchInterval: 120000,
  });
  const agents = useQuery({
    queryKey: ["stats", "agents"],
    queryFn: () => api<{ data: AgentStat[] }>("/api/v1/stats/agents"),
    refetchInterval: 120000,
  });

  const s = overview.data?.data;

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-5 flex items-end justify-between">
        <div>
          <h1 className="text-lg font-bold text-slate-800">{t("dashboard")}</h1>
          <p className="text-xs text-slate-400">۱۴ روز گذشته · {formatJalaliLong(new Date())}</p>
        </div>
      </div>

      {overview.isLoading ? (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
          <StatCard icon="💬" label={t("totalConversations")} value={formatNumber(s?.total_conversations)} hint={`${formatNumber(s?.messages)} پیام`} />
          <StatCard icon="⏳" label={t("pending")} value={formatNumber(s?.pending)} hint={`${formatNumber(s?.active)} فعال`} />
          <StatCard icon="⚡" label={t("avgFirstResponse")} value={formatDuration(s?.avg_first_response_seconds)} />
          <StatCard
            icon="⭐"
            label={t("csat")}
            value={s && s.csat_count > 0 ? `${s.csat_avg.toFixed(1)} / ۵` : "—"}
            hint={`${formatNumber(s?.csat_count)} نظر`}
          />
        </div>
      )}

      <div className="mt-5 grid gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-sm font-semibold">روند گفتگوها</h3>
            <div className="flex gap-3 text-[10px] text-slate-400">
              <span className="flex items-center gap-1"><i className="h-2 w-2 rounded-full bg-brand-600" /> گفتگو</span>
              <span className="flex items-center gap-1"><i className="h-2 w-2 rounded-full bg-emerald-500" /> حل‌شده</span>
              <span className="flex items-center gap-1"><i className="h-2 w-2 rounded-full bg-slate-400" /> پیام</span>
            </div>
          </div>
          {series.isLoading ? <Spinner /> : <LineChart points={series.data?.data || []} />}
        </Card>

        <Card>
          <h3 className="mb-3 text-sm font-semibold">{t("topAgents")}</h3>
          {agents.isLoading ? (
            <Spinner />
          ) : (agents.data?.data || []).length === 0 ? (
            <Empty>—</Empty>
          ) : (
            <table className="w-full text-xs">
              <thead>
                <tr className="text-slate-400">
                  <th className="pb-2 text-start font-medium">کارشناس</th>
                  <th className="pb-2 text-center font-medium">گفتگو</th>
                  <th className="pb-2 text-center font-medium">پاسخ اول</th>
                </tr>
              </thead>
              <tbody>
                {(agents.data?.data || []).slice(0, 8).map((a) => (
                  <tr key={a.agent_id} className="border-t border-slate-50">
                    <td className="py-2">{a.name}</td>
                    <td className="py-2 text-center">{formatNumber(a.conversations)}</td>
                    <td className="py-2 text-center text-slate-500">{formatDuration(a.avg_first_response_seconds)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-4 xl:grid-cols-4">
        <StatCard icon="👥" label={t("visitors")} value={formatNumber(s?.unique_visitors)} />
        <StatCard icon="🎫" label={t("tickets")} value={formatNumber(s?.tickets_open)} hint="باز" />
        <StatCard icon="✅" label={t("resolved")} value={formatNumber(s?.resolved)} />
        <StatCard icon="⌛" label="میانگین حل" value={formatDuration((s?.avg_resolution_minutes || 0) * 60)} />
      </div>
    </div>
  );
}
