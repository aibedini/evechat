import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { agentSocket } from "@/lib/ws";
import type { Envelope, Message } from "@/lib/types";

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
      agentSocket.on("__open", () => setState("open")),
      agentSocket.on("conversation.new", (data: any) => {
        qc.invalidateQueries({ queryKey: ["conversations"] });
        handlersRef.current.onConversation?.(data?.id, data?.status);
      }),
      agentSocket.on("conversation.updated", (data: any) => {
        qc.invalidateQueries({ queryKey: ["conversations"] });
        if (data?.conversation_id) {
          qc.invalidateQueries({ queryKey: ["conversation", data.conversation_id] });
          handlersRef.current.onConversation?.(data.conversation_id, data.status);
        }
      }),
      agentSocket.on("message.received", (msg: Message) => {
        if (!msg) return;
        qc.invalidateQueries({ queryKey: ["conversations"] });
        qc.invalidateQueries({ queryKey: ["messages", msg.conversation_id] });
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
          qc.invalidateQueries({ queryKey: ["conversation", data.conversation_id] });
          handlersRef.current.onConversation?.(data.conversation_id, data.status);
        }
      }),
      agentSocket.on("message.received", (msg: Message) => {
        if (!msg) return;
        qc.invalidateQueries({ queryKey: ["conversations"] });
        qc.invalidateQueries({ queryKey: ["messages", msg.conversation_id] });
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
    const timer = setInterval(() => setState(agentSocket.state), 2000);
    return () => {
      clearInterval(timer);
      unsubs.forEach((u) => u());
    };
  }, [qc]);

  return { state };
}
