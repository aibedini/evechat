import { useState, type ReactNode } from "react";
import { t } from "@/lib/i18n";
import { PageGuide, type PageGuideContent } from "@/components/PageGuide";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  intro?: string;
  actions?: ReactNode;
  guide?: PageGuideContent;
}

/**
 * Title + intro + primary action, followed by the page's short "what do I do
 * here?" guide. The intro stays a two-line summary on small screens; the guide
 * below it is always visible so a new user is never left without a next step.
 */
export function PageHeader({ title, intro, actions, guide }: PageHeaderProps) {
  const [open, setOpen] = useState(false);
  return (
    <header className="page-header">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="page-title">{title}</h1>
          {intro ? <p className={cn("page-intro", open && "is-open")}>{intro}</p> : null}
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
      {intro ? (
        <button type="button" className="intro-toggle" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
          {open ? t("less") : t("more")}
        </button>
      ) : null}
      {guide ? <PageGuide steps={guide.steps} note={guide.note} /> : null}
    </header>
  );
}
