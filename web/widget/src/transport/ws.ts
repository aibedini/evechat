import type { Channel, ChannelHost, Envelope } from "../types";

const PING_EVERY = 25000;
const PONG_TIMEOUT = 20000;

/**
 * Primary transport. The server keeps the socket alive with app-level pings;
 * a missed pong forces a reconnect (and therefore a sync.request).
 */
export class WsChannel implements Channel {
  readonly mode = "ws" as const;
  private ws: WebSocket | null = null;
  private pingTimer: any = 0;
  private lastPong = 0;
  private closed = false;

  constructor(private url: string, private host: ChannelHost) {}

  open(): void {
    this.closed = false;
    let ws: WebSocket;
    try {
      ws = new WebSocket(this.url);
    } catch {
      this.host.onFail();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      this.lastPong = Date.now();
      this.host.onOpen();
      this.startHeartbeat();
    };
    ws.onmessage = (ev: MessageEvent) => {
      let env: Envelope;
      try {
        env = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      if (env.type === "ping") {
        this.rawSend({ type: "pong" });
        return;
      }
      if (env.type === "pong") {
        this.lastPong = Date.now();
        return;
      }
      this.lastPong = Date.now();
      this.host.onEvent(env);
    };
    ws.onclose = () => {
      this.stopHeartbeat();
      if (!this.closed) this.host.onFail();
    };
    ws.onerror = () => {
      /* onclose always follows; failure is reported there */
    };
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.pingTimer = setInterval(() => {
      if (Date.now() - this.lastPong > PONG_TIMEOUT) {
        this.host.onFail();
        this.close();
        return;
      }
      this.rawSend({ type: "ping" });
    }, PING_EVERY);
  }

  private stopHeartbeat(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = 0;
    }
  }

  private rawSend(e: Envelope): void {
    try {
      if (this.ws && this.ws.readyState === 1) this.ws.send(JSON.stringify(e));
    } catch {
      /* the socket will report a close event */
    }
  }

  send(e: Envelope): Promise<void> {
    if (!this.ws || this.ws.readyState !== 1) return Promise.reject(new Error("socket not open"));
    this.rawSend(e);
    return Promise.resolve();
  }

  close(): void {
    this.closed = true;
    this.stopHeartbeat();
    if (this.ws) {
      try {
        this.ws.close();
      } catch {
        /* ignore */
      }
      this.ws = null;
    }
  }
}
