import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Conversation, ListResponse, Visitor } from "@/lib/types";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/input";
import { Card, Modal, Spinner } from "@/components/ui/card";
import { EmptyState } from "@/components/EmptyState";
import { HelpTip } from "@/components/HelpTip";
import { PageHeader } from "@/components/PageHeader";
import { formatDateTime, maskPhone, timeAgo } from "@/lib/utils";

const page = copy.pages.customers;

export function VisitorsPage() {
  const [q, setQ] = useState("");
  const [online, setOnline] = useState("");
  const [device, setDevice] = useState("");
  const [selected, setSelected] = useState<Visitor | null>(null);
  const [pageIndex, setPageIndex] = useState(0);

  const visitors = useQuery({
    queryKey: ["visitors", q, online, device, pageIndex],
    queryFn: () =>
      api<ListResponse<Visitor>>("/api/v1/visitors", {
        query: { q, online: online || undefined, device: device || undefined, limit: 25, offset: pageIndex * 25 },
      }),
    refetchInterval: 30000,
  });

  const history = useQuery({
    queryKey: ["visitor-history", selected?.id],
    enabled: !!selected,
    queryFn: () => api<ListResponse<Conversation>>(`/api/v1/visitors/${selected!.id}/conversations`),
  });

  const items = visitors.data?.data || [];
  const total = visitors.data?.total || 0;

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader
        title={page.title}
        intro={page.intro}
        guide={page.guide}
        actions={<span className="chip bg-slate-100 text-slate-500">{total}</span>}
      />

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <input className="input w-56" placeholder={t("search")} value={q} onChange={(e) => setQ(e.target.value)} />
        <Select className="w-32" value={online} onChange={(e) => setOnline(e.target.value)}>
          <option value="">{page.all}</option>
          <option value="1">{t("online")}</option>
        </Select>
        <Select className="w-36" value={device} onChange={(e) => setDevice(e.target.value)}>
          <option value="">{page.allDevices}</option>
          <option value="mobile">{page.devices.mobile}</option>
          <option value="tablet">{page.devices.tablet}</option>
          <option value="desktop">{page.devices.desktop}</option>
        </Select>
      </div>

      <Card className="overflow-hidden p-0">
        {visitors.isLoading ? (
          <div className="flex justify-center py-10">
            <Spinner />
          </div>
        ) : items.length === 0 ? (
          <EmptyState title={page.empty.title} body={page.empty.body} cta={{ label: page.empty.cta, to: page.empty.to }} icon="👥" />
        ) : (
          <table className="table-cards w-full text-xs">
            <thead className="bg-slate-50 text-slate-500">
              <tr>
                <th className="px-4 py-3 text-start font-medium">{page.columns.customer}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.mobile}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.device}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.reseller}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.lastSeen}</th>
                <th className="px-4 py-3 text-start font-medium">{page.columns.status}</th>
              </tr>
            </thead>
            <tbody>
              {items.map((v) => (
                <tr
                  key={v.id}
                  onClick={() => setSelected(v)}
                  className="cursor-pointer border-t border-slate-50 hover:bg-slate-50"
                >
                  <td className="px-4 py-3">
                    <div className="font-medium text-slate-800">{v.name || page.unknown}</div>
                    <div className="text-[10px] text-slate-400">{v.email || v.id.slice(-8)}</div>
                  </td>
                  <td className="px-4 py-3" dir="ltr" data-label={page.columns.mobile}>{maskPhone(v.phone || v.eve_mobile)}</td>
                  <td className="px-4 py-3" data-label={page.columns.device}>{v.device_type || copy.misc.dash} / {v.os || copy.misc.dash}</td>
                  <td className="px-4 py-3" data-label={page.columns.reseller}>{v.eve_reseller_id || copy.misc.dash}</td>
                  <td className="px-4 py-3 text-slate-500" data-label={page.columns.lastSeen}>{timeAgo(v.last_seen_at)}</td>
                  <td className="px-4 py-3">
                    {v.is_banned ? (
                      <span className="chip bg-red-100 text-red-700">{page.banned}</span>
                    ) : (
                      <span className="chip bg-emerald-50 text-emerald-700">{t("online")}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      <div className="mt-3 flex justify-end gap-2">
        <Button variant="ghost" size="sm" disabled={pageIndex === 0} onClick={() => setPageIndex((p) => p - 1)}>
          {page.previous}
        </Button>
        <Button variant="ghost" size="sm" disabled={(pageIndex + 1) * 25 >= total} onClick={() => setPageIndex((p) => p + 1)}>
          {page.next}
        </Button>
      </div>

      <Modal open={!!selected} onClose={() => setSelected(null)} title={t("visitorInfo")}>
        {selected && (
          <div className="space-y-3 text-xs">
            <div className="grid grid-cols-2 gap-2">
              {[
                [page.fields.name, selected.name],
                [page.fields.email, selected.email],
                [page.fields.mobile, selected.phone || selected.eve_mobile],
                [page.fields.device, `${selected.device_type || copy.misc.dash} / ${selected.os || copy.misc.dash} / ${selected.browser || copy.misc.dash}`],
                [page.fields.reseller, selected.eve_reseller_id],
                [page.fields.eveId, selected.eve_user_id],
                [page.fields.firstSeen, formatDateTime(selected.first_seen_at)],
                [page.fields.lastSeen, formatDateTime(selected.last_seen_at)],
              ].map(([k, v]) => (
                <div key={String(k)} className="rounded-lg bg-slate-50 p-2">
                  <div className="text-[10px] text-slate-400">{k}</div>
                  <div className="truncate text-slate-700">{String(v || copy.misc.dash)}</div>
                </div>
              ))}
            </div>
            <div>
              <div className="mb-1 font-semibold text-slate-600">{t("pastConversations")}</div>
              {history.isLoading ? (
                <Spinner />
              ) : (history.data?.data || []).length === 0 ? (
                <div className="text-slate-400">{page.noHistory}</div>
              ) : (
                (history.data?.data || []).map((c) => (
                  <div key={c.id} className="flex justify-between border-b border-slate-50 py-1.5">
                    <span>{c.subject || c.id.slice(-6)}</span>
                    <span className="text-slate-400">{timeAgo(c.created_at)}</span>
                  </div>
                ))
              )}
            </div>
            <div className="flex flex-wrap items-center gap-2 pt-2">
              <Button
                size="sm"
                variant="ghost"
                onClick={async () => {
                  await api(`/api/v1/visitors/${selected.id}/export`).then((data) => {
                    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
                    const a = document.createElement("a");
                    a.href = URL.createObjectURL(blob);
                    a.download = `visitor-${selected.id}.json`;
                    a.click();
                  });
                }}
              >
                {page.gdprExport}
              </Button>
              <Button
                size="sm"
                variant="danger"
                onClick={async () => {
                  if (!confirm(t("confirmDelete"))) return;
                  await api(`/api/v1/visitors/${selected.id}/anonymize`, { method: "POST", body: {} });
                  setSelected(null);
                  visitors.refetch();
                }}
              >
                {page.deleteData}
              </Button>
              <HelpTip text={page.anonymizeHelp} />
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
