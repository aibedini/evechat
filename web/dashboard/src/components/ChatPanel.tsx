import { useEffect, useMemo, useRef } from "react";
import type { Agent, CannedResponse, Conversation, Message } from "@/lib/types";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { cn, formatJalaliLong, formatJalaliTime, initials, statusColor, statusLabel } from "@/lib/utils";
import { Composer } from "./Composer";
import { Button } from "./ui/button";
import { Select } from "./ui/input";
import { Spinner } from "./ui/card";

function dayKey(msg: Message): string {
  const d = new Date(msg.created_at);
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

export function ChatPanel({
  conversation,
  messages,
  agents,
  canned,
  loading,
  typing,
  onSend,
  onAssign,
  onClaim,
  onBack,
  onResolve,
  onReopen,
  onTransfer,
  onRead,
}: {
  conversation?: Conversation;
  messages: Message[];
  agents: Agent[];
  canned: CannedResponse[];
  loading: boolean;
  typing: boolean;
  onSend: (text: string, internal: boolean) => void;
  onAssign: (agentId: string) => void;
  onClaim: () => void;
  onBack?: () => void;
  onResolve: () => void;
  onReopen: () => void;
  onTransfer: (agentId: string, note: string) => void;
  onRead: () => void;
}) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length, conversation?.id]);

  useEffect(() => {
    if (conversation) onRead();
  }, [conversation?.id, messages.length]);

  const groups = useMemo(() => {
    const out: { key: string; label: string; items: Message[] }[] = [];
    for (const msg of messages) {
      const key = dayKey(msg);
      const last = out[out.length - 1];
      if (last && last.key === key) last.items.push(msg);
      else out.push({ key, label: formatJalaliLong(new Date(msg.created_at)), items: [msg] });
    }
    return out;
  }, [messages]);

  if (!conversation) {
    return (
      <div className="inbox-empty flex flex-1 items-center justify-center bg-slate-50 text-sm text-slate-400">
        {t("noVisitorSelected")}
      </div>
    );
  }

  const visitorName = conversation.visitor?.name || conversation.visitor?.phone || copy.pages.inbox.unknownVisitor;
  const isClosed = conversation.status === "resolved" || conversation.status === "closed";

  return (
    <div className="inbox-chat flex min-w-0 flex-1 flex-col bg-slate-50">
      <header className="flex items-center gap-3 border-b border-slate-200 bg-white px-4 py-3">
        {onBack && (
          <button
            type="button"
            onClick={onBack}
            aria-label={copy.pages.inbox.backToList}
            className="rounded-lg border border-slate-200 px-2 py-1 text-sm md:hidden"
          >
            ›
          </button>
        )}
        <span className="grid h-9 w-9 place-items-center rounded-full bg-slate-100 text-xs font-bold text-slate-600">
          {initials(visitorName)}
        </span>
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">{visitorName}</div>
          <div className="flex items-center gap-2 text-[11px] text-slate-400">
            <span className={cn("chip px-1.5 py-0.5 text-[10px]", statusColor(conversation.status))}>
              {statusLabel(conversation.status)}
            </span>
            <span>{conversation.channel}</span>
            {conversation.page_title && <span className="truncate">· {conversation.page_title}</span>}
          </div>
        </div>

        <div className="ms-auto flex items-center gap-2">
          {conversation.status === "pending" && (
            <Button size="sm" onClick={onClaim}>
              {t("claim")}
            </Button>
          )}
          <Select
            className="h-9 w-40 text-xs"
            value={conversation.assigned_agent_id || ""}
            onChange={(e) => onAssign(e.target.value)}
          >
            <option value="">— {t("assign")} —</option>
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name} {a.is_online ? "●" : ""}
              </option>
            ))}
          </Select>
          {isClosed ? (
            <Button variant="ghost" size="sm" onClick={onReopen}>
              {t("reopen")}
            </Button>
          ) : (
            <Button variant="ghost" size="sm" onClick={onResolve}>
              {t("resolve")}
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              const target = agents.find((a) => a.id !== conversation.assigned_agent_id);
              if (target) onTransfer(target.id, "");
            }}
            disabled={agents.length < 2}
          >
            {t("transfer")}
          </Button>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto px-5 py-4">
        {loading && (
          <div className="flex justify-center py-6">
            <Spinner />
          </div>
        )}
        {!loading && messages.length === 0 && (
          <p className="py-6 text-center text-xs text-slate-400">{copy.pages.inbox.noMessages}</p>
        )}
        {groups.map((g) => (
          <div key={g.key}>
            <div className="my-3 flex items-center gap-2">
              <span className="h-px flex-1 bg-slate-200" />
              <span className="text-[10px] text-slate-400">{g.label}</span>
              <span className="h-px flex-1 bg-slate-200" />
            </div>
            {g.items.map((m) => (
              <div
                key={m.id}
                className={cn(
                  "mb-2 flex",
                  m.sender_type === "visitor" ? "justify-start" : "justify-end"
                )}
              >
                <div
                  className={cn(
                    "max-w-[70%] rounded-2xl px-3.5 py-2 text-sm leading-7 shadow-sm",
                    m.sender_type === "visitor" && "bg-white text-slate-800",
                    m.sender_type === "agent" && "bg-brand-600 text-white",
                    m.sender_type === "bot" && "bg-slate-200 text-slate-700",
                    m.sender_type === "system" && "bg-transparent text-xs text-slate-400 shadow-none",
                    m.is_internal && "border border-amber-300 bg-amber-50 text-amber-900"
                  )}
                >
                  {m.is_internal && <div className="mb-1 text-[10px] font-bold text-amber-700">{copy.pages.inbox.internalNoteShort}</div>}
                  <div className="whitespace-pre-wrap break-words">{m.content}</div>
                  <div className={cn("mt-1 flex gap-1 text-[10px]", m.sender_type === "agent" ? "text-white/70" : "text-slate-400")}>
                    <span>{m.sender_name || (m.sender_type === "visitor" ? visitorName : "")}</span>
                    <span>{formatJalaliTime(new Date(m.created_at))}</span>
                    {m.sender_type === "agent" && <span>{m.is_read ? "✓✓" : "✓"}</span>}
                  </div>
                </div>
              </div>
            ))}
          </div>
        ))}
        {typing && <div className="text-xs text-slate-400">{t("typing")}</div>}
        <div ref={bottomRef} />
      </div>

      <Composer onSend={onSend} canned={canned} disabled={conversation.status === "closed"} />
    </div>
  );
}
