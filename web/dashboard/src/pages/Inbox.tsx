import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useAuth } from "@/hooks/useAuth";
import { useAgentEvents } from "@/hooks/useWebSocket";
import {
  useAgents,
  useCannedResponses,
  useConversationAction,
  useConversations,
  useSendMessage,
  type ConversationFilter,
} from "@/hooks/useConversations";
import type { Conversation, Message } from "@/lib/types";
import { t } from "@/lib/i18n";
import { ConversationList, type ListFilters } from "@/components/ConversationList";
import { ChatPanel } from "@/components/ChatPanel";
import { VisitorPanel } from "@/components/VisitorPanel";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Modal } from "@/components/ui/card";

function beep() {
  try {
    const Ctx = (window as any).AudioContext || (window as any).webkitAudioContext;
    if (!Ctx) return;
    const ctx = new Ctx();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.frequency.value = 660;
    gain.gain.setValueAtTime(0.06, ctx.currentTime);
    osc.start();
    osc.stop(ctx.currentTime + 0.18);
  } catch {
    /* audio is a nice-to-have */
  }
}

export function InboxPage() {
  const { session } = useAuth();
  const [filters, setFilters] = useState<ListFilters>({ tab: "pending", q: "" });
  const [activeId, setActiveId] = useState<string | undefined>();
  const [typing, setTyping] = useState(false);
  const [toast, setToast] = useState("");
  const [ticketModal, setTicketModal] = useState(false);
  const [ticketTitle, setTicketTitle] = useState("");
  const [ticketPriority, setTicketPriority] = useState("normal");
  const [muted, setMuted] = useState(false);
  const [grace, setGrace] = useState(false);
  const typingTimer = useRef<any>(0);

  const statusForTab: Record<string, string> = {
    pending: "pending",
    "agent=me": "all",
    all: "all",
    resolved: "resolved",
  };
  const filter: ConversationFilter = {
    status: statusForTab[filters.tab] || "all",
    agent: filters.tab === "agent=me" ? "me" : undefined,
    q: filters.q || undefined,
    limit: 50,
  };

  const list = useConversations(filter);
  const detail = useQuery({
    queryKey: ["conversation", activeId],
    enabled: !!activeId,
    queryFn: () =>
      api<{ conversation: Conversation; messages: Message[]; ticket: any }>(
        `/api/v1/conversations/${activeId}`
      ),
  });
  const agents = useAgents();
  const canned = useCannedResponses();
  const send = useSendMessage();
  const assign = useConversationAction("assign");
  const claim = useConversationAction("claim");
  const resolve = useConversationAction("resolve");
  const reopen = useConversationAction("reopen");
  const transfer = useConversationAction("transfer");

  const conversation = detail.data?.conversation;
  const messages = detail.data?.messages || [];

  const showToast = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(""), 3000);
  }, []);

  const { state: socketState } = useAgentEvents({
    onMessage: (msg) => {
      if (msg.conversation_id !== activeId) {
        if (!muted) beep();
        if ("Notification" in window && Notification.permission === "granted") {
          new Notification("پیام جدید", { body: msg.content.slice(0, 80) });
        }
      }
    },
    onConversation: () => undefined,
    onTyping: (convId) => {
      if (convId === activeId) {
        setTyping(true);
        clearTimeout(typingTimer.current);
        typingTimer.current = setTimeout(() => setTyping(false), 2500);
      }
    },
    onTypingStop: (convId) => {
      if (convId === activeId) setTyping(false);
    },
  });

  useEffect(() => {
    const timer = setTimeout(() => setGrace(true), 5000);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if ("Notification" in window && Notification.permission === "default") {
      const handler = () => Notification.requestPermission().catch(() => undefined);
      window.addEventListener("click", handler, { once: true });
      return () => window.removeEventListener("click", handler);
    }
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        (document.getElementById("conv-search") as HTMLInputElement | null)?.focus();
      }
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === "r" && activeId) {
        e.preventDefault();
        resolve.mutate({ id: activeId });
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [activeId, resolve]);

  const history = useMemo(() => {
    if (!conversation?.visitor_id) return [];
    return (list.data?.data || []).filter((c) => c.visitor_id === conversation.visitor_id && c.id !== conversation.id);
  }, [conversation?.visitor_id, conversation?.id, list.data]);

  const onSend = (text: string, internal: boolean) => {
    if (!activeId) return;
    send.mutate(
      { conversationId: activeId, content: text, internal, clientId: crypto.randomUUID() },
      {
        onError: (err: any) => showToast(err?.message || t("error")),
      }
    );
  };

  return (
    <div className="inbox flex h-full min-h-0 flex-1" data-active={!!activeId}>
      <div className="inbox-list flex flex-col">
        <ConversationList
          items={list.data?.data || []}
          total={list.data?.total || 0}
          activeId={activeId}
          loading={list.isLoading}
          filters={filters}
          onChange={setFilters}
          onSelect={setActiveId}
        />
      </div>

      <ChatPanel
        conversation={conversation}
        messages={messages}
        agents={agents.data?.data || []}
        canned={canned.data?.data || []}
        loading={detail.isLoading}
        typing={typing}
        onSend={onSend}
        onAssign={(agentId) => activeId && assign.mutate({ id: activeId, body: { agent_id: agentId } })}
        onClaim={() => activeId && claim.mutate({ id: activeId })}
        onBack={() => setActiveId(undefined)}
        onResolve={() => activeId && resolve.mutate({ id: activeId })}
        onReopen={() => activeId && reopen.mutate({ id: activeId })}
        onTransfer={(agentId, note) => activeId && transfer.mutate({ id: activeId, body: { agent_id: agentId, note } })}
        onRead={() => {
          if (!activeId) return;
          api(`/api/v1/conversations/${activeId}/read`, { method: "POST", body: {} }).catch(() => undefined);
        }}
      />

      <VisitorPanel
        visitor={conversation?.visitor}
        conversation={conversation}
        history={history}
        onBan={async () => {
          if (!conversation) return;
          await api(`/api/v1/visitors/${conversation.visitor_id}/ban`, {
            method: "POST",
            body: { duration: "24h" },
          }).catch(() => undefined);
          showToast("کاربر مسدود شد");
          detail.refetch();
        }}
        onUnban={async () => {
          if (!conversation) return;
          await api(`/api/v1/visitors/${conversation.visitor_id}/ban`, { method: "DELETE" }).catch(() => undefined);
          showToast("مسدودی برداشته شد");
          detail.refetch();
        }}
        onConvertTicket={() => {
          setTicketTitle(conversation?.subject || conversation?.page_title || "تیکت از گفتگو");
          setTicketModal(true);
        }}
      />

      {(toast || (grace && socketState !== "open")) && (
        <div className="fixed bottom-4 start-1/2 z-40 -translate-x-1/2 rounded-xl bg-slate-900 px-4 py-2 text-xs text-white shadow-lg">
          {toast ? <span>{toast}</span> : <span className="text-amber-300">{t("connectionLost")}</span>}
        </div>
      )}

      <Modal
        open={ticketModal}
        onClose={() => setTicketModal(false)}
        title={t("convertTicket")}
        footer={
          <>
            <Button variant="ghost" onClick={() => setTicketModal(false)}>
              {t("cancel")}
            </Button>
            <Button
              onClick={async () => {
                if (!activeId) return;
                try {
                  await api(`/api/v1/conversations/${activeId}/convert-ticket`, {
                    method: "POST",
                    body: { title: ticketTitle, priority: ticketPriority },
                  });
                  setTicketModal(false);
                  showToast("تیکت created".replace("created", "ایجاد شد"));
                } catch (err: any) {
                  showToast(err?.message || t("error"));
                }
              }}
            >
              {t("create")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <Label>{t("title")}</Label>
            <Input value={ticketTitle} onChange={(e) => setTicketTitle(e.target.value)} />
          </div>
          <div>
            <Label>{t("priority")}</Label>
            <Select value={ticketPriority} onChange={(e) => setTicketPriority(e.target.value)}>
              <option value="low">کم</option>
              <option value="normal">عادی</option>
              <option value="high">بالا</option>
              <option value="urgent">فوری</option>
            </Select>
          </div>
          <div className="text-[11px] text-slate-400">کاربر: {conversation?.visitor?.name || "—"}</div>
        </div>
      </Modal>

      <button
        onClick={() => setMuted(!muted)}
        className="fixed bottom-4 end-4 z-40 rounded-full bg-white px-3 py-2 text-xs shadow-card"
        title={muted ? t("notificationOff") : t("notificationOn")}
      >
        {muted ? "🔇" : "🔔"}
      </button>
      <span className="hidden">{session?.agent?.name}</span>
    </div>
  );
}
