import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Channel, ChannelInboxEntry, ListResponse } from "@/lib/types";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input, Label, Switch, Textarea, Select } from "@/components/ui/input";
import { Card, Modal, Spinner } from "@/components/ui/card";
import { Callout } from "@/components/Callout";
import { EmptyState } from "@/components/EmptyState";
import { FieldHelp } from "@/components/FieldHelp";
import { PageHeader } from "@/components/PageHeader";
import { cn, formatDateTime } from "@/lib/utils";

const page = copy.pages.channels;

interface HeaderRow {
  key: string;
  value: string;
}

interface ChannelForm {
  id: string;
  name: string;
  kind: string;
  outbound_url: string;
  headers: HeaderRow[];
  session_ttl_seconds: number;
  mapping: string;
  is_active: boolean;
}

interface OutboundTestResult {
  ok: boolean;
  status: number;
  headers: Record<string, string>;
  error?: string;
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
  headers: [],
  session_ttl_seconds: 86400,
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
  const [testResult, setTestResult] = useState<OutboundTestResult | null>(null);

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
        throw new Error(page.invalidMapping);
      }
      const body = {
        name: form.name,
        kind: form.kind,
        outbound_url: form.outbound_url,
        outbound_headers: Object.fromEntries(
          form.headers.map((h) => [h.key.trim(), h.value.trim()]).filter(([key]) => key)
        ),
        session_ttl_seconds: Number(form.session_ttl_seconds) || 0,
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
    onError: (err: any) => setError(err?.message || page.saveError),
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
    mutationFn: (id: string) =>
      api<OutboundTestResult>(`/api/v1/channels/${id}/test`, { method: "POST" }),
    onSuccess: (data) => {
      setTestResult(data);
      setError("");
      qc.invalidateQueries({ queryKey: ["channel-inbox"] });
    },
    onError: (err: any) => setError(err?.message || page.testError),
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
      setPreview(t("error") + ": " + (e?.message || page.invalidBody));
    }
  };

  const showCurl = async (ch: Channel) => {
    const res = await api<{ curl: string }>(`/api/v1/channels/${ch.id}/curl`);
    setCurl(res.curl);
  };

  const items = channels.data?.data || [];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader
        title={page.title}
        intro={page.intro}
        guide={page.guide}
        actions={<Button onClick={() => setEditing({ ...blank })}>+ {page.addChannel}</Button>}
      />

      {credentials && (
        <div className="mb-3 rounded-xl bg-amber-50 p-3 text-xs text-amber-800">
          <div className="mb-1">{page.inboundKey}</div>
          <code dir="ltr" className="break-all">{credentials.secret}</code>
          <div className="mt-2 mb-1">{page.inboundUrl}</div>
          <code dir="ltr" className="break-all">{credentials.url}</code>
          <FieldHelp>{page.secretHelp}</FieldHelp>
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
              {page.copyCurl}
            </Button>
          </div>
        </div>
      )}
      {error && <div className="mb-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}
      {testResult && (
        <div className="mb-3">
          <Callout tone={testResult.ok ? "success" : "warning"}>
            <div>
              {testResult.ok ? page.testOk : page.testFailed}
              {testResult.status ? ` (${testResult.status})` : ""}
            </div>
            {testResult.error && <div dir="ltr" className="mt-1 text-[11px]">{testResult.error}</div>}
            {Object.keys(testResult.headers).length > 0 && (
              <div className="mt-2 text-[11px]">
                {page.testSent}:
                {Object.entries(testResult.headers).map(([key, value]) => (
                  <div key={key} dir="ltr">
                    <code>{key}: {value}</code>
                  </div>
                ))}
              </div>
            )}
          </Callout>
        </div>
      )}

      {channels.isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner />
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          title={page.empty.title}
          body={page.empty.body}
          cta={{ label: page.empty.cta, onClick: () => setEditing({ ...blank }) }}
          icon="🔀"
        />
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
                  {ch.outbound_url || page.noOutbound}
                </div>
              </div>
              <span className={cn("chip", ch.is_active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")}>
                {ch.is_active ? page.active : page.inactive}
              </span>
              <Button size="sm" variant="ghost" onClick={() => showCurl(ch)}>
                curl
              </Button>
              <Button size="sm" variant="ghost" onClick={() => test.mutate(ch.id)}>
                {t("test")}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setInspectFor(ch)}>
                {page.inspector}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => rotate.mutate(ch.id)}>
                {page.rotateSecret}
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
                    headers: Object.entries(ch.outbound_headers || {}).map(([key, value]) => ({ key, value })),
                    session_ttl_seconds: ch.session_ttl_seconds ?? 86400,
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
        title={editing?.id ? t("edit") : page.addChannel}
        footer={
          <>
            <Button variant="ghost" onClick={() => setEditing(null)}>
              {t("cancel")}
            </Button>
            <Button data-testid="channel-save" disabled={!editing?.name.trim() || save.isPending} onClick={() => editing && save.mutate(editing)}>
              {t("save")}
            </Button>
          </>
        }
      >
        {editing && (
          <div className="space-y-3">
            <div>
              <Label>{t("name")}</Label>
              <Input data-testid="channel-name" value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
              <FieldHelp>{page.nameHelp}</FieldHelp>
            </div>
            <div>
              <Label>{page.kind}</Label>
              <Select value={editing.kind} onChange={(e) => setEditing({ ...editing, kind: e.target.value })}>
                <option value="webhook">webhook</option>
                <option value="telegram">telegram</option>
                <option value="sms">sms</option>
                <option value="custom">custom</option>
              </Select>
            </div>
            <div>
              <Label>{page.outboundLabel}</Label>
              <Input
                data-testid="channel-outbound-url"
                dir="ltr"
                value={editing.outbound_url}
                onChange={(e) => setEditing({ ...editing, outbound_url: e.target.value })}
                placeholder="https://example.com/hooks/evechat"
              />
              <FieldHelp>{page.outboundHelp}</FieldHelp>
            </div>
            <div>
              <Label>{page.sessionTTL}</Label>
              <Input
                data-testid="channel-ttl"
                dir="ltr"
                type="number"
                min={0}
                max={2592000}
                value={editing.session_ttl_seconds}
                onChange={(e) => setEditing({ ...editing, session_ttl_seconds: Number(e.target.value) })}
              />
              <FieldHelp>{page.sessionHelp}</FieldHelp>
            </div>
            <div>
              <Label>{page.headersLabel}</Label>
              <div className="space-y-2">
                {editing.headers.map((header, index) => (
                  <div key={index} className="flex gap-2">
                    <Input
                      data-testid={`channel-header-key-${index}`}
                      dir="ltr"
                      placeholder={page.headerKey}
                      value={header.key}
                      onChange={(e) =>
                        setEditing({
                          ...editing,
                          headers: editing.headers.map((h, i) => (i === index ? { ...h, key: e.target.value } : h)),
                        })
                      }
                    />
                    <Input
                      data-testid={`channel-header-value-${index}`}
                      dir="ltr"
                      placeholder={page.headerValue}
                      value={header.value}
                      onChange={(e) =>
                        setEditing({
                          ...editing,
                          headers: editing.headers.map((h, i) => (i === index ? { ...h, value: e.target.value } : h)),
                        })
                      }
                    />
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => setEditing({ ...editing, headers: editing.headers.filter((_, i) => i !== index) })}
                    >
                      {page.removeHeader}
                    </Button>
                  </div>
                ))}
              </div>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="mt-2"
                onClick={() => setEditing({ ...editing, headers: [...editing.headers, { key: "", value: "" }] })}
              >
                + {page.addHeader}
              </Button>
              <FieldHelp>{page.headersHelp}</FieldHelp>
            </div>
            <div>
              <Label>{page.mappingLabel}</Label>
              <Textarea
                data-testid="channel-mapping"
                dir="ltr"
                className="min-h-[140px] font-mono text-xs"
                value={editing.mapping}
                onChange={(e) => setEditing({ ...editing, mapping: e.target.value })}
              />
              <FieldHelp>{page.mappingHelp}</FieldHelp>
            </div>
            <Switch checked={editing.is_active} onChange={(v) => setEditing({ ...editing, is_active: v })} label={page.active} />
          </div>
        )}
      </Modal>

      <Modal open={!!inspectFor} onClose={() => setInspectFor(null)} title={page.inspectorTitle}>
        <div className="space-y-3">
          <div>
            <Label>{page.sampleBody}</Label>
            <Textarea
              dir="ltr"
              className="min-h-[80px] font-mono text-xs"
              value={sample}
              onChange={(e) => setSample(e.target.value)}
            />
            <Button size="sm" className="mt-2" onClick={() => inspectFor && runPreview(inspectFor)}>
              {page.previewMapping}
            </Button>
            <FieldHelp>{page.mappingHelp}</FieldHelp>
            {preview && (
              <pre dir="ltr" className="mt-2 max-h-40 overflow-auto rounded-xl bg-slate-900 p-3 text-[10px] text-slate-200">
                {preview}
              </pre>
            )}
          </div>
          <FieldHelp>{page.trafficHelp}</FieldHelp>
          {inbox.isLoading ? (
            <Spinner />
          ) : (inbox.data?.data || []).length === 0 ? (
            <p className="py-4 text-center text-xs text-slate-400">{page.noRequests}</p>
          ) : (
            <div className="max-h-72 overflow-y-auto">
              <table className="w-full text-[11px]">
                <thead className="text-slate-400">
                  <tr>
                    <th className="py-1 text-start">{page.time}</th>
                    <th className="py-1 text-start">{page.kind}</th>
                    <th className="py-1 text-start">{page.status}</th>
                    <th className="py-1 text-start">{page.details}</th>
                  </tr>
                </thead>
                <tbody>
                  {(inbox.data?.data || []).map((e) => (
                    <tr key={e.id} className="border-t border-slate-50">
                      <td className="py-1.5 text-slate-400">{formatDateTime(e.received_at)}</td>
                      <td className="py-1.5">
                        {e.method === "OUTBOUND TEST" ? page.outbound : page.inbound}
                      </td>
                      <td className="py-1.5">
                        <span
                          className={cn(
                            "chip",
                            e.status === "processed" || e.status === "sent"
                              ? "bg-emerald-50 text-emerald-700"
                              : e.status === "rejected" || e.status === "failed"
                                ? "bg-red-50 text-red-700"
                                : "bg-slate-100 text-slate-500"
                          )}
                        >
                          {e.status}
                        </span>
                      </td>
                      <td className="py-1.5" dir="ltr">
                        {e.method === "OUTBOUND TEST" ? (
                          <details>
                            <summary className="cursor-pointer text-brand-600">{page.details}</summary>
                            <pre className="mt-1 max-h-32 overflow-auto whitespace-pre-wrap break-all text-[10px] text-slate-500">
                              {e.body}
                            </pre>
                          </details>
                        ) : (
                          <span className="text-red-500">{(e.error || copy.misc.dash).slice(0, 80)}</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </Modal>

      <Modal open={!!curl} onClose={() => setCurl("")} title={page.curlTitle}>
        <pre dir="ltr" className="overflow-x-auto rounded-xl bg-slate-900 p-3 text-[11px] text-slate-200">
          {curl}
        </pre>
      </Modal>
    </div>
  );
}
