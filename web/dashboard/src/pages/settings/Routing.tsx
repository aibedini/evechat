import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { ListResponse, RoutingRule } from "@/lib/types";
import { t } from "@/lib/i18n";
import { useAgents } from "@/hooks/useConversations";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Switch, Textarea } from "@/components/ui/input";
import { Card, Empty, Modal, Spinner } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface RuleForm {
  id: string;
  name: string;
  priority: number;
  is_active: boolean;
  domains: string;
  pages: string;
  devices: string;
  languages: string;
  resellers: string;
  businessHours: string;
  variables: string;
  agent_id: string;
  telegram_id: string;
  phone: string;
  auto_reply: string;
}

const emptyForm: RuleForm = {
  id: "",
  name: "",
  priority: 100,
  is_active: true,
  domains: "",
  pages: "",
  devices: "",
  languages: "",
  resellers: "",
  businessHours: "any",
  variables: "{}",
  agent_id: "",
  telegram_id: "",
  phone: "",
  auto_reply: "",
};

function toForm(rule: RoutingRule): RuleForm {
  const c = rule.conditions || {};
  const r = rule.route_to || {};
  const list = (v: any) => (Array.isArray(v) ? v.join(", ") : v ? String(v) : "");
  return {
    id: rule.id,
    name: rule.name,
    priority: rule.priority,
    is_active: rule.is_active,
    domains: list(c.domains),
    pages: list(c.pages),
    devices: list(c.device_types),
    languages: list(c.languages),
    resellers: list(c.eve_reseller_id),
    businessHours: c.business_hours === undefined ? "any" : String(c.business_hours),
    variables: JSON.stringify(c.variables || {}, null, 2),
    agent_id: r.agent_id || "",
    telegram_id: r.telegram_id || "",
    phone: r.phone || "",
    auto_reply: rule.auto_reply || "",
  };
}

function fromForm(form: RuleForm) {
  const csv = (s: string) => s.split(",").map((x) => x.trim()).filter(Boolean);
  const conditions: any = {};
  if (form.domains) conditions.domains = csv(form.domains);
  if (form.pages) conditions.pages = csv(form.pages);
  if (form.devices) conditions.device_types = csv(form.devices);
  if (form.languages) conditions.languages = csv(form.languages);
  if (form.resellers) conditions.eve_reseller_id = csv(form.resellers);
  if (form.businessHours !== "any") conditions.business_hours = form.businessHours === "true";
  try {
    const vars = JSON.parse(form.variables || "{}");
    if (Object.keys(vars).length) conditions.variables = vars;
  } catch {
    /* invalid JSON is ignored rather than blocking the save */
  }
  const route_to: any = {};
  if (form.agent_id) route_to.agent_id = form.agent_id;
  if (form.telegram_id) route_to.telegram_id = form.telegram_id;
  if (form.phone) route_to.phone = form.phone;
  return {
    name: form.name,
    priority: Number(form.priority) || 0,
    is_active: form.is_active,
    conditions,
    route_to,
    auto_reply: form.auto_reply || null,
  };
}

