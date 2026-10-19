import type { Conversation } from "@/lib/types";
import { cn, initials, statusColor, statusLabel, timeAgo } from "@/lib/utils";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { EmptyState } from "./EmptyState";
import { HelpTip } from "./HelpTip";
import { Spinner } from "./ui/card";

const inbox = copy.pages.inbox;

const tabs = [
  { key: "pending", label: t("pending") },
  { key: "agent=me", label: t("mine") },
  { key: "all", label: t("all") },
  { key: "resolved", label: t("resolved") },
];

export interface ListFilters {
  tab: string;
  q: string;
}

export function ConversationList({
  items,
  total,
  activeId,
  loading,
  filters,
  onChange,
  onSelect,
}: {
  items: Conversation[];
  total: number;
  activeId?: string;
  loading: boolean;
  filters: ListFilters;
  onChange: (f: ListFilters) => void;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="flex h-full w-full shrink-0 flex-col border-e border-slate-200 bg-white sm:w-[290px] lg:w-[330px]">
      <div className="border-b border-slate-100 p-3">
        <input
          id="conv-search"
          className="input"
          placeholder={t("search")}
          value={filters.q}
          onChange={(e) => onChange({ ...filters, q: e.target.value })}
        />
        <div className="mt-2 flex gap-1 overflow-x-auto">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => onChange({ ...filters, tab: tab.key })}
              className={cn(
                "whitespace-nowrap rounded-lg px-2.5 py-1.5 text-xs font-medium transition",
                filters.tab === tab.key ? "bg-brand-50 text-brand-700" : "text-slate-500 hover:bg-slate-50"
              )}
            >
              {tab.label}
            </button>
          ))}
          <span className="ms-auto self-center pe-1 text-[11px] text-slate-400">{total}</span>
          <HelpTip text={inbox.tabsHelp} />
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {loading && (
          <div className="flex justify-center py-8">
            <Spinner />
          </div>
        )}
        {!loading && items.length === 0 && (
          <div className="p-3">
            <EmptyState
              title={inbox.empty.title}
              body={inbox.empty.body}
              cta={{ label: inbox.empty.cta, to: inbox.empty.to }}
              icon="💬"
            />
          </div>
        )}
        {items.map((c) => {
          const visitor = c.visitor;
          const name = visitor?.name || visitor?.phone || inbox.unknownVisitor;
          return (
            <button
              key={c.id}
              onClick={() => onSelect(c.id)}
              className={cn(
                "flex w-full items-start gap-3 border-b border-slate-50 px-3 py-3 text-start transition hover:bg-slate-50",
                activeId === c.id && "bg-brand-50/70 hover:bg-brand-50"
              )}
            >
              <span className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-bold text-slate-600">
                {initials(name)}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2">
                  <span className="truncate text-sm font-medium text-slate-800">{name}</span>
                  <span className="ms-auto whitespace-nowrap text-[10px] text-slate-400">
                    {timeAgo(c.last_message_at || c.created_at)}
                  </span>
                </span>
                <span className="mt-0.5 flex items-center gap-2">
                  <span className={cn("chip px-1.5 py-0.5 text-[10px]", statusColor(c.status))}>
                    {statusLabel(c.status)}
                  </span>
                  {c.subject ? (
                    <span className="truncate text-[11px] text-slate-500">{c.subject}</span>
                  ) : (
                    <span className="truncate text-[11px] text-slate-400">{c.page_title || c.page_url || ""}</span>
                  )}
                  {c.unread_agent > 0 && (
                    <span className="ms-auto grid h-5 min-w-5 place-items-center rounded-full bg-brand-600 px-1 text-[10px] font-bold text-white">
                      {c.unread_agent}
                    </span>
                  )}
                </span>
              </span>
            </button>
          );
        })}
      </div>

      <div className="border-t border-slate-100 px-3 py-2 text-[10px] text-slate-400">
        {t("ctrlK")} · {t("ctrlShiftR")}
      </div>
    </div>
  );
}
