import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Channel, ChannelInboxEntry, ListResponse } from "@/lib/types";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Label, Switch, Textarea, Select } from "@/components/ui/input";
import { Card, Empty, Modal, Spinner } from "@/components/ui/card";
import { cn, formatDateTime } from "@/lib/utils";

interface ChannelForm {
  id: string;
  name: string;
  kind: string;
  outbound_url: string;
  mapping: string;
  is_active: boolean;
}

const DEFAULT_MAPPING = JSON.stringify(
  {
    external_id: "$.external_id",
    name: "$.name",
    message: "$.message",
    metadata: "$.metadata",
  },
  null,
  2
);

const blank: ChannelForm = {
  id: "",
  name: "",
  kind: "webhook",
  outbound_url: "",
  mapping: DEFAULT_MAPPING,
  is_active: true,
};

export function ChannelsSettingsPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<ChannelForm | null>(null);
  const [inspectFor, setInspectFor] = useState<Channel | null>(null);
  const [credentials, setCredentials] = useState<{ secret: string; url: string } | null>(null);
  const [curl, setCurl] = useState("");
  const [preview, setPreview] = useState("");
  const [sample, setSample] = useState('{"external_id":"user-42","name":"Test User","message":"hello"}');
  const [error, setError] = useState("");

  const channels = useQuery({
    queryKey: ["channels"],
    queryFn: () => api<ListResponse<Channel>>("/api/v1/channels"),
  });

  const inbox = useQuery({
    queryKey: ["channel-inbox", inspectFor?.id],
    enabled: !!inspectFor,
    queryFn: () => api<ListResponse<ChannelInboxEntry>>(`/api/v1/channels/${inspectFor!.id}/inbox`),
  });

  const save = useMutation({
    mutationFn: async (form: ChannelForm) => {
      let mapping: unknown = {};
      try {
        mapping = JSON.parse(form.mapping || "{}");
      } catch {
        throw new Error("نگاشت باید JSON معتبر باشد");
      }
      const body = {
        name: form.name,
        kind: form.kind,
        outbound_url: form.outbound_url,
        mapping,
        is_active: form.is_active,
      };
      if (form.id) return api(`/api/v1/channels/${form.id}`, { method: "PUT", body });
      const created = await api<Channel & { inbound_secret: string; inbound_url: string }>("/api/v1/channels", {
        method: "POST",
        body,
      });
      setCredentials({ secret: created.inbound_secret, url: created.inbound_url });
      return created;
    },
    onSuccess: () => {
      setEditing(null);
      setError("");
      qc.invalidateQueries({ queryKey: ["channels"] });
    },
    onError: (err: any) => setError(err?.message || "خطا در ذخیره"),
  });

  const rotate = useMutation({
    mutationFn: (id: string) =>
      api<{ inbound_secret: string }>(`/api/v1/channels/${id}/rotate-secret`, { method: "POST" }),
    onSuccess: (data, id) => {
      const ch = (channels.data?.data || []).find((c) => c.id === id);
      setCredentials({
        secret: data.inbound_secret,
        url: ch ? `${location.origin}/api/channels/${ch.id}/inbound` : "",
      });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/channels/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["channels"] }),
  });

  const test = useMutation({
    mutationFn: (id: string) => api(`/api/v1/channels/${id}/test`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["channel-inbox"] }),
    onError: (err: any) => setError(err?.message || "خطا در تست"),
  });

  const runPreview = async (ch: Channel) => {
    try {
      const payload = JSON.parse(sample);
      const res = await api<Record<string, unknown>>(`/api/v1/channels/${ch.id}/preview`, {
        method: "POST",
        body: { payload },
      });
      setPreview(JSON.stringify(res, null, 2));
    } catch (e: any) {
      setPreview("خطا: " + (e?.message || "بدنه نامعتبر"));
    }
  };

  const showCurl = async (ch: Channel) => {
    const res = await api<{ curl: string }>(`/api/v1/channels/${ch.id}/curl`);
    setCurl(res.curl);
  };

  const items = channels.data?.data || [];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-lg font-bold">کانال‌ها</h1>
        <p className="text-xs text-slate-500">راه‌هایی که مشتری با شما حرف می‌زند (پیامک، پیام‌رسان‌ها)</p>
        <Button className="ms-auto" onClick={() => setEditing({ ...blank })}>
          + کانال
        </Button>
      </div>

      {credentials && (
        <div className="mb-3 rounded-xl bg-amber-50 p-3 text-xs text-amber-800">
          <div className="mb-1">کلید ورودی (فقط یک بار نمایش داده می‌شود):</div>
          <code dir="ltr" className="break-all">{credentials.secret}</code>
          <div className="mt-2 mb-1">آدرس ورودی:</div>
          <code dir="ltr" className="break-all">{credentials.url}</code>
          <div className="mt-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={() =>
                navigator.clipboard?.writeText(
                  `curl -X POST ${credentials.url} -H 'Content-Type: application/json' -H 'X-Channel-Secret: ${credentials.secret}' -d '{"external_id":"user-42","name":"Test User","message":"hello"}'`
                )
              }
            >
              کپی دستور curl
            </Button>
          </div>
        </div>
      )}
      {error && <div className="mb-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}

      {channels.isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner />
        </div>
      ) : items.length === 0 ? (
        <Card>
          <Empty>کانالی ثبت نشده است</Empty>
        </Card>
      ) : (
        <div className="space-y-2">
          {items.map((ch) => (
            <Card key={ch.id} className="flex items-center gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium">{ch.name}</span>
                  <span className="chip bg-slate-100 text-[10px] text-slate-500">{ch.kind}</span>
                </div>
                <div className="truncate text-[11px] text-slate-400" dir="ltr">
                  {ch.outbound_url || "بدون آدرس بازگشت"}
                </div>
              </div>
              <span className={cn("chip", ch.is_active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")}>
                {ch.is_active ? "فعال" : "غیرفعال"}
              </span>
              <Button size="sm" variant="ghost" onClick={() => showCurl(ch)}>
                curl
              </Button>
              <Button size="sm" variant="ghost" onClick={() => test.mutate(ch.id)}>
                {t("test")}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setInspectFor(ch)}>
                بازرس
              </Button>
              <Button size="sm" variant="ghost" onClick={() => rotate.mutate(ch.id)}>
                چرخش کلید
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() =>
                  setEditing({
                    id: ch.id,
                    name: ch.name,
                    kind: ch.kind,
                    outbound_url: ch.outbound_url || "",
                    mapping: JSON.stringify(ch.mapping || {}, null, 2),
                    is_active: ch.is_active,
                  })
                }
              >
                {t("edit")}
              </Button>
              <Button size="sm" variant="danger" onClick={() => confirm(t("confirmDelete")) && remove.mutate(ch.id)}>
                {t("delete")}
              </Button>
            </Card>
          ))}
        </div>
      )}

      <Modal
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing?.id ? t("edit") : "کانال جدید"}
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("cancel")}
            </Button>
            <Button disabled={!editing?.name.trim() || save.isPending} onClick={() => editing && save.mutate(editing)}>
              {t("save")}
            </Button>
          </>
        }
      >
        {editing && (
          <div className="space-y-3">
            <div>
              <Label>نام</Label>
              <Input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
            </div>
            <div>
              <Label>نوع</Label>
              <Select value={editing.kind} onChange={(e) => setEditing({ ...editing, kind: e.target.value })}>
                <option value="webhook">webhook</option>
                <option value="telegram">telegram</option>
                <option value="sms">sms</option>
                <option value="custom">custom</option>
              </Select>
            </div>
            <div>
              <Label>آدرس بازگشت (اختیاری)</Label>
              <Input
                dir="ltr"
                value={editing.outbound_url}
                onChange={(e) => setEditing({ ...editing, outbound_url: e.target.value })}
                placeholder="https://example.com/hooks/evechat"
              />
            </div>
            <div>
              <Label>نگاشت (JSONPath)</Label>
              <Textarea
                dir="ltr"
                className="min-h-[140px] font-mono text-xs"
                value={editing.mapping}
                onChange={(e) => setEditing({ ...editing, mapping: e.target.value })}
              />
            </div>
            <Switch checked={editing.is_active} onChange={(v) => setEditing({ ...editing, is_active: v })} label="فعال" />
          </div>
        )}
      </Modal>

      <Modal open={!!inspectFor} onClose={() => setInspectFor(null)} title="بازرس وبهوک">
        <div className="space-y-3">
          <div>
            <Label>بدنه‌ی نمونه برای پیش‌نمایش نگاشت</Label>
            <Textarea
              dir="ltr"
              className="min-h-[80px] font-mono text-xs"
              value={sample}
              onChange={(e) => setSample(e.target.value)}
            />
            <Button size="sm" className="mt-2" onClick={() => inspectFor && runPreview(inspectFor)}>
              پیش‌نمایش نگاشت
            </Button>
            {preview && (
              <pre dir="ltr" className="mt-2 max-h-40 overflow-auto rounded-xl bg-slate-900 p-3 text-[10px] text-slate-200">
                {preview}
              </pre>
            )}
          </div>
          {inbox.isLoading ? (
            <Spinner />
          ) : (inbox.data?.data || []).length === 0 ? (
            <Empty>درخواستی ثبت نشده است</Empty>
          ) : (
            <div className="max-h-72 overflow-y-auto">
              <table className="w-full text-[11px]">
                <thead className="text-slate-400">
                  <tr>
                    <th className="py-1 text-start">زمان</th>
                    <th className="py-1 text-start">وضعیت</th>
                    <th className="py-1 text-start">IP</th>
                    <th className="py-1 text-start">خطا</th>
                  </tr>
                </thead>
                <tbody>
                  {(inbox.data?.data || []).map((e) => (
                    <tr key={e.id} className="border-t border-slate-50">
                      <td className="py-1.5 text-slate-400">{formatDateTime(e.received_at)}</td>
                      <td className="py-1.5">
                        <span
                          className={cn(
                            "chip",
                            e.status === "processed"
                              ? "bg-emerald-50 text-emerald-700"
                              : e.status === "rejected"
                                ? "bg-red-50 text-red-700"
                                : "bg-slate-100 text-slate-500"
                          )}
                        >
                          {e.status}
                        </span>
                      </td>
                      <td className="py-1.5" dir="ltr">{e.remote_ip}</td>
                      <td className="py-1.5 text-red-500">{(e.error || "").slice(0, 60)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Modal>

      <Modal open={!!curl} onClose={() => setCurl("")} title="دستور curl">
        <pre dir="ltr" className="overflow-x-auto rounded-xl bg-slate-900 p-3 text-[11px] text-slate-200">
          {curl}
        </pre>
      </Modal>
    </div>
  );
}
