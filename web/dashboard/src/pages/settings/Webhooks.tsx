import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { ListResponse, Webhook, WebhookDelivery } from "@/lib/types";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Label, Switch } from "@/components/ui/input";
import { Card, Empty, Modal, Spinner } from "@/components/ui/card";
import { cn, formatDateTime } from "@/lib/utils";

const EVENTS = ["chat.started", "message.received", "chat.assigned", "chat.resolved", "ticket.created", "csat.received"];

interface HookForm {
  id: string;
  url: string;
  events: string[];
  description: string;
  is_active: boolean;
}

const blank: HookForm = { id: "", url: "", events: ["*"], description: "", is_active: true };

export function WebhooksSettingsPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<HookForm | null>(null);
  const [logFor, setLogFor] = useState<Webhook | null>(null);
  const [createdSecret, setCreatedSecret] = useState("");
  const [result, setResult] = useState("");

  const hooks = useQuery({
    queryKey: ["webhooks"],
    queryFn: () => api<ListResponse<Webhook>>("/api/v1/webhooks"),
  });

  const deliveries = useQuery({
    queryKey: ["webhook-deliveries", logFor?.id],
    enabled: !!logFor,
    queryFn: () => api<ListResponse<WebhookDelivery>>(`/api/v1/webhooks/${logFor!.id}/deliveries`),
  });

  const save = useMutation({
    mutationFn: async (form: HookForm) => {
      const body = {
        url: form.url,
        events: form.events,
        description: form.description || undefined,
        is_active: form.is_active,
      };
      if (form.id) return api(`/api/v1/webhooks/${form.id}`, { method: "PUT", body });
      const created = await api<Webhook>("/api/v1/webhooks", { method: "POST", body });
      setCreatedSecret(created.secret || "");
      return created;
    },
    onSuccess: () => {
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["webhooks"] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/webhooks/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["webhooks"] }),
  });

  const test = useMutation({
    mutationFn: (id: string) => api(`/api/v1/webhooks/${id}/test`, { method: "POST" }),
    onSuccess: (data: any) => {
      setResult(data?.ok ? "ارسال آزمایشی موفق بود" : "خطا: " + (data?.error || ""));
      qc.invalidateQueries({ queryKey: ["webhook-deliveries"] });
    },
    onError: (err: any) => setResult(err?.message || "خطا"),
  });

  const items = hooks.data?.data || [];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-lg font-bold">{t("webhooks")}</h1>
        <Button className="ms-auto" onClick={() => setEditing({ ...blank })}>
          + وب‌هوک
        </Button>
      </div>

      {createdSecret && (
        <div className="mb-3 rounded-xl bg-amber-50 p-3 text-xs text-amber-800">
          کلید امضا (فقط یک بار نمایش داده می‌شود): <code dir="ltr">{createdSecret}</code>
        </div>
      )}
      {result && <div className="mb-3 rounded-xl bg-slate-100 px-3 py-2 text-xs text-slate-600">{result}</div>}

      {hooks.isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner />
        </div>
      ) : items.length === 0 ? (
        <Card>
          <Empty>وب‌هوکی ثبت نشده است</Empty>
        </Card>
      ) : (
        <div className="space-y-2">
          {items.map((h) => (
            <Card key={h.id} className="flex items-center gap-3">
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium" dir="ltr">
                  {h.url}
                </div>
                <div className="mt-1 flex flex-wrap gap-1">
                  {(h.events || []).map((e) => (
                    <span key={e} className="chip bg-slate-100 text-[10px] text-slate-500">
                      {e}
                    </span>
                  ))}
                </div>
              </div>
              <span className={cn("chip", h.is_active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")}>
                {h.is_active ? "فعال" : "غیرفعال"}
              </span>
              <Button size="sm" variant="ghost" onClick={() => test.mutate(h.id)}>
                {t("test")}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setLogFor(h)}>
                {t("deliveries")}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() =>
                  setEditing({
                    id: h.id,
                    url: h.url,
                    events: h.events || ["*"],
                    description: h.description || "",
                    is_active: h.is_active,
                  })
                }
              >
                {t("edit")}
              </Button>
              <Button size="sm" variant="danger" onClick={() => confirm(t("confirmDelete")) && remove.mutate(h.id)}>
                {t("delete")}
              </Button>
            </Card>
          ))}
        </div>
      )}

      <Modal
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing?.id ? t("edit") : "وب‌هوک جدید"}
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("cancel")}
            </Button>
            <Button disabled={!editing?.url.trim() || save.isPending} onClick={() => editing && save.mutate(editing)}>
              {t("save")}
            </Button>
          </>
        }
      >
        {editing && (
          <div className="space-y-3">
            <div>
              <Label>{t("webhookUrl")}</Label>
              <Input dir="ltr" value={editing.url} onChange={(e) => setEditing({ ...editing, url: e.target.value })} placeholder="https://example.com/hooks/evechat" />
            </div>
            <div>
              <Label>{t("description")}</Label>
              <Input value={editing.description} onChange={(e) => setEditing({ ...editing, description: e.target.value })} />
            </div>
            <div>
              <Label>{t("events")}</Label>
              <div className="flex flex-wrap gap-2">
                {EVENTS.map((e) => {
                  const on = editing.events.includes(e) || editing.events.includes("*");
                  return (
                    <button
                      key={e}
                      type="button"
                      onClick={() =>
                        setEditing({
                          ...editing,
                          events: on
                            ? editing.events.filter((x) => x !== e && x !== "*")
                            : [...editing.events.filter((x) => x !== "*"), e],
                        })
                      }
                      className={cn(
                        "chip border",
                        on ? "border-brand-200 bg-brand-50 text-brand-700" : "border-slate-200 text-slate-500"
                      )}
                    >
                      {e}
                    </button>
                  );
                })}
              </div>
            </div>
            <Switch checked={editing.is_active} onChange={(v) => setEditing({ ...editing, is_active: v })} label="فعال" />
          </div>
        )}
      </Modal>

      <Modal open={!!logFor} onClose={() => setLogFor(null)} title={t("deliveries")}>
        {deliveries.isLoading ? (
          <Spinner />
        ) : (deliveries.data?.data || []).length === 0 ? (
          <Empty>ارسالی ثبت نشده است</Empty>
        ) : (
          <table className="w-full text-[11px]">
            <thead className="text-slate-400">
              <tr>
                <th className="py-1 text-start">رویداد</th>
                <th className="py-1 text-start">کد</th>
                <th className="py-1 text-start">تلاش</th>
                <th className="py-1 text-start">زمان</th>
                <th className="py-1 text-start">خطا</th>
              </tr>
            </thead>
            <tbody>
              {(deliveries.data?.data || []).map((d) => (
                <tr key={d.id} className="border-t border-slate-50">
                  <td className="py-1.5">{d.event}</td>
                  <td className="py-1.5">{d.status_code || "—"}</td>
                  <td className="py-1.5">{d.attempt}</td>
                  <td className="py-1.5 text-slate-400">{formatDateTime(d.created_at)}</td>
                  <td className="py-1.5 text-red-500">{d.error?.slice(0, 40) || ""}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Modal>
    </div>
  );
}
