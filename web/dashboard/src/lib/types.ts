export type Role = "superadmin" | "admin" | "agent";

export interface Agent {
  id: string;
  project_id: string;
  email: string;
  name: string;
  role: Role;
  telegram_id?: string | null;
  max_conversations: number;
  is_online: boolean;
  is_active: boolean;
  avatar_url?: string | null;
  created_at: string;
}

export interface Project {
  id: string;
  name: string;
  api_key: string;
  domains: string[];
  settings: Record<string, any>;
  is_active: boolean;
}

export interface Visitor {
  id: string;
  project_id: string;
  name?: string | null;
  email?: string | null;
  phone?: string | null;
  ip?: string | null;
  device_type?: string | null;
  os?: string | null;
  browser?: string | null;
  language?: string | null;
  eve_mobile?: string | null;
  eve_user_id?: string | null;
  eve_reseller_id?: string | null;
  eve_subscription_info?: Record<string, any>;
  metadata?: Record<string, any>;
  is_banned: boolean;
  banned_until?: string | null;
  first_seen_at: string;
  last_seen_at: string;
}

export interface Conversation {
  id: string;
  project_id: string;
  visitor_id: string;
  assigned_agent_id?: string | null;
  status: "pending" | "active" | "resolved" | "closed";
  channel: string;
  subject?: string | null;
  page_url?: string | null;
  page_title?: string | null;
  unread_agent: number;
  unread_visitor: number;
  last_message_at?: string | null;
  first_response_at?: string | null;
  resolved_at?: string | null;
  csat_score?: number | null;
  created_at: string;
  visitor?: Visitor;
  agent?: Agent;
}

export interface Message {
  id: string;
  conversation_id: string;
  client_message_id?: string | null;
  sender_type: "visitor" | "agent" | "bot" | "system";
  sender_id?: string | null;
  sender_name?: string | null;
  content_type: string;
  content: string;
  attachments?: string[];
  is_internal: boolean;
  is_read: boolean;
  created_at: string;
  status?: "pending" | "sent" | "failed";
}

export interface Ticket {
  id: string;
  project_id: string;
  conversation_id?: string | null;
  title: string;
  description?: string | null;
  status: string;
  priority: "low" | "normal" | "high" | "urgent";
  assigned_agent_id?: string | null;
  visitor_id?: string | null;
  tags: string[];
  internal_notes: { id: string; author_id: string; text: string; at: string }[];
  due_at?: string | null;
  resolved_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface KBArticle {
  id: string;
  title: string;
  slug: string;
  content: string;
  excerpt?: string | null;
  tags: string[];
  is_published: boolean;
  view_count: number;
  updated_at: string;
}

export interface Webhook {
  id: string;
  url: string;
  events: string[];
  secret?: string;
  description?: string | null;
  is_active: boolean;
  created_at: string;
}

export interface WebhookDelivery {
  id: string;
  event: string;
  status_code?: number | null;
  attempt: number;
  error?: string | null;
  delivered_at?: string | null;
  created_at: string;
}

export interface RoutingRule {
  id: string;
  name: string;
  priority: number;
  conditions: Record<string, any>;
  route_to: Record<string, any>;
  auto_reply?: string | null;
  is_active: boolean;
}

export interface CannedResponse {
  id: string;
  shortcut: string;
  title: string;
  body: string;
}

export interface ListResponse<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface OverviewStats {
  total_conversations: number;
  pending: number;
  active: number;
  resolved: number;
  closed: number;
  avg_first_response_seconds: number;
  avg_resolution_minutes: number;
  csat_avg: number;
  csat_count: number;
  messages: number;
  unique_visitors: number;
  tickets_open: number;
}

export interface SeriesPoint {
  bucket: string;
  conversations: number;
  messages: number;
  resolved: number;
}

export interface AgentStat {
  agent_id: string;
  name: string;
  conversations: number;
  resolved: number;
  avg_first_response_seconds: number;
  csat_avg: number;
}

export type Envelope = { type: string; id?: string; data?: any; ts?: number };
export interface Channel {
  id: string;
  project_id: string;
  name: string;
  kind: string;
  outbound_url?: string | null;
  mapping: Record<string, unknown>;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ChannelInboxEntry {
  id: string;
  channel_id?: string;
  project_id?: string;
  method: string;
  remote_ip: string;
  status: string;
  error?: string;
  body?: string;
  mapped?: string;
  conversation_id?: string;
  received_at: string;
}
