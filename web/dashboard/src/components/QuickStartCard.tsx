import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "@/lib/api";
import type { Agent, ListResponse } from "@/lib/types";
import { copy } from "@/lib/copy.fa";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const STORAGE_KEY = "evechat:quickstart-dismissed";

interface QuickStartProps {
  uniqueVisitors: number;
  conversations: number;
  messages: number;
}

interface IntegrationStatus {
  telegram?: { enabled?: boolean };
}

/**
 * Four onboarding steps whose ticks come from real data, never from a manual
 * checkbox. The card disappears when all four are done or the user dismisses it.
 */
export function QuickStartCard({ uniqueVisitors, conversations, messages }: QuickStartProps) {
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) === "1";
    } catch {
      return false;
    }
  });
  const agents = useQuery({
    queryKey: ["agents"],
    queryFn: () => api<ListResponse<Agent>>("/api/v1/agents"),
  });
  const integrations = useQuery({
    queryKey: ["integrations"],
    queryFn: () => api<IntegrationStatus>("/api/v1/integrations/status"),
  });

  const activeAgents = (agents.data?.data || []).filter((a) => a.is_active).length;
  const done = [
    uniqueVisitors > 0,
    activeAgents > 1,
    conversations > 0 && messages > 0,
    !!integrations.data?.telegram?.enabled,
  ];
  if (dismissed || done.every(Boolean)) return null;

  const dismiss = () => {
    setDismissed(true);
    try {
      localStorage.setItem(STORAGE_KEY, "1");
    } catch {
      /* private mode: just hide it for this session */
    }
  };

  return (
    <div className="mb-4" data-testid="quick-start">
      <Card>
        <div className="mb-3 flex items-center justify-between gap-2">
          <h2 className="text-sm font-semibold">{copy.quickStart.title}</h2>
          <button type="button" className="text-xs text-slate-400 hover:text-slate-600" onClick={dismiss}>
            {copy.quickStart.dismiss}
          </button>
        </div>
        <ol className="grid gap-2 md:grid-cols-2">
          {copy.quickStart.steps.map((step, index) => (
            <li key={step.id}>
              <Link
                to={step.to}
                className={cn(
                  "flex items-start gap-3 rounded-xl border p-3 transition hover:border-brand-300",
                  done[index] ? "border-emerald-200 bg-emerald-50/60" : "border-slate-200"
                )}
              >
                <span
                  className={cn(
                    "mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full text-[11px] font-bold",
                    done[index] ? "bg-emerald-600 text-white" : "bg-slate-200 text-slate-600"
                  )}
                  aria-hidden="true"
                >
                  {done[index] ? "✓" : index + 1}
                </span>
                <span className="min-w-0">
                  <span className={cn("block text-xs font-medium", done[index] && "text-emerald-800 line-through")}>
                    {step.title}
                  </span>
                  <span className="mt-0.5 block text-[11px] leading-5 text-slate-500">{step.body}</span>
                </span>
              </Link>
            </li>
          ))}
        </ol>
      </Card>
    </div>
  );
}
