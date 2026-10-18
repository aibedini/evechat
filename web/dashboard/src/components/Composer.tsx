import { useEffect, useMemo, useRef, useState } from "react";
import type { CannedResponse } from "@/lib/types";
import { t } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export function Composer({
  onSend,
  canned,
  disabled,
}: {
  onSend: (text: string, internal: boolean) => void;
  canned: CannedResponse[];
  disabled?: boolean;
}) {
  const [text, setText] = useState("");
  const [internal, setInternal] = useState(false);
  const [showCanned, setShowCanned] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const ref = useRef<HTMLTextAreaElement>(null);

  const matches = useMemo(() => {
    if (!text.startsWith("/")) return [];
    const q = text.slice(1).toLowerCase();
    return canned.filter((c) => c.shortcut.toLowerCase().includes(q) || c.title.includes(q)).slice(0, 6);
  }, [text, canned]);

  useEffect(() => {
    if (matches.length > 0 && text.startsWith("/")) setShowCanned(true);
    else setShowCanned(false);
  }, [matches.length, text]);

  const send = () => {
    const value = text.trim();
    if (!value) return;
    onSend(value, internal);
    setText("");
    setShowCanned(false);
    if (ref.current) ref.current.style.height = "auto";
  };

  return (
    <div className="relative border-t border-slate-200 bg-white p-3">
      {showCanned && (
        <div className="absolute bottom-full start-3 end-3 mb-1 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-lg">
          {matches.map((c, i) => (
            <button
              key={c.id}
              onMouseEnter={() => setHighlight(i)}
              onClick={() => {
                setText(c.body);
                setShowCanned(false);
                ref.current?.focus();
              }}
              className={cn("block w-full px-3 py-2 text-start text-xs", i === highlight && "bg-slate-50")}
            >
              <span className="font-mono text-brand-600">/{c.shortcut}</span>
              <span className="ms-2 text-slate-600">{c.title}</span>
            </button>
          ))}
        </div>
      )}

      <div className="mb-2 flex items-center gap-2">
        <button
          onClick={() => setInternal(!internal)}
          className={cn(
            "chip border",
            internal ? "border-amber-300 bg-amber-50 text-amber-700" : "border-slate-200 text-slate-500"
          )}
        >
          📝 {t("internalNote")}
        </button>
        <span className="ms-auto text-[10px] text-slate-400">{t("cannedHint")}</span>
      </div>

      <div className="flex items-end gap-2">
        <textarea
          ref={ref}
          className={cn("input min-h-[44px] flex-1 resize-none", internal && "border-amber-300 bg-amber-50/60")}
          rows={1}
          placeholder={internal ? t("internalNote") : t("placeholder")}
          value={text}
          disabled={disabled}
          onChange={(e) => {
            setText(e.target.value);
            e.target.style.height = "auto";
            e.target.style.height = Math.min(e.target.scrollHeight, 140) + "px";
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send();
            }
            if (showCanned && (e.key === "ArrowDown" || e.key === "ArrowUp")) {
              e.preventDefault();
              setHighlight((h) => Math.max(0, Math.min(matches.length - 1, h + (e.key === "ArrowDown" ? 1 : -1))));
            }
            if (showCanned && e.key === "Tab") {
              e.preventDefault();
              const c = matches[highlight];
              if (c) {
                setText(c.body);
                setShowCanned(false);
              }
            }
          }}
        />
        <button className="btn-primary h-11 px-4" onClick={send} disabled={disabled || !text.trim()}>
          {t("send")}
        </button>
      </div>
    </div>
  );
}
