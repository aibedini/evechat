import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardTitle, Spinner } from "@/components/ui/card";
import { PageHeader } from "@/components/PageHeader";

const page = copy.pages.integrations;

interface Status {
  eve: { configured: boolean; signature: boolean };
  telegram: { enabled: boolean; username: string };
  gmweb: { configured: boolean };
  ai: { enabled: boolean; model: string };
  smtp?: { configured?: boolean };
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
      setResult(label + ": " + page.runOk);
    } catch (err: any) {
      setResult(label + ": " + (err?.message || t("error")));
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
      title: page.telegramTitle,
      icon: "✈️",
      enabled: !!s?.telegram.enabled,
      description: page.cards.telegram,
      detail: s?.telegram.username ? "@" + s.telegram.username : page.tokenNotSet,
      actions: (
        <>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => run(page.setWebhook, () => api("/api/v1/integrations/telegram/set-webhook", { method: "POST" }))}
          >
            {page.setWebhook}
          </Button>
          <Button
            size="sm"
            onClick={() => run(page.sendTest, () => api("/api/v1/integrations/telegram/test", { method: "POST" }))}
          >
            {page.sendTest}
          </Button>
        </>
      ),
    },
    {
      key: "eve",
      title: "EVE",
      icon: "📱",
      enabled: !!s?.eve.configured,
      description: page.cards.eve,
      detail: s?.eve.signature ? page.signatureRequired : page.signatureOptional,
      actions: (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => run(page.testSearch, () => api("/api/v1/integrations/eve/user", { query: { mobile: "09120000000" } }))}
        >
          {page.testSearch}
        </Button>
      ),
    },
    {
      key: "gmweb",
      title: "GMweb (SMS)",
      icon: "💬",
      enabled: !!s?.gmweb.configured,
      description: page.cards.gmweb,
      detail: s?.gmweb.configured ? page.ready : page.statusNotConfigured,
      actions: (
        <div className="flex items-center gap-2">
          <Input className="h-8 w-36" dir="ltr" placeholder="0912..." value={phone} onChange={(e) => setPhone(e.target.value)} />
          <Button
            size="sm"
            onClick={() => run(page.sendTest, () => api("/api/v1/integrations/gmweb/test", { method: "POST", body: { phone } }))}
            disabled={!phone}
          >
            {page.send}
          </Button>
        </div>
      ),
    },
    {
      key: "ai",
      title: page.aiTitle,
      icon: "🤖",
      enabled: !!s?.ai.enabled,
      description: page.cards.ai,
      detail: s?.ai.enabled ? `${page.aiModelPrefix}${s.ai.model}` : page.aiDisabled,
      actions: null,
    },
    {
      key: "smtp",
      title: page.smtpTitle,
      icon: "✉️",
      enabled: !!s?.smtp?.configured,
      description: page.cards.smtp,
      detail: s?.smtp?.configured ? page.statusConnected : page.statusNotConfigured,
      actions: null,
    },
  ];

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader title={page.title} intro={page.intro} guide={page.guide} />
      {result && <div className="mb-3 rounded-xl bg-slate-100 px-3 py-2 text-xs text-slate-600">{result}</div>}

      <div className="grid gap-3 md:grid-cols-2">
        {integrations.map((i) => (
          <Card key={i.key} data-testid={`integration-${i.key}`} className="flex flex-col">
            <div className="mb-2 flex items-center gap-2">
              <span className="text-xl">{i.icon}</span>
              <span className="text-sm font-semibold">{i.title}</span>
              <span
                className={
                  "chip ms-auto " +
                  (i.enabled ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500")
                }
              >
                {i.enabled ? page.statusConnected : page.statusNotConfigured}
              </span>
            </div>
            <p className="text-xs leading-6 text-slate-500">{i.description}</p>
            <div className="mt-1 text-[11px] text-slate-400">{i.detail}</div>
            {i.actions && <div className="mt-3 flex flex-wrap items-center gap-2">{i.actions}</div>}
          </Card>
        ))}
      </div>

      <Card className="mt-4">
        <CardTitle>{page.helpTitle}</CardTitle>
        <div className="space-y-1 text-xs leading-6 text-slate-500">
          <p>{page.helpEnv}</p>
          <p dir="ltr" className="rounded-lg bg-slate-50 p-2 font-mono text-[10px]">
            EVECHAT_TELEGRAM__BOT_TOKEN, EVECHAT_TELEGRAM__WEBHOOK_URL, EVECHAT_INTEGRATIONS__EVE__API_KEY, EVECHAT_AI__CLAUDE_API_KEY
          </p>
          <p>{page.helpAi}</p>
        </div>
      </Card>
    </div>
  );
}
