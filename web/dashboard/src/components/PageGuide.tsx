import { useState } from "react";
import { copy } from "@/lib/copy.fa";
import { cn } from "@/lib/utils";

export interface PageGuideContent {
  steps: readonly string[];
  note?: string;
}

interface PageGuideProps extends PageGuideContent {
  className?: string;
}

/**
 * A short, always-visible "what do I do here?" guide. On desktop the steps are
 * open; on a phone the header stays visible and the steps expand on tap so the
 * guide never pushes the working area off screen. Technical terms stay in
 * English (curl, webhook, JSONPath, PoW, ...).
 */
export function PageGuide({ steps, note, className }: PageGuideProps) {
  const [open, setOpen] = useState(false);
  return (
    <section className={cn("page-guide", className)} data-testid="page-guide" aria-label={copy.guide.title}>
      <div className="page-guide-head">
        <span className="page-guide-icon" aria-hidden="true">
          🧭
        </span>
        <h2 className="page-guide-title">{copy.guide.title}</h2>
        <span className="page-guide-count">
          {steps.length.toLocaleString("fa-IR")} {copy.guide.stepUnit}
        </span>
        <button
          type="button"
          className="page-guide-toggle"
          aria-expanded={open}
          onClick={() => setOpen((value) => !value)}
        >
          {open ? copy.guide.hide : copy.guide.show}
        </button>
      </div>
      <div className={cn("page-guide-body", !open && "hidden md:block")}>
        <ol className="page-guide-steps">
          {steps.map((step, index) => (
            <li key={index}>
              <span className="page-guide-num" aria-hidden="true">
                {(index + 1).toLocaleString("fa-IR")}
              </span>
              <span>{step}</span>
            </li>
          ))}
        </ol>
        {note ? <p className="page-guide-note">{note}</p> : null}
      </div>
    </section>
  );
}
