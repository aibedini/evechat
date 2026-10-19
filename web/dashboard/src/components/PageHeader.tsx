import { useState, type ReactNode } from "react";
import { t } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  intro?: string;
  actions?: ReactNode;
}

/** Title + intro + primary action, with a collapsible intro on mobile. */
export function PageHeader({ title, intro, actions }: PageHeaderProps) {
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
    </header>
  );
}
