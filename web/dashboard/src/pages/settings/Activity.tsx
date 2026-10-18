import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, apiBase, getAccessToken } from "@/lib/api";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/input";
import { Card, Empty, Modal, Spinner } from "@/components/ui/card";
import { formatDateTime } from "@/lib/utils";

interface AuditEntry {
  id: string;
  project_id?: string;
  actor_type: string;
  actor_id?: string;
  actor_name?: string;
  action: string;
  target_type?: string;
  target_id?: string;
  ip?: string;
  user_agent?: string;
  detail: string;
  created_at: string;
}

const ACTIONS = [
  { value: "", label: "همه رویدادها" },
  { value: "agent.", label: "کاربران" },
  { value: "conversation.", label: "گفتگوها" },
  { value: "settings.", label: "تنظیمات" },
  { value: "security.", label: "امنیت" },
  { value: "setup.", label: "نصب" },
  { value: "login.", label: "ورود" },
];

export function ActivityPage() {
  const [action, setAction] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [selected, setSelected] = useState<AuditEntry | null>(null);

  const entries = useQuery({
    queryKey: ["audit", action, from, to],
    queryFn: () =>
      api<{ data: AuditEntry[]; total: number }>("/api/v1/audit", {
        query: { action: action || undefined, from: from || undefined, to: to || undefined, limit: 100 },
      }),
    refetchInterval: 60000,
  });

  const exportCSV = async () => {
    const res = await fetch(
      `${apiBase}/api/v1/audit?format=csv${action ? "&action=" + action : ""}`,
      { headers: { Authorization: "Bearer " + getAccessToken() } }
    );
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "audit.csv";
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const detailPretty = (raw: string): string => {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  };

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <h1 className="text-lg font-bold">گزارش فعالیت</h1>
        <span className="chip bg-slate-100 text-slate-500">{entries.data?.total || 0}</span>
        <div className="ms-auto flex flex-wrap items-center gap-2">
          <Select className="w-40" value={action} onChange={(e) => setAction(e.target.value)}>
            {ACTIONS.map((a) => (
              <option key={a.value} value={a.value}>
                {a.label}
              </option>
            ))}
          </Select>
          <Input className="w-40" type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
          <Input className="w-40" type="date" value={to} onChange={(e) => setTo(e.target.value)} />
          <Button variant="ghost" onClick={exportCSV}>
            CSV
          </Button>
        </div>
      </div>

      <Card className="overflow-hidden p-0">
        {entries.isLoading ? (
          <div className="flex justify-center py-10">
            <Spinner />
          </div>
        ) : (entries.data?.data || []).length === 0 ? (
          <Empty>{t("empty")}</Empty>
        ) : (
          <table className="w-full text-xs">
            <thead className="bg-slate-50 text-slate-500">
              <tr>
                <th className="px-4 py-3 text-start font-medium">زمان</th>
                <th className="px-4 py-3 text-start font-medium">رویداد</th>
                <th className="px-4 py-3 text-start font-medium">کاربر</th>
                <th className="px-4 py-3 text-start font-medium">هدف</th>
                <th className="px-4 py-3 text-start font-medium">IP</th>
              </tr>
            </thead>
            <tbody>
              {(entries.data?.data || []).map((e) => (
                <tr
                  key={e.id}
                  className="cursor-pointer border-t border-slate-50 hover:bg-slate-50"
                  onClick={() => setSelected(e)}
                >
                  <td className="px-4 py-2.5 whitespace-nowrap text-slate-500">{formatDateTime(e.created_at)}</td>
                  <td className="px-4 py-2.5 font-mono text-[11px]" dir="ltr">
                    {e.action}
                  </td>
                  <td className="px-4 py-2.5">{e.actor_name || e.actor_id || e.actor_type}</td>
                  <td className="px-4 py-2.5 text-slate-500" dir="ltr">
                    {(e.target_type || "") + " " + (e.target_id || "")}
                  </td>
                  <td className="px-4 py-2.5 text-slate-400" dir="ltr">
                    {e.ip || "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <Modal open={!!selected} onClose={() => setSelected(null)} title="جزئیات رویداد">
        {selected && (
          <div className="space-y-3 text-xs">
            <pre dir="ltr" className="max-h-72 overflow-auto rounded-xl bg-slate-900 p-3 text-[11px] text-slate-200">
              {detailPretty(selected.detail)}
            </pre>
            <div className="text-[11px] text-slate-400" dir="ltr">
              {selected.user_agent}
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
