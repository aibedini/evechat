import { useEffect, useRef, useState } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { agentSocket } from "@/lib/ws";
import type { Conversation, Envelope, ListResponse, Message } from "@/lib/types";

export interface RealtimeHandlers {
  onMessage?(msg: Message, conversationId: string): void;
  onTyping?(conversationId: string, agentName: string): void;
  onTypingStop?(conversationId: string): void;
  onConversation?(conversationId: string, status?: string): void;
}

/**
 * Writes an incoming message into every cache that shows it. Realtime events
 * used to invalidate three query keys per frame, which turned a busy inbox into
 * a fetch storm; patching the cache is instant and keeps history in place
 * (BUG-4).
 */
function appendMessage(qc: QueryClient, msg: Message): void {
  if (!msg || !msg.conversation_id) return;
  qc.setQueryData<{ conversation: Conversation; messages: Message[] } | undefined>(
    ["conversation", msg.conversation_id],
    (old) => {
      if (!old || !Array.isArray(old.messages)) return old;
      if (old.messages.some((m) => m.id === msg.id)) return old;
      return { ...old, messages: [...old.messages, msg] };
    }
  );
  qc.setQueryData<ListResponse<Message> | undefined>(["messages", msg.conversation_id], (old) => {
    if (!old || !Array.isArray(old.data)) return old;
    if (old.data.some((m) => m.id === msg.id)) return old;
    return { ...old, data: [...old.data, msg], total: old.total + 1 };
  });
  qc.setQueriesData<ListResponse<Conversation> | undefined>({ queryKey: ["conversations"] }, (old) => {
    if (!old || !Array.isArray(old.data)) return old;
    const index = old.data.findIndex((c) => c.id === msg.conversation_id);
    if (index < 0) return old;
    const next = [...old.data];
    next[index] = {
      ...next[index],
      last_message_at: typeof msg.created_at === "string" ? msg.created_at : next[index].last_message_at,
    };
    return { ...old, data: next };
  });
}

/**
 * Applies a conversation.updated frame to the open panel's cache. Refetching
 * the detail on every unread/status frame caused the BUG-4 fetch storm; the
 * frame already carries the fields the panel renders.
 */
function patchConversation(qc: QueryClient, data: any): void {
  if (!data?.conversation_id) return;
  qc.setQueryData<{ conversation: Conversation; messages: Message[] } | undefined>(
    ["conversation", data.conversation_id],
    (old) => {
      if (!old) return old;
      const conversation = { ...old.conversation };
      if (typeof data.status === "string") conversation.status = data.status;
      if (data.assigned_agent_id !== undefined) conversation.assigned_agent_id = data.assigned_agent_id;
      if (data.last_message_at !== undefined) conversation.last_message_at = data.last_message_at;
      if (typeof data.unread_agent === "number") conversation.unread_agent = data.unread_agent;
      if (typeof data.unread_visitor === "number") conversation.unread_visitor = data.unread_visitor;
      return { ...old, conversation };
    }
  );
}
export interface RealtimeHandlers {
  onMessage?(msg: Message, conversationId: string): void;
  onTyping?(conversationId: string, agentName: string): void;
  onTypingStop?(conversationId: string): void;
  onConversation?(conversationId: string, status?: string): void;
}

/** Connects the dashboard socket and keeps the query cache fresh. */
export function useWebSocket(handlers: RealtimeHandlers = {}) {
  const qc = useQueryClient();
  const [state, setState] = useState(agentSocket.state);
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  useEffect(() => {
    const unsubs: Array<() => void> = [];
    unsubs.push(
      agentSocket.on("__open", () => {
        setState("open");
        // The socket has no replay buffer: refresh once on (re)connect so a
        // frame sent during the handshake is not lost until the next poll.
        qc.invalidateQueries({ queryKey: ["conversations"] });
        qc.invalidateQueries({ queryKey: ["conversation"] });
        qc.invalidateQueries({ queryKey: ["messages"] });
      }),
      agentSocket.on("conversation.new", (data: any) => {
        qc.invalidateQueries({ queryKey: ["conversations"] });
        handlersRef.current.onConversation?.(data?.id, data?.status);
      }),
      agentSocket.on("conversation.updated", (data: any) => {
        qc.invalidateQueries({ queryKey: ["conversations"] });
        if (data?.conversation_id) {
          patchConversation(qc, data);
          handlersRef.current.onConversation?.(data.conversation_id, data.status);
        }
      }),
      agentSocket.on("message.received", (msg: Message) => {
        if (!msg) return;
        appendMessage(qc, msg);
        if (msg.sender_type === "visitor") {
          handlersRef.current.onMessage?.(msg, msg.conversation_id);
        }
      }),
      agentSocket.on("visitor.typing", (data: any) => {
        if (data?.conversation_id) handlersRef.current.onTyping?.(data.conversation_id, "");
      }),
      agentSocket.on("visitor.typing.stop", (data: any) => {
        if (data?.conversation_id) handlersRef.current.onTypingStop?.(data.conversation_id);
      }),
      agentSocket.on("presence.update", () => {
        qc.invalidateQueries({ queryKey: ["agents"] });
      })
    );

    const timer = setInterval(() => setState(agentSocket.state), 3000);
    agentSocket.restart();

    return () => {
      clearInterval(timer);
      unsubs.forEach((u) => u());
      agentSocket.disconnect();
    };
  }, [qc]);

  const send = (e: Envelope) => agentSocket.send(e);
  return { state, send };
}

/**
 * Registers realtime handlers on the socket owned by the layout. Use this in
 * pages so a page mount never restarts the shared connection.
 */
export function useAgentEvents(handlers: RealtimeHandlers = {}) {
  const qc = useQueryClient();
  const [state, setState] = useState(agentSocket.state);
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  useEffect(() => {
    const unsubs: Array<() => void> = [];
    unsubs.push(
      agentSocket.on("conversation.new", (data: any) => {
        qc.invalidateQueries({ queryKey: ["conversations"] });
        handlersRef.current.onConversation?.(data?.id, data?.status);
      }),
      agentSocket.on("conversation.updated", (data: any) => {
        qc.invalidateQueries({ queryKey: ["conversations"] });
        if (data?.conversation_id) {
          patchConversation(qc, data);
          handlersRef.current.onConversation?.(data.conversation_id, data.status);
        }
      }),
      agentSocket.on("message.received", (msg: Message) => {
        if (!msg) return;
        appendMessage(qc, msg);
        if (msg.sender_type === "visitor") {
          handlersRef.current.onMessage?.(msg, msg.conversation_id);
        }
      }),
      agentSocket.on("visitor.typing", (data: any) => {
        if (data?.conversation_id) handlersRef.current.onTyping?.(data.conversation_id, "");
      }),
      agentSocket.on("visitor.typing.stop", (data: any) => {
        if (data?.conversation_id) handlersRef.current.onTypingStop?.(data.conversation_id);
      })
    );
    unsubs.push(
      agentSocket.on("__open", () => {
        setState("open");
        qc.invalidateQueries({ queryKey: ["conversations"] });
        qc.invalidateQueries({ queryKey: ["conversation"] });
        qc.invalidateQueries({ queryKey: ["messages"] });
      })
    );
    const timer = setInterval(() => setState(agentSocket.state), 2000);
    return () => {
      clearInterval(timer);
      unsubs.forEach((u) => u());
    };
  }, [qc]);

  return { state };
}