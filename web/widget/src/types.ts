export interface Envelope {
  type: string;
  id?: string;
  data?: any;
  ts?: number;
}

export type TransportState = "connecting" | "open" | "reconnecting" | "degraded" | "offline";

export type Mode = "ws" | "sse" | "poll";

/** Low level channel used by the transport manager. */
export interface Channel {
  readonly mode: Mode;
  open(): void;
  close(): void;
  send(e: Envelope): Promise<void>;
}

/** Callbacks a channel reports back to the manager. */
export interface ChannelHost {
  onOpen(): void;
  onEvent(e: Envelope): void;
  onFail(): void;
}

export interface Message {
  id: string;
  client_id?: string;
  client_message_id?: string;
  conversation_id?: string;
  sender_type: "visitor" | "agent" | "bot" | "system";
  sender_name?: string;
  content: string;
  created_at?: string | number;
  attachments?: string[];
  status?: "pending" | "sent" | "read" | "failed";
  is_internal?: boolean;
}

export interface Conversation {
  id: string;
  status: string;
  assigned_agent_id?: string | null;
}

export interface SessionResponse {
  token: string;
  visitor_id: string;
  expires_at?: string;
  conversation?: Conversation;
  messages?: Message[];
  online?: boolean;
}

export interface WidgetConfig {
  project_id: string;
  name: string;
  locale: string;
  direction: string;
  colors: Record<string, string>;
  position: string;
  greeting: string;
  offline_message: string;
  widget_title?: string;
  online: boolean;
  pre_chat_form?: any;
  business_hours?: any;
  proactive?: ProactiveTrigger[];
}

export interface ProactiveTrigger {
  after_seconds: number;
  page_pattern: string;
  message: string;
}

export interface HostConfig {
  projectKey: string;
  apiUrl: string;
  locale?: string;
  position?: string;
  autoOpen?: boolean;
  visitor?: {
    name?: string;
    email?: string;
    mobile?: string;
    eve_user_id?: string;
    eve_reseller_id?: string;
    eve_mobile?: string;
    metadata?: Record<string, any>;
    subscription?: Record<string, any>;
  };
  colors?: Record<string, string>;
  theme?: "auto" | "light" | "dark";
  font?: "vazirmatn" | "inherit" | "none";
}

export interface OutboxItem {
  client_message_id: string;
  conversation_id: string;
  content: string;
  content_type: string;
  created_at: number;
  attempts: number;
  status: "queued" | "failed";
}
