import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { Card, CardTitle, Spinner } from "@/components/ui/card";

interface SettingsResponse {
  project: { id: string; name: string; api_key: string; domains: string[]; settings: any };
  widget: any;
  embed_code: string;
}

export function WidgetSettingsPage() {
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
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-lg font-bold">{t("widgetSettings")}</h1>
        {saved && <span className="chip bg-emerald-50 text-emerald-700">{t("saved")}</span>}
        <Button className="ms-auto" onClick={save}>
          {t("save")}
        </Button>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <div className="space-y-4 xl:col-span-2">
          <Card>
            <CardTitle>اطلاعات پایه</CardTitle>
            <div className="grid gap-3 md:grid-cols-2">
              <div>
                <Label>{t("name")}</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div>
                <Label>{t("domains")}</Label>
                <Input dir="ltr" value={domains} onChange={(e) => setDomains(e.target.value)} placeholder="eve.ir, www.eve.ir" />
              </div>
            </div>
          </Card>

          <Card>
            <CardTitle>{t("widgetColors")}</CardTitle>
            <div className="grid gap-3 md:grid-cols-3">
              <div>
                <Label>رنگ اصلی</Label>
                <input
                  type="color"
                  className="h-10 w-full cursor-pointer rounded-xl border border-slate-200"
                  value={primary}
                  onChange={(e) => setPrimary(e.target.value)}
                />
              </div>
              <div>
                <Label>{t("position")}</Label>
                <Select value={position} onChange={(e) => setPosition(e.target.value)}>
                  <option value="bottom-left">پایین چپ</option>
                  <option value="bottom-right">پایین راست</option>
                </Select>
              </div>
              <div className="flex items-end">
                <div
                  className="grid h-10 w-full place-items-center rounded-xl text-xs font-medium text-white"
                  style={{ background: primary }}
                >
                  پیش‌نمایش دکمه
                </div>
              </div>
            </div>
          </Card>

          <Card>
            <CardTitle>پیام‌ها</CardTitle>
            <div className="space-y-3">
              <div>
                <Label>{t("greeting")}</Label>
                <Textarea value={greeting} onChange={(e) => setGreeting(e.target.value)} />
              </div>
              <div>
                <Label>{t("offlineMessage")}</Label>
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
              {t("businessHours")}
            </CardTitle>
            <div className="grid gap-3 md:grid-cols-3">
              <div>
                <Label>شروع</Label>
                <Input type="time" value={hours.start} onChange={(e) => setHours({ ...hours, start: e.target.value })} />
              </div>
              <div>
                <Label>پایان</Label>
                <Input type="time" value={hours.end} onChange={(e) => setHours({ ...hours, end: e.target.value })} />
              </div>
              <div>
                <Label>منطقه زمانی</Label>
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
            <Button
              className="mt-3 w-full"
              variant="ghost"
              onClick={() => navigator.clipboard?.writeText(data?.embed_code || "")}
            >
              {t("copy")}
            </Button>
          </Card>

          <Card>
            <CardTitle>پیش‌نمایش</CardTitle>
            <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
              <div className="rounded-xl bg-white p-3 shadow-sm">
                <div className="rounded-t-xl p-2 text-xs text-white" style={{ background: primary }}>
                  {name || t("appName")}
                </div>
                <div className="p-3 text-xs text-slate-500">{greeting || "سلام 👋"}</div>
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
