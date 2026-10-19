import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Card, CardTitle, Spinner } from "@/components/ui/card";
import { PageHeader } from "@/components/PageHeader";
import { cn } from "@/lib/utils";

const page = copy.pages.health;

interface CheckResult {
  id: string;
  name: string;
  level: number;
  result: { ok: boolean; message: string; fix?: string; detail?: string };
}

interface Report {
  checks: CheckResult[];
  summary: { ok: number; warn: number; fail: number; total: number };
  version: string;
  go_version: string;
  duration: string;
  time: string;
}

export function DiagnosticsPage() {
  const report = useQuery({
    queryKey: ["diagnostics"],
    queryFn: () => api<Report>("/api/v1/diagnostics"),
    staleTime: 0,
  });

  return (
    <div className="flex-1 overflow-y-auto p-6">
      <PageHeader
        title={page.title}
        intro={page.intro}
        guide={page.guide}
        actions={
          <>
            {report.data && (
              <span className="chip bg-slate-100 text-slate-600">
                {report.data.summary.ok} {page.ok} · {report.data.summary.warn} {page.warn} · {report.data.summary.fail} {page.fail}
              </span>
            )}
            <Button variant="ghost" onClick={() => report.refetch()}>
              {page.recheck}
            </Button>
          </>
        }
      />

      {report.isLoading ? (
        <div className="flex justify-center py-10">
          <Spinner />
        </div>
      ) : (
        <Card className="p-0">
          <table className="w-full text-xs">
            <thead className="bg-slate-50 text-slate-500">
              <tr>
                <th className="px-4 py-3 text-start font-medium">{page.check}</th>
                <th className="px-4 py-3 text-start font-medium">{page.status}</th>
                <th className="px-4 py-3 text-start font-medium">{page.message}</th>
                <th className="px-4 py-3 text-start font-medium">{page.fix}</th>
              </tr>
            </thead>
            <tbody>
              {(report.data?.checks || []).map((c) => (
                <tr key={c.id} className="border-t border-slate-50 align-top">
                  <td className="px-4 py-3 font-mono text-[10px] text-slate-500" dir="ltr">
                    {c.id}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={cn(
                        "chip",
                        c.result.ok
                          ? "bg-emerald-50 text-emerald-700"
                          : c.level === 0
                          ? "bg-red-50 text-red-700"
                          : "bg-amber-50 text-amber-700"
                      )}
                    >
                      {c.result.ok ? `✓ ${page.ok}` : c.level === 0 ? `✗ ${page.fail}` : `! ${page.warn}`}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-700">
                    {c.name}: {c.result.message}
                    {c.result.detail && <div className="text-[10px] text-slate-400" dir="ltr">{c.result.detail}</div>}
                  </td>
                  <td className="px-4 py-3 text-[11px] text-slate-500" dir="ltr">
                    {c.result.fix || copy.misc.dash}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}

      <Card className="mt-4">
        <CardTitle>{page.usefulCommands}</CardTitle>
        <pre dir="ltr" className="overflow-x-auto rounded-xl bg-slate-900 p-3 text-[11px] text-slate-200">
{`evechat doctor --verbose
evechat doctor --json | jq '.summary'
evechat doctor --fix`}
        </pre>
        <p className="mt-2 text-[11px] text-slate-400">
          {page.version}: {report.data?.version} · Go {report.data?.go_version} · {report.data?.duration}
        </p>
      </Card>
    </div>
  );
}