export function RoutingSettingsPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<RuleForm | null>(null);
  const [testOpen, setTestOpen] = useState(false);
  const [sample, setSample] = useState({ url: "http://eve.ir/subscription", device: "mobile", language: "fa" });
  const [testResult, setTestResult] = useState<any>(null);
  const [dragging, setDragging] = useState<string | null>(null);

  const rules = useQuery({
    queryKey: ["routing-rules"],
    queryFn: () => api<ListResponse<RoutingRule>>("/api/v1/routing-rules"),
  });
  const agents = useAgents();

  const save = useMutation({
    mutationFn: (form: RuleForm) => {
      const body = fromForm(form);
      return form.id
        ? api(`/api/v1/routing-rules/${form.id}`, { method: "PUT", body })
        : api("/api/v1/routing-rules", { method: "POST", body });
    },
    onSuccess: () => {
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["routing-rules"] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => api(`/api/v1/routing-rules/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["routing-rules"] }),
  });

  const reorder = useMutation({
    mutationFn: (order: { id: string; priority: number }[]) =>
      api("/api/v1/routing-rules/reorder", { method: "POST", body: { order } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["routing-rules"] }),
  });

  const items = rules.data?.data || [];

  const moveRule = (fromId: string, toId: string) => {
    const sorted = [...items].sort((a, b) => b.priority - a.priority);
    const from = sorted.findIndex((r) => r.id === fromId);
    const to = sorted.findIndex((r) => r.id === toId);
    if (from < 0 || to < 0 || from === to) return;
    const reordered = [...sorted];
    const [moved] = reordered.splice(from, 1);
    reordered.splice(to, 0, moved);
    const order = reordered.map((r, i) => ({ id: r.id, priority: (reordered.length - i) * 10 }));
    reorder.mutate(order);
  };

  const runTest = async () => {
    try {
      const res = await api("/api/v1/routing-rules/test", {
        method: "POST",
        body: {
          visitor_sample: { device_type: sample.device, language: sample.language, metadata: {}, subscription: {} },
          page: { url: sample.url, title: "" },
        },
      });
      setTestResult(res);
    } catch (err: any) {
      setTestResult({ error: err?.message });
    }
  };

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-lg font-bold">{t("routing")}</h1>
        <Button className="ms-auto" variant="ghost" onClick={() => setTestOpen(true)}>
          تست قانون
        </Button>
        <Button onClick={() => setEditing({ ...emptyForm })}>+ قانون جدید</Button>
      </div>

      <Card className="p-0">
        {rules.isLoading ? (
          <div className="flex justify-center py-8">
            <Spinner />
          </div>
        ) : items.length === 0 ? (
          <Empty>قانونی تعریف نشده است</Empty>
        ) : (
          <div>
            {items.map((rule) => (
              <div
                key={rule.id}
                draggable
                onDragStart={() => setDragging(rule.id)}
                onDragOver={(e) => e.preventDefault()}
                onDrop={() => {
                  if (dragging) moveRule(dragging, rule.id);
                  setDragging(null);
                }}
                className={cn(
                  "flex items-center gap-3 border-b border-slate-100 px-4 py-3 last:border-0",
                  dragging === rule.id && "bg-slate-50 opacity-60"
                )}
              >
                <span className="cursor-grab text-slate-300">⠿</span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{rule.name}</span>
                    <span className={cn("chip", rule.is_active ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")}>
                      {rule.is_active ? "فعال" : "غیرفعال"}
                    </span>
                    <span className="chip bg-slate-100 text-slate-500">اولویت {rule.priority}</span>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-1 text-[10px] text-slate-400">
                    {Object.keys(rule.conditions || {}).length === 0 && <span>بدون شرط (همیشه)</span>}
                    {Object.entries(rule.conditions || {}).map(([k, v]) => (
                      <span key={k} className="rounded bg-slate-50 px-1.5 py-0.5">
                        {k}: {JSON.stringify(v)}
                      </span>
                    ))}
                  </div>
                </div>
                <div className="text-[11px] text-slate-400">
                  {rule.route_to?.agent_id ? "→ کارشناس" : rule.route_to?.telegram_id ? "→ تلگرام" : rule.route_to?.phone ? "→ پیامک" : "→ صف"}
                </div>
                <Button size="sm" variant="ghost" onClick={() => setEditing(toForm(rule))}>
                  {t("edit")}
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  onClick={() => confirm(t("confirmDelete")) && remove.mutate(rule.id)}
                >
                  {t("delete")}
                </Button>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Modal
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing?.id ? t("edit") : "قانون جدید"}
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
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>{t("name")}</Label>
                <Input value={editing.name} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
              </div>
              <div>
                <Label>اولویت</Label>
                <Input
                  type="number"
                  value={editing.priority}
                  onChange={(e) => setEditing({ ...editing, priority: Number(e.target.value) })}
                />
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-3">
              <div className="mb-2 text-xs font-semibold text-slate-600">شرایط (خالی = بدون محدودیت)</div>
              <div className="grid gap-2 md:grid-cols-2">
                <div>
                  <Label>دامنه‌ها</Label>
                  <Input dir="ltr" value={editing.domains} onChange={(e) => setEditing({ ...editing, domains: e.target.value })} placeholder="eve.ir" />
                </div>
                <div>
                  <Label>صفحه‌ها (glob)</Label>
                  <Input dir="ltr" value={editing.pages} onChange={(e) => setEditing({ ...editing, pages: e.target.value })} placeholder="/billing*, */subscription" />
                </div>
                <div>
                  <Label>دستگاه‌ها</Label>
                  <Input dir="ltr" value={editing.devices} onChange={(e) => setEditing({ ...editing, devices: e.target.value })} placeholder="mobile, tablet" />
                </div>
                <div>
                  <Label>زبان‌ها</Label>
                  <Input dir="ltr" value={editing.languages} onChange={(e) => setEditing({ ...editing, languages: e.target.value })} placeholder="fa, en" />
                </div>
                <div>
                  <Label>شناسه نمایندگی</Label>
                  <Input dir="ltr" value={editing.resellers} onChange={(e) => setEditing({ ...editing, resellers: e.target.value })} />
                </div>
                <div>
                  <Label>ساعات کاری</Label>
                  <Select
                    value={editing.businessHours}
                    onChange={(e) => setEditing({ ...editing, businessHours: e.target.value })}
                  >
                    <option value="any">بدون شرط</option>
                    <option value="true">داخل ساعات کاری</option>
                    <option value="false">خارج از ساعات کاری</option>
                  </Select>
                </div>
              </div>
              <div className="mt-2">
                <Label>متغیرها (JSON)</Label>
                <Textarea
                  className="min-h-[70px] font-mono text-xs"
                  dir="ltr"
                  value={editing.variables}
                  onChange={(e) => setEditing({ ...editing, variables: e.target.value })}
                />
              </div>
            </div>

            <div className="rounded-xl bg-slate-50 p-3">
              <div className="mb-2 text-xs font-semibold text-slate-600">مقصد</div>
              <div className="grid gap-2 md:grid-cols-3">
                <div>
                  <Label>کارشناس</Label>
                  <Select value={editing.agent_id} onChange={(e) => setEditing({ ...editing, agent_id: e.target.value })}>
                    <option value="">—</option>
                    {(agents.data?.data || []).map((a: any) => (
                      <option key={a.id} value={a.id}>
                        {a.name}
                      </option>
                    ))}
                  </Select>
                </div>
                <div>
                  <Label>شناسه تلگرام</Label>
                  <Input dir="ltr" value={editing.telegram_id} onChange={(e) => setEditing({ ...editing, telegram_id: e.target.value })} />
                </div>
                <div>
                  <Label>شماره پیامک</Label>
                  <Input dir="ltr" value={editing.phone} onChange={(e) => setEditing({ ...editing, phone: e.target.value })} />
                </div>
              </div>
            </div>

            <div>
              <Label>پاسخ خودکار</Label>
              <Textarea value={editing.auto_reply} onChange={(e) => setEditing({ ...editing, auto_reply: e.target.value })} />
            </div>

            <Switch checked={editing.is_active} onChange={(v) => setEditing({ ...editing, is_active: v })} label="فعال" />
          </div>
        )}
      </Modal>

      <Modal
        open={testOpen}
        onClose={() => setTestOpen(false)}
        title="تست قانون"
        footer={
          <>
            <Button variant="ghost" onClick={() => setTestOpen(false)}>
              بستن
            </Button>
            <Button onClick={runTest}>اجرای تست</Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <Label>آدرس صفحه</Label>
            <Input dir="ltr" value={sample.url} onChange={(e) => setSample({ ...sample, url: e.target.value })} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>دستگاه</Label>
              <Select value={sample.device} onChange={(e) => setSample({ ...sample, device: e.target.value })}>
                <option value="mobile">mobile</option>
                <option value="tablet">tablet</option>
                <option value="desktop">desktop</option>
              </Select>
            </div>
            <div>
              <Label>زبان</Label>
              <Input dir="ltr" value={sample.language} onChange={(e) => setSample({ ...sample, language: e.target.value })} />
            </div>
          </div>
          {testResult && (
            <pre dir="ltr" className="max-h-56 overflow-auto rounded-xl bg-slate-900 p-3 text-[10px] text-slate-200">
              {JSON.stringify(testResult, null, 2)}
            </pre>
          )}
        </div>
      </Modal>
    </div>
  );
}
