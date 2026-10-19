import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type CalloutTone = "info" | "success" | "warning";

const toneClass: Record<CalloutTone, string> = {
  info: "callout-info",
  success: "callout-success",
  warning: "callout-warning",
};

/** Coloured note/hint box. */
export function Callout({ tone = "info", children }: { tone?: CalloutTone; children: ReactNode }) {
  return (
    <div className={cn("callout", toneClass[tone])} role={tone === "warning" ? "alert" : "note"}>
      {children}
    </div>
  );
}
