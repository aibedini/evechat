import { Outbox, clientMessageID } from "./queue";
import { solvePoWAsync } from "./pow";
import { TransportManager } from "./transport/transport";
import { WsChannel } from "./transport/ws";
import { SseChannel } from "./transport/sse";
import { PollChannel } from "./transport/poll";
import {
  draftKey,
  openKey,
  sessionKey,
  storageGet,
  storageRemove,
  storageSet,
} from "./config";
import type {
  Envelope,
  HostConfig,
  Message,
  OutboxItem,
  SessionResponse,
  TransportState,
  WidgetConfig,
} from "./types";

export interface ChatState {
  ready: boolean;
  preparing: boolean;
  open: boolean;
  connection: TransportState;
  messages: Message[];
  typing: boolean;
  unread: number;
  agentName: string;
  csatPending: boolean;
  error: string;
  conversationId: string;
}

type Listener = (state: ChatState) => void;
type EventName = "ready" | "message" | "open" | "close" | "connection" | "csat";

export class Chat {
  state: ChatState = {
    ready: false,
    preparing: false,
    open: false,
    connection: "offline",
    messages: [],
    typing: false,
    unread: 0,
    agentName: "",
    csatPending: false,
    error: "",
    conversationId: "",
  };

  widgetConfig: WidgetConfig | null = null;
  private listeners: Listener[] = [];
  private eventListeners: Record<string, Array<(data: any) => void>> = {};
  private outbox = new Outbox();
  private transport: TransportManager | null = null;
  private token = "";
  private conversationId = "";
  private api = "";
  private closed = false;
  private lastServerTs = 0;
  private seen: Record<string, true> = {};
  private pendingStatus: Record<string, Message["status"]> = {};
  private flushTimer: any = 0;
  private sessionPromise: Promise<void> | null = null;

  constructor(private cfg: HostConfig) {
    this.api = (cfg.apiUrl || location.origin).replace(/\/+$/, "");
  }

  // ---------- lifecycle ----------

  async init(): Promise<void> {
    try {
      await this.loadConfig();
    } catch {
      this.widgetConfig = null;
    }
    this.state.open =
      this.cfg.autoOpen === true || storageGet(openKey(this.cfg.projectKey)) === "1";
    this.emit();
    // The proof-of-work challenge is deferred until the panel is opened (or
    // the host sends a message), so loading the page never blocks on it.
    if (this.state.open) void this.ensureSession();
  }

  /**
   * Starts the visitor session on demand. The PoW solver yields to the event
   * loop and the whole step has a soft timeout; the page stays interactive.
   */
  async ensureSession(): Promise<void> {
    if (this.state.ready) return;
    if (this.sessionPromise) return this.sessionPromise;
    this.state.preparing = true;
    this.state.error = "";
    this.emit();
    this.sessionPromise = (async () => {
      try {
        await this.startSession();
      } catch (err) {
        this.state.error = "session";
        this.state.preparing = false;
        this.sessionPromise = null;
        this.emit();
        return;
      }
      this.state.ready = true;
      this.state.preparing = false;
      this.state.error = "";
      this.startTransport();
      this.flushOutbox();
      this.emitEvent("ready", null);
      this.emit();
      this.watchVisibility();
      if (this.state.open) {
        this.sync();
        this.markRead();
      }
    })();
    return this.sessionPromise;
  }

  retrySession(): void {
    this.sessionPromise = null;
    this.state.error = "";
    void this.ensureSession();
  }

  private async loadConfig(): Promise<void> {
    const res = await fetch(
      `${this.api}/api/widget/v1/config?project_key=${encodeURIComponent(this.cfg.projectKey)}`,
      { headers: { Accept: "application/json" }, credentials: "omit" }
    );
    if (!res.ok) throw new Error("config " + res.status);
    const cfg = (await res.json()) as WidgetConfig;
    if (this.cfg.colors) cfg.colors = { ...(cfg.colors || {}), ...this.cfg.colors };
    if (this.cfg.position) cfg.position = this.cfg.position;
    if (this.cfg.locale) cfg.locale = this.cfg.locale;
    this.widgetConfig = cfg;
  }

