import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Conversation, CannedResponse, ListResponse, Message } from "@/lib/types";

export interface ConversationFilter {
  status?: string;
  agent?: string;
  q?: string;
  limit?: number;
  offset?: number;
}

export function useConversations(filter: ConversationFilter) {
  return useQuery({
    queryKey: ["conversations", filter],
    queryFn: () =>
      api<ListResponse<Conversation>>("/api/v1/conversations", {
        query: {
          status: filter.status,
          agent: filter.agent,
          q: filter.q,
          limit: filter.limit ?? 50,
          offset: filter.offset ?? 0,
        },
      }),
    refetchInterval: 30000,
  });
}

export function useConversation(id?: string) {
  return useQuery({
    queryKey: ["conversation", id],
    enabled: !!id,
    queryFn: () =>
      api<{ conversation: Conversation; messages: Message[]; ticket: any }>(
        `/api/v1/conversations/${id}`
      ),
    refetchInterval: 20000,
  });
}

export function useMessages(id?: string) {
  return useQuery({
    queryKey: ["messages", id],
    enabled: !!id,
    queryFn: () => api<ListResponse<Message>>(`/api/v1/conversations/${id}/messages`, { query: { limit: 100 } }),
    refetchInterval: 20000,
  });
}

export function useSendMessage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { conversationId: string; content: string; internal?: boolean; clientId: string }) =>
      api<{ message: Message }>(
        input.internal
          ? `/api/v1/conversations/${input.conversationId}/note`
          : `/api/v1/conversations/${input.conversationId}/messages`,
        {
          method: "POST",
          body: input.internal
            ? { content: input.content }
            : { content: input.content, client_message_id: input.clientId },
        }
      ),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: ["messages", variables.conversationId] });
      qc.invalidateQueries({ queryKey: ["conversations"] });
      qc.invalidateQueries({ queryKey: ["conversation", variables.conversationId] });
    },
  });
}

export function useConversationAction(action: "assign" | "transfer" | "resolve" | "reopen") {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { id: string; body?: any }) =>
      api(`/api/v1/conversations/${input.id}/${action}`, { method: "POST", body: input.body || {} }),
    onSuccess: (_d, variables) => {
      qc.invalidateQueries({ queryKey: ["conversations"] });
      qc.invalidateQueries({ queryKey: ["conversation", variables.id] });
      qc.invalidateQueries({ queryKey: ["messages", variables.id] });
    },
  });
}

export function useCannedResponses() {
  return useQuery({
    queryKey: ["canned"],
    queryFn: () => api<ListResponse<CannedResponse>>("/api/v1/canned-responses"),
    staleTime: 60000,
  });
}

export function useAgents() {
  return useQuery({
    queryKey: ["agents"],
    queryFn: () => api<ListResponse<any>>("/api/v1/agents"),
    staleTime: 30000,
  });
}
