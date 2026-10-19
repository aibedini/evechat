import type { ReactNode } from "react";

/** Grey helper text below an input. */
export function FieldHelp({ children }: { children: ReactNode }) {
  return <p className="field-help">{children}</p>;
}
