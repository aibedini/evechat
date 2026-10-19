import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { Card, CardTitle, Spinner } from "@/components/ui/card";
import { FieldHelp } from "@/components/FieldHelp";
import { HelpTip } from "@/components/HelpTip";
import { PageHeader } from "@/components/PageHeader";

const page = copy.pages.widget;

interface SettingsResponse {
  project: { id: string; name: string; api_key: string; domains: string[]; settings: any };
  widget: any;
  embed_code: string;
}

export function WidgetSettingsPage() {
  const [wallboard, setWallboard] = useState<{ token: string; url: string } | null>(null);
  const [wallboardError, setWallboardError] = useState("");
  const [copied, setCopied] = useState(false);
  const rotateWallboard = async () => {
    try {
      const res = await api<{ token: string; url: string }>("/api/v1/settings/project/wallboard-token", { method: "POST" });
      setWallboard(res);
      setWallboardError("");
    } catch (err: any) {
      setWallboardError(err?.message || t("error"));
    }
  };
  const revokeWallboard = async () => {
    await api("/api/v1/settings/project/wallboard-token", { method: "DELETE" });
    setWallboard(null);
  };
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["project-settings"],
    queryFn: () => api<SettingsResponse>("/api/v1/settings/project"),
  });

  const [name, setName] = useState("");
  const [domains, setDomains] = useState("");
  const [primary, setPrimary] = useState("#2563eb");
  const [position, setPosition] = useState("bottom-left");
  const [greeting, setGreeting] = useState("");
  const [offline, setOffline] = useState("");
  const [hours, setHours] = useState({ enabled: false, start: "09:00", end: "18:00", timezone: "Asia/Tehran" });
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!data) return;
    setName(data.project.name);
    setDomains((data.project.domains || []).join(", "));
    const s = data.project.settings || {};
    setPrimary(s.colors?.primary || "#2563eb");
    setPosition(s.position || "bottom-left");
    setGreeting(s.greeting || "");
    setOffline(s.offline_message || "");
    setHours({
      enabled: !!s.business_hours?.enabled,
      start: s.business_hours?.start || "09:00",
      end: s.business_hours?.end || "18:00",
      timezone: s.business_hours?.timezone || "Asia/Tehran",
    });
  }, [data]);

  if (isLoading) {
    return (
      <div className="flex flex-1 justify-center py-16">
        <Spinner />
      </div>
    );
  }

  const save = async () => {
    await api("/api/v1/settings/project", {
      method: "PUT",
      body: {
        name,
        domains: domains.split(",").map((d) => d.trim()).filter(Boolean),
        settings: {
          colors: { primary, launcher: primary },
          position,
          greeting,
          offline_message: offline,
          business_hours: hours,
        },
      },
    });
    setSaved(true);
    setTimeout(() => setSaved(false), 2500);
    refetch();
  };

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader
        title={page.title}
        intro={page.intro}
        guide={page.guide}
        actions={
          <>
            {saved && <span className="chip bg-emerald-50 text-emerald-700">{t("saved")}</span>}
            <Button onClick={save}>{t("save")}</Button>
          </>
        }
      />

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="space-y-4 xl:col-span-2">
          <Card>
            <CardTitle>{page.basic}</CardTitle>
            <div className="grid gap-3 md:grid-cols-2">
              <div>
                <Label>{t("name")}</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div>
                <Label>{t("domains")}</Label>
                <Input dir="ltr" value={domains} onChange={(e) => setDomains(e.target.value)} placeholder="eve.ir, www.eve.ir" />
                <FieldHelp>{page.domainsHelp}</FieldHelp>
              </div>
            </div>
          </Card>

          <Card>
            <CardTitle>{t("widgetColors")}</CardTitle>
            <div className="grid gap-3 md:grid-cols-3">
              <div>
                <Label>
                  <span className="inline-flex items-center gap-1">
                    {page.primaryColor}
                    <HelpTip text={page.colorHelp} />
                  </span>
                </Label>
                <input
                  type="color"
                  className="h-10 w-full cursor-pointer rounded-xl border border-slate-200"
                  value={primary}
                  onChange={(e) => setPrimary(e.target.value)}
                />
              </div>
              <div>
                <Label>
                  <span className="inline-flex items-center gap-1">
                    {t("position")}
                    <HelpTip text={page.positionHelp} />
                  </span>
                </Label>
                <Select value={position} onChange={(e) => setPosition(e.target.value)}>
                  <option value="bottom-left">{page.bottomLeft}</option>
                  <option value="bottom-right">{page.bottomRight}</option>
                </Select>
              </div>
              <div className="flex items-end">
                <div
                  className="grid h-10 w-full place-items-center rounded-xl text-xs font-medium text-white"
                  style={{ background: primary }}
                >
                  {page.buttonPreview}
                </div>
              </div>
            </div>
          </Card>

          <Card>
            <CardTitle>{page.messages}</CardTitle>
            <div className="space-y-3">
              <div>
                <Label>
                  <span className="inline-flex items-center gap-1">
                    {t("greeting")}
                    <HelpTip text={page.greetingHelp} />
                  </span>
                </Label>
                <Textarea value={greeting} onChange={(e) => setGreeting(e.target.value)} />
              </div>
              <div>
                <Label>
                  <span className="inline-flex items-center gap-1">
                    {t("offlineMessage")}
                    <HelpTip text={page.offlineHelp} />
                  </span>
                </Label>
                <Textarea value={offline} onChange={(e) => setOffline(e.target.value)} />
              </div>
            </div>
          </Card>

          <Card>
            <CardTitle
              extra={
                <input
                  type="checkbox"
                  checked={hours.enabled}
                  onChange={(e) => setHours({ ...hours, enabled: e.target.checked })}
                />
              }
            >
              <span className="inline-flex items-center gap-1">
                {t("businessHours")}
                <HelpTip text={page.businessHoursHelp} />
              </span>
            </CardTitle>
            <div className="grid gap-3 md:grid-cols-3">
              <div>
                <Label>{page.start}</Label>
                <Input type="time" value={hours.start} onChange={(e) => setHours({ ...hours, start: e.target.value })} />
              </div>
              <div>
                <Label>{page.end}</Label>
                <Input type="time" value={hours.end} onChange={(e) => setHours({ ...hours, end: e.target.value })} />
              </div>
              <div>
                <Label>{page.timezone}</Label>
                <Input dir="ltr" value={hours.timezone} onChange={(e) => setHours({ ...hours, timezone: e.target.value })} />
              </div>
            </div>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardTitle>{t("embedCode")}</CardTitle>
            <pre dir="ltr" className="overflow-x-auto rounded-xl bg-slate-900 p-3 text-[10px] leading-5 text-slate-200">
              {data?.embed_code}
            </pre>
            <FieldHelp>{page.embedHelp}</FieldHelp>
            <Button
              className="mt-3 w-full"
              variant="ghost"
              onClick={() => {
                navigator.clipboard?.writeText(data?.embed_code || "");
                setCopied(true);
                setTimeout(() => setCopied(false), 2000);
              }}
            >
              {copied ? t("copied") : t("copy")}
            </Button>
          </Card>

          <Card>
            <CardTitle>{page.wallboardTitle}</CardTitle>
            <p className="mb-3 text-xs text-slate-500">{page.wallboardBody}</p>
            {wallboard && (
              <div className="mb-3 space-y-2 rounded-xl bg-slate-100 p-3 text-[11px]">
                <div dir="ltr" className="break-all">{wallboard.url}</div>
                <Button size="sm" variant="ghost" onClick={() => navigator.clipboard?.writeText(wallboard.url)}>
                  {t("copy")}
                </Button>
              </div>
            )}
            {wallboardError && <div className="mb-2 text-xs text-red-600">{wallboardError}</div>}
            <div className="flex flex-wrap gap-2">
              <Button size="sm" onClick={rotateWallboard}>
                {wallboard ? page.newToken : page.createLink}
              </Button>
              <Button size="sm" variant="ghost" onClick={revokeWallboard}>
                {page.revokeLink}
              </Button>
            </div>
          </Card>

          <Card>
            <CardTitle>{page.preview}</CardTitle>
            <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
              <div className="rounded-xl bg-white p-3 shadow-sm">
                <div className="rounded-t-xl p-2 text-xs text-white" style={{ background: primary }}>
                  {name || t("appName")}
                </div>
                <div className="p-3 text-xs text-slate-500">{greeting || page.hello}</div>
              </div>
              <div className="mt-3 flex justify-start">
                <span
                  className="grid h-11 w-11 place-items-center rounded-full text-lg text-white shadow"
                  style={{ background: primary }}
                >
                  💬
                </span>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}
