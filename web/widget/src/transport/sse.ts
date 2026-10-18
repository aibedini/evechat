import type { Channel, ChannelHost, Envelope } from "../types";

/**
 * Degraded transport: Server-Sent Events for the downlink plus REST for the
 * uplink. Used when the WebSocket handshake is blocked by a proxy.
 */
export class SseChannel implements Channel {
  readonly mode = "sse" as const;
  private es: EventSource | null = null;
  private closed = false;

  constructor(private url: string, private host: ChannelHost) {}

  open(): void {
    this.closed = false;
    let es: EventSource;
    try {
      es = new EventSource(this.url);
    } catch {
      this.host.onFail();
      return;
    }
    this.es = es;
    let opened = false;
    es.onopen = () => {
      opened = true;
      this.host.onOpen();
    };
    es.onmessage = (ev: MessageEvent) => {
      let env: Envelope;
      try {
        env = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      this.host.onEvent(env);
    };
    es.onerror = () => {
      if (this.closed) return;
      // EventSource retries by itself, but the manager owns the backoff policy.
      es.close();
      this.es = null;
      if (!opened) {
        this.host.onFail();
        return;
      }
      this.host.onFail();
    };
  }

  send(_e: Envelope): Promise<void> {
    return Promise.reject(new Error("sse channel uses REST for uplink"));
  }

  close(): void {
    this.closed = true;
    if (this.es) {
      try {
        this.es.close();
      } catch {
        /* ignore */
      }
      this.es = null;
    }
  }
}
