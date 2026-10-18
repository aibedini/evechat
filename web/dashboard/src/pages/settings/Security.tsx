import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Label, Textarea } from "@/components/ui/input";
import { Card, CardTitle, Spinner } from "@/components/ui/card";

interface Blocklist {
  blocked_ips: string[];
  blocked_cidrs: string[];
  blocked_user_agents: string[];
  pow_difficulty: number;
  active: { ips: number; cidrs: number; user_agents: number };
}

export function SecurityPage() {
  const [ips, setIps] = useState("");
  const [cidrs, setCidrs] = useState("");
  const [uas, setUas] = useState("");
  const [difficulty, setDifficulty] = useState(16);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");

  const blocklist = useQuery({
    queryKey: ["blocklist"],
    queryFn: () => api<Blocklist>("/api/v1/security/blocklist"),
  });

  useEffect(() => {
    if (!blocklist.data) return;
    setIps(blocklist.data.blocked_ips.join("\n"));
    setCidrs(blocklist.data.blocked_cidrs.join("\n"));
    setUas(blocklist.data.blocked_user_agents.join("\n"));
    setDifficulty(blocklist.data.pow_difficulty);
  }, [blocklist.data]);

  const save = async () => {
    setError("");
    try {
      await api("/api/v1/security/blocklist", {
        method: "PUT",
        body: {
          blocked_ips: ips.split("\n").map((x) => x.trim()).filter(Boolean),
          blocked_cidrs: cidrs.split("\n").map((x) => x.trim()).filter(Boolean),
          blocked_user_agents: uas.split("\n").map((x) => x.trim()).filter(Boolean),
          pow_difficulty: difficulty,
        },
      });
      setSaved(true);
      setTimeout(() => setSaved(false), 2500);
      blocklist.refetch();
    } catch (err: any) {
      setError(err?.message || t("error"));
    }
  };

  if (blocklist.isLoading) {
    return (
      <div className="flex flex-1 justify-center py-16">
        <Spinner />
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <div className="mb-4 flex items-center gap-3">
        <h1 className="text-lg font-bold">امنیت و ضد‌اسپم</h1>
        {saved && <span className="chip bg-emerald-50 text-emerald-700">{t("saved")}</span>}
        <Button className="ms-auto" onClick={save}>
          {t("save")}
        </Button>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardTitle extra={<span className="text-[10px] text-slate-400">{blocklist.data?.active.ips} مورد</span>}>
            IP های مسدود
          </CardTitle>
          <Textarea dir="ltr" className="min-h-[140px] font-mono text-xs" value={ips} onChange={(e) => setIps(e.target.value)} />
        </Card>

        <Card>
          <CardTitle extra={<span className="text-[10px] text-slate-400">{blocklist.data?.active.cidrs} مورد</span>}>
            محدوده‌های CIDR
          </CardTitle>
          <Textarea dir="ltr" className="min-h-[140px] font-mono text-xs" value={cidrs} onChange={(e) => setCidrs(e.target.value)} />
        </Card>

        <Card>
          <CardTitle extra={<span className="text-[10px] text-slate-400">{blocklist.data?.active.user_agents} مورد</span>}>
            User-Agent های مسدود
          </CardTitle>
          <Textarea dir="ltr" className="min-h-[140px] font-mono text-xs" value={uas} onChange={(e) => setUas(e.target.value)} />
        </Card>

        <Card>
          <CardTitle>سختی اثبات‌کار (PoW)</CardTitle>
          <Label>بیت‌های صفر پیشرو (۰ = غیرفعال)</Label>
          <input
            type="range"
            min={0}
            max={24}
            value={difficulty}
            onChange={(e) => setDifficulty(Number(e.target.value))}
            className="w-full"
          />
          <div className="text-sm font-semibold">{difficulty} بیت</div>
          <p className="mt-2 text-[11px] leading-6 text-slate-500">
            مقدار ۱۶ حدود ۵۰ میلی‌ثانیه روی موبایل طول می‌کشد. بالاتر = مقاومت بیشتر در برابر ربات‌ها و تأخیر بیشتر برای
            کاربران.
          </p>
        </Card>
      </div>

      {error && <div className="mt-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}
    </div>
  );
}