  private async startSession(): Promise<void> {
    const token = storageGet(sessionKey(this.cfg.projectKey)) || "";
    let challenge = "";
    let nonce = "";
    try {
      const qs = "project_key=" + encodeURIComponent(this.cfg.projectKey) + (token ? "&token=" + encodeURIComponent(token) : "");
      const res = await fetch(`${this.api}/api/widget/v1/challenge?${qs}`, { credentials: "omit" });
      if (res.ok) {
        const data = await res.json();
        if (data && data.challenge && data.difficulty > 0) {
          challenge = data.challenge;
          nonce = await solvePoWAsync(challenge, data.difficulty, { chunkMs: 8, timeoutMs: 15_000 });
        }
      }
    } catch {
      /* the server replies pow_required and the caller retries */
    }
    const body = {
      token,
      locale: this.cfg.locale || "fa",
      challenge,
      nonce,
      visitor: this.cfg.visitor || {},
      page: { url: location.href, title: document.title, referrer: document.referrer },
    };
    const res = await fetch(
      `${this.api}/api/widget/v1/session?project_key=${encodeURIComponent(this.cfg.projectKey)}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
        credentials: "omit",
      }
    );
    if (!res.ok) throw new Error("session " + res.status);
    const data = (await res.json()) as SessionResponse;
    this.token = data.token;
    storageSet(sessionKey(this.cfg.projectKey), data.token);
    // A resume without a conversation means the previous thread is resolved or
    // closed: forgetting the id makes the next message start a fresh thread
    // instead of writing into a closed one.
    this.conversationId = data.conversation ? data.conversation.id : "";
    for (const msg of data.messages || []) this.upsertMessage(msg);
  }

  private startTransport(): void {
    const qs = "token=" + encodeURIComponent(this.token);
    this.transport = new TransportManager({
      urls: {
        ws: `${this.api.replace(/^http/, "ws")}/ws/visitor?${qs}&project=${encodeURIComponent(
          this.widgetConfig ? this.widgetConfig.project_id : ""
        )}`,
        sse: `${this.api}/api/widget/v1/events?${qs}`,
        poll: `${this.api}/api/widget/v1/poll?${qs}`,
      },
      factories: {
        ws: (url, host) => new WsChannel(url, host),
        sse: (url, host) => new SseChannel(url, host),
        poll: (url, host) => new PollChannel(url, host),
      },
      onEvent: (e) => this.handleEvent(e),
      onState: (s) => {
        this.state.connection = s;
        this.emitEvent("connection", s);
        this.emit();
      },
      onOpen: () => {
        this.sync();
        this.flushOutbox();
      },
      rest: async (e) => {
        if (e.type !== "message.send") throw new Error("rest transport requires a conversation");
        await this.restSend(e.data.content, e.data.client_message_id);
      },
    });
    this.transport.start();
    if (this.flushTimer) clearInterval(this.flushTimer);
    this.flushTimer = setInterval(() => this.flushOutbox(), 15000);
  }

  private watchVisibility(): void {
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "visible" && this.transport) {
        if (this.transport.state !== "open") {
          this.transport.stop();
          this.transport.start();
        } else {
          this.sync();
        }
      }
    });
  }

  destroy(): void {
    this.closed = true;
    clearInterval(this.flushTimer);
    if (this.transport) this.transport.stop();
  }

  // ---------- public actions ----------

  subscribe(fn: Listener): () => void {
    this.listeners.push(fn);
    fn(this.state);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== fn);
    };
  }

  on(name: EventName, cb: (data: any) => void): void {
    (this.eventListeners[name] = this.eventListeners[name] || []).push(cb);
  }

  emitEvent(name: EventName, data: any): void {
    for (const cb of this.eventListeners[name] || []) {
      try {
        cb(data);
      } catch {
        /* listener errors must not break the widget */
      }
    }
  }

  setOpen(open: boolean): void {
    if (this.state.open === open) return;
    this.state.open = open;
    if (open) this.state.unread = 0;
    storageSet(openKey(this.cfg.projectKey), open ? "1" : "0");
    this.emitEvent(open ? "open" : "close", null);
    this.emit();
    if (open) {
      this.sync();
      this.markRead();
      void this.ensureSession();
    }
  }

  toggle(): void {
    this.setOpen(!this.state.open);
  }

  setDraft(text: string): void {
    storageSet(draftKey(this.cfg.projectKey), text);
  }

  draft(): string {
    return storageGet(draftKey(this.cfg.projectKey)) || "";
  }

  identify(visitor: HostConfig["visitor"]): void {
    this.cfg = { ...this.cfg, visitor: { ...(this.cfg.visitor || {}), ...(visitor || {}) } };
    if (this.state.ready) this.startSession().catch(() => undefined);
    else if (this.state.open) void this.ensureSession();
  }

  setVariable(key: string, value: any): void {
    const visitor = this.cfg.visitor || {};
    const metadata = { ...(visitor.metadata || {}), [key]: value };
    this.cfg = { ...this.cfg, visitor: { ...visitor, metadata } };
    if (this.state.ready) this.startSession().catch(() => undefined);
    else if (this.state.open) void this.ensureSession();
  }

  async send(text: string): Promise<void> {
    const content = text.trim();
    if (!content) return;
    if (!this.state.ready) void this.ensureSession();
    const id = clientMessageID();
    const item: OutboxItem = {
      client_message_id: id,
      conversation_id: this.conversationId,
      content,
      content_type: "text",
      created_at: Date.now(),
      attempts: 0,
      status: "queued",
    };
    await this.outbox.put(item);
    this.upsertMessage({
      id,
      client_id: id,
      conversation_id: this.conversationId,
      sender_type: "visitor",
      content,
      created_at: Date.now(),
      status: "pending",
    });
    storageRemove(draftKey(this.cfg.projectKey));
    this.emit();
    await this.deliver(item);
  }

  async retry(clientId: string): Promise<void> {    const items = await this.outbox.all();
    const item = items.find((x) => x.client_message_id === clientId);
    if (!item) return;
    await this.outbox.update(clientId, { status: "queued", attempts: 0 });
    const msg = this.state.messages.find((m) => m.client_id === clientId || m.id === clientId);
    if (msg) {
      msg.status = "pending";
      this.emit();
    }
    await this.deliver({ ...item, attempts: 0, status: "queued" });
  }

  sendTyping(typing: boolean): void {
    if (!this.conversationId || !this.transport) return;
    this.transport
      .send({
        type: typing ? "typing.start" : "typing.stop",
        data: { conversation_id: this.conversationId },
      })
      .catch(() => undefined);
  }

  async submitCSAT(score: number, comment: string): Promise<void> {
    try {
      await fetch(`${this.api}/api/widget/v1/csat?token=${encodeURIComponent(this.token)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ conversation_id: this.conversationId, score, comment }),
        credentials: "omit",
      });
      this.state.csatPending = false;
      this.emitEvent("csat", { score });
      this.emit();
    } catch {
      /* keep the prompt visible for a retry */
    }
  }

  // ---------- delivery ----------

  private async deliver(item: OutboxItem): Promise<void> {
    if (!this.transport) return;
    try {
      if (this.conversationId) {
        await this.transport.send({
          type: "message.send",
          id: item.client_message_id,
          data: {
            conversation_id: this.conversationId,
            content: item.content,
            content_type: item.content_type,
            client_message_id: item.client_message_id,
          },
        });
      } else {
        await this.transport.send({
          type: "conversation.start",
          id: item.client_message_id,
          data: {
            message: item.content,
            client_message_id: item.client_message_id,
            page: { url: location.href, title: document.title, referrer: document.referrer },
          },
        });
      }
      await this.outbox.update(item.client_message_id, { attempts: item.attempts + 1 });
    } catch {
      const attempts = item.attempts + 1;
      if (attempts >= 10) {
        await this.outbox.update(item.client_message_id, { attempts, status: "failed" });
        this.setMessageStatus(item.client_message_id, "failed");
        this.emit();
      } else {
        await this.outbox.update(item.client_message_id, { attempts });
      }
    }
  }

  private async restSend(content: string, clientId: string): Promise<void> {
    if (!this.conversationId) throw new Error("no conversation");
    const res = await fetch(
      `${this.api}/api/widget/v1/conversations/${encodeURIComponent(
        this.conversationId
      )}/messages?token=${encodeURIComponent(this.token)}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content, client_message_id: clientId }),
        credentials: "omit",
      }
    );
    if (!res.ok) throw new Error("send " + res.status);
  }

  private async flushOutbox(): Promise<void> {
    if (this.closed) return;
    const items = await this.outbox.pending();
    for (const item of items) {
      if (this.transport && this.transport.state === "open") {
        await this.deliver(item);
      }
    }
  }

  private sync(): void {
    if (!this.conversationId) return;
    fetch(
      `${this.api}/api/widget/v1/conversations/${encodeURIComponent(
        this.conversationId
      )}/sync?token=${encodeURIComponent(this.token)}&since=${this.lastServerTs}`,
      { headers: { Accept: "application/json" }, credentials: "omit" }
    )
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (!data) return;
        for (const e of data.events || []) this.handleEvent(e);
        for (const m of data.messages || []) this.upsertMessage(m);
        if (data.now) this.lastServerTs = data.now;
        this.emit();
      })
      .catch(() => undefined);
  }

  private markRead(): void {
    if (!this.conversationId || !this.transport) return;
    this.transport
      .send({ type: "read.ack", data: { conversation_id: this.conversationId } })
      .catch(() => undefined);
  }

  // ---------- inbound ----------

  private handleEvent(e: Envelope): void {
    if (e.ts && e.ts > this.lastServerTs) this.lastServerTs = e.ts;
    const data: any = e.data || {};
    switch (e.type) {
      case "conversation.created": {
        if (data.id) this.conversationId = data.id;
        if (data.message) {
          const msg = data.message as Message;
          this.setMessageStatus(msg.client_message_id || "", "sent");
          this.upsertMessage(msg);
        }
        if (!this.state.open) this.state.unread++;
        break;
      }
      case "message.sent": {
        const clientId = data.client_id || e.id || "";
        this.setMessageStatus(clientId, "sent", data.id);
        if (clientId) {
          this.outbox.remove(clientId);
          delete this.pendingStatus[clientId];
        }
        break;
      }
      case "message.received": {
        const msg = data as Message;
        if (msg.sender_type === "visitor") {
          this.setMessageStatus(msg.client_message_id || msg.id, "sent", msg.id);
        }
        this.upsertMessage(msg);
        if (!this.state.open && msg.sender_type !== "visitor") this.state.unread++;
        if (msg.sender_type !== "visitor") {
          this.state.typing = false;
          this.emitEvent("message", msg);
        }
        break;
      }
      case "agent.assigned":
        if (data.agent_name) this.state.agentName = data.agent_name;
        break;
      case "agent.typing":
        this.state.typing = true;
        break;
      case "agent.typing.stop":
        this.state.typing = false;
        break;
      case "conversation.resolved":
        this.state.typing = false;
        // The thread is over; the next message must open a new conversation.
        this.conversationId = "";
        this.upsertMessage({
          id: "system-" + Date.now(),
          sender_type: "system",
          content: "",
          created_at: Date.now(),
        });
        break;
      case "conversation.updated":
        if (data.assigned_agent_id && data.agent_name) this.state.agentName = data.agent_name;
        if (data.status === "resolved" || data.status === "closed") this.conversationId = "";
        break;
      case "csat.request":
        this.state.csatPending = true;
        break;
      case "sync.response": {
        if (data.conversation && data.conversation.id) this.conversationId = data.conversation.id;
        for (const ev of data.events || []) this.handleEvent(ev);
        for (const m of data.messages || []) this.upsertMessage(m);
        if (data.now) this.lastServerTs = Math.max(this.lastServerTs, data.now);
        break;
      }
      case "error":
        this.state.error = data.message || "";
        break;
      default:
        break;
    }
    this.emit();
  }

  private upsertMessage(msg: Message): void {
    if (!msg || !msg.content) return;
    const key = msg.id;
    if (key && this.seen[key]) {
      const existing = this.state.messages.find((m) => m.id === key);
      if (existing && typeof msg.content === "string" && msg.content) existing.content = msg.content;
      return;
    }
    const byClient = msg.client_message_id
      ? this.state.messages.find((m) => m.client_id === msg.client_message_id)
      : undefined;
    if (key) this.seen[key] = true;
    if (byClient) {
      byClient.id = msg.id;
      byClient.content = msg.content || byClient.content;
      byClient.created_at = msg.created_at || byClient.created_at;
      byClient.status = "sent";
      return;
    }
    const sender = msg.sender_type;
    this.state.messages.push({
      ...msg,
      status: sender === "visitor" ? "sent" : undefined,
    });
    this.state.messages.sort((a, b) => {
      const ta = typeof a.created_at === "number" ? a.created_at : Date.parse(String(a.created_at || 0));
      const tb = typeof b.created_at === "number" ? b.created_at : Date.parse(String(b.created_at || 0));
      return ta - tb;
    });
  }

  private setMessageStatus(clientId: string, status: Message["status"], serverId?: string): void {
    if (!clientId) return;
    const msg = this.state.messages.find((m) => m.client_id === clientId || m.id === clientId);
    if (!msg) return;
    msg.status = status;
    if (serverId) msg.id = serverId;
    this.pendingStatus[clientId] = status;
  }

  private emit(): void {
    this.state.conversationId = this.conversationId;
    for (const fn of this.listeners) {
      try {
        fn(this.state);
      } catch {
        /* keep other listeners alive */
      }
    }
  }
}
