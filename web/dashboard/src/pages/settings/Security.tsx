import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Label, Textarea } from "@/components/ui/input";
import { Card, CardTitle, Spinner } from "@/components/ui/card";
import { HelpTip } from "@/components/HelpTip";
import { PageHeader } from "@/components/PageHeader";

const page = copy.pages.security;

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

      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardTitle extra={<span className="text-[10px] text-slate-400">{blocklist.data?.active.ips} {page.items}</span>}>
            <span className="inline-flex items-center gap-1">
              {page.blockedIps}
              <HelpTip text={page.blocklistHelp} />
            </span>
          </CardTitle>
          <Textarea dir="ltr" className="min-h-[140px] font-mono text-xs" value={ips} onChange={(e) => setIps(e.target.value)} />
        </Card>

        <Card>
          <CardTitle extra={<span className="text-[10px] text-slate-400">{blocklist.data?.active.cidrs} {page.items}</span>}>
            {page.cidrRanges}
          </CardTitle>
          <Textarea dir="ltr" className="min-h-[140px] font-mono text-xs" value={cidrs} onChange={(e) => setCidrs(e.target.value)} />
        </Card>

        <Card>
          <CardTitle extra={<span className="text-[10px] text-slate-400">{blocklist.data?.active.user_agents} {page.items}</span>}>
            {page.blockedUas}
          </CardTitle>
          <Textarea dir="ltr" className="min-h-[140px] font-mono text-xs" value={uas} onChange={(e) => setUas(e.target.value)} />
        </Card>

        <Card>
          <CardTitle>
            <span className="inline-flex items-center gap-1">
              {page.powTitle}
              <HelpTip text={page.powHelp} />
            </span>
          </CardTitle>
          <Label>{page.powLabel}</Label>
          <input
            type="range"
            min={0}
            max={24}
            value={difficulty}
            onChange={(e) => setDifficulty(Number(e.target.value))}
            className="w-full"
          />
          <div className="text-sm font-semibold">{difficulty} {page.bits}</div>
          <p className="mt-2 text-[11px] leading-6 text-slate-500">{page.powDetail}</p>
        </Card>
      </div>

      {error && <div className="mt-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}
    </div>
  );
}
