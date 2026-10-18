import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Card, CardTitle, Spinner } from "@/components/ui/card";

interface Status {
  eve: { configured: boolean; signature: boolean };
  telegram: { enabled: boolean; username: string };
  gmweb: { configured: boolean };
  ai: { enabled: boolean; model: string };
}

export function IntegrationsSettingsPage() {
  const [phone, setPhone] = useState("");
  const [result, setResult] = useState("");

  const status = useQuery({
    queryKey: ["integrations"],
    queryFn: () => api<Status>("/api/v1/integrations/status"),
  });

  const run = async (label: string, fn: () => Promise<any>) => {
    setResult("…");
    try {
      await fn();
      setResult(label + ": موفق");
    } catch (err: any) {
      setResult(label + ": " + (err?.message || "خطا"));
    }
  };

  if (status.isLoading) {
    return (
      <div className="flex flex-1 justify-center py-16">
        <Spinner />
      </div>
    );
  }
  const s = status.data;

  const integrations = [
    {
      key: "telegram",
      title: "تلگرام",
      icon: "✈️",
      enabled: !!s?.telegram.enabled,
      detail: s?.telegram.username ? "@" + s.telegram.username : "توکن تنظیم نشده",
      actions: (
        <>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => run("ثبت وب‌هوک", () => api("/api/v1/integrations/telegram/set-webhook", { method: "POST" }))}
          >
            ثبت وب‌هوک
          </Button>
          <Button
            size="sm"
            onClick={() => run("تست تلگرام", () => api("/api/v1/integrations/telegram/test", { method: "POST" }))}
          >
            ارسال تست
          </Button>
        </>
      ),
    },
    {
      key: "eve",
      title: "EVE",
      icon: "📱",
      enabled: !!s?.eve.configured,
      detail: s?.eve.signature ? "امضای HMAC اجباری" : "امضا اختیاری",
      actions: (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => run("جستجوی EVE", () => api("/api/v1/integrations/eve/user", { query: { mobile: "09120000000" } }))}
        >
          تست جستجو
        </Button>
      ),
    },
    {
      key: "gmweb",
      title: "GMweb (SMS)",
      icon: "💬",
      enabled: !!s?.gmweb.configured,
      detail: s?.gmweb.configured ? "آماده ارسال" : "تنظیم نشده",
      actions: (
        <div className="flex items-center gap-2">
          <Input className="h-8 w-36" dir="ltr" placeholder="0912..." value={phone} onChange={(e) => setPhone(e.target.value)} />
          <Button
            size="sm"
            onClick={() => run("پیامک تست", () => api("/api/v1/integrations/gmweb/test", { method: "POST", body: { phone } }))}
            disabled={!phone}
          >
            ارسال
          </Button>
        </div>
      ),
    },
    {
      key: "ai",
      title: "دستیار هوشمند",
      icon: "🤖",
      enabled: !!s?.ai.enabled,
      detail: s?.ai.enabled ? `مدل: ${s.ai.model}` : "غیرفعال (کلید API تنظیم نشده)",
      actions: null,
    },
  ];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <h1 className="mb-4 text-lg font-bold">{t("integrations")}</h1>
      {result && <div className="mb-3 rounded-xl bg-slate-100 px-3 py-2 text-xs text-slate-600">{result}</div>}

      <div className="grid gap-3 md:grid-cols-2">
        {integrations.map((i) => (
          <Card key={i.key} className="flex flex-col">
            <div className="mb-2 flex items-center gap-2">
              <span className="text-xl">{i.icon}</span>
              <span className="text-sm font-semibold">{i.title}</span>
              <span
                className={
                  "chip ms-auto " +
                  (i.enabled ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")
                }
              >
                {i.enabled ? "فعال" : "غیرفعال"}
              </span>
            </div>
            <div className="text-xs text-slate-500">{i.detail}</div>
            {i.actions && <div className="mt-3 flex items-center gap-2">{i.actions}</div>}
          </Card>
        ))}
      </div>

      <Card className="mt-4">
        <CardTitle>راهنما</CardTitle>
        <div className="space-y-1 text-xs leading-6 text-slate-500">
          <p>تنظیمات یکپارچه‌سازی از طریق متغیرهای محیطی یا فایل YAML انجام می‌شود (کلیدها هرگز در دیتابیس ذخیره نمی‌شوند).</p>
          <p dir="ltr" className="rounded-lg bg-slate-50 p-2 font-mono text-[10px]">
            EVECHAT_TELEGRAM__BOT_TOKEN, EVECHAT_TELEGRAM__WEBHOOK_URL, EVECHAT_INTEGRATIONS__EVE__API_KEY, EVECHAT_AI__CLAUDE_API_KEY
          </p>
          <p>برای فعال‌سازی دستیار هوشمند، کلید Anthropic و ai.enabled=true را تنظیم کنید.</p>
          <Label> </Label>
        </div>
      </Card>
    </div>
  );
}
