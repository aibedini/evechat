import { useId, useState } from "react";
import { t } from "@/lib/i18n";

interface HelpTipProps {
  /** Short explanation shown next to a term or field. */
  text: string;
  label?: string;
}

/** A keyboard-accessible "?" bubble. Click, Enter or Space toggles it. */
export function HelpTip({ text, label }: HelpTipProps) {
  const id = useId();
  const [open, setOpen] = useState(false);
  return (
    <span className="help-tip">
      <button
        type="button"
        className="help-tip-button"
        aria-label={label || t("help")}
        aria-expanded={open}
        aria-describedby={open ? id : undefined}
        onClick={() => setOpen((v) => !v)}
        onBlur={() => setOpen(false)}
      >
        ?
      </button>
      {open ? (
        <span role="tooltip" id={id} className="help-tip-bubble">
          {text}
        </span>
      ) : null}
    </span>
  );
}
