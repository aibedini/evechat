import { getAccessToken } from "./api";
import type { Envelope } from "./types";

type Handler = (data: any) => void;

const BACKOFF = [1000, 2000, 4000, 8000, 15000, 30000];

/**
 * Reconnecting dashboard socket with the same backoff policy as the widget.
 * On every reconnect the hooks refetch the inbox and the open thread.
 */
export class AgentSocket {
  private ws: WebSocket | null = null;
  private attempt = 0;
  private closed = false;
  private timer: any = 0;
  private handlers: Record<string, Handler[]> = {};
  private openHandlers: Array<() => void> = [];
  state: "offline" | "connecting" | "open" | "reconnecting" = "offline";

  connect(): void {
    if (this.closed) return;
    const token = getAccessToken();
    if (!token) return;
    const proto = location.protocol === "https:" ? "wss" : "ws";
    this.state = this.attempt === 0 ? "connecting" : "reconnecting";
    let ws: WebSocket;
    try {
      ws = new WebSocket(`${proto}://${location.host}/ws/agent?token=${encodeURIComponent(token)}`);
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      this.attempt = 0;
      this.state = "open";
      for (const h of this.openHandlers) h();
      this.emitLocal("__open", null);
    };
    ws.onmessage = (ev) => {
      let env: Envelope;
      try {
        env = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      if (env.type === "ping") {
        this.send({ type: "pong" });
        return;
      }
      if (env.type === "pong") return;
      this.emitLocal(env.type, env.data);
      this.emitLocal("*", env);
    };
    ws.onclose = () => {
      if (this.closed) return;
      this.state = "offline";
      this.scheduleReconnect();
    };
    ws.onerror = () => {
      /* onclose follows */
    };
  }

  private scheduleReconnect(): void {
    const delay = BACKOFF[Math.min(this.attempt, BACKOFF.length - 1)];
    this.attempt++;
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.connect(), delay);
  }

  disconnect(): void {
    this.closed = true;
    clearTimeout(this.timer);
    if (this.ws) {
      try {
        this.ws.close();
      } catch {
        /* ignore */
      }
      this.ws = null;
    }
    this.state = "offline";
  }

  restart(): void {
    this.closed = false;
    this.attempt = 0;
    this.connect();
  }

  send(e: Envelope): void {
    try {
      if (this.ws && this.ws.readyState === 1) this.ws.send(JSON.stringify(e));
    } catch {
      /* reconnection handles delivery */
    }
  }

  on(type: string, cb: Handler): () => void {
    (this.handlers[type] = this.handlers[type] || []).push(cb);
    return () => {
      this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== cb);
    };
  }

  onOpen(cb: () => void): () => void {
    this.openHandlers.push(cb);
    return () => {
      this.openHandlers = this.openHandlers.filter((h) => h !== cb);
    };
  }

  private emitLocal(type: string, data: any): void {
    for (const cb of this.handlers[type] || []) {
      try {
        cb(data);
      } catch {
        /* one broken listener must not break the socket */
      }
    }
  }
}

export const agentSocket = new AgentSocket();
