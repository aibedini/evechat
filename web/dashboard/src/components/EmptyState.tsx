import type { ReactNode } from "react";
import { Link } from "react-router-dom";

export interface EmptyStateCta {
  label: string;
  to?: string;
  onClick?: () => void;
}

interface EmptyStateProps {
  title: string;
  body: string;
  /** Required by design: an empty state always tells the user what to do next. */
  cta: EmptyStateCta;
  icon?: ReactNode;
}

export function EmptyState({ title, body, cta, icon }: EmptyStateProps) {
  const button = cta.to ? (
    <Link to={cta.to} className="btn-primary">
      {cta.label}
    </Link>
  ) : (
    <button type="button" className="btn-primary" onClick={cta.onClick}>
      {cta.label}
    </button>
  );
  return (
    <div className="empty-state" data-testid="empty-state">
      {icon ? <div className="text-3xl" aria-hidden="true">{icon}</div> : null}
      <h2 className="empty-title">{title}</h2>
      <p className="empty-body">{body}</p>
      <div className="mt-2">{button}</div>
    </div>
  );
}
