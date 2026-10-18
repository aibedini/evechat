import type { Channel, ChannelHost, Envelope } from "../types";

/**
 * Last-resort transport: long polling. The server holds a request up to 25s and
 * returns every buffered envelope newer than `since`.
 */
export class PollChannel implements Channel {
  readonly mode = "poll" as const;
  private closed = false;
  private since = 0;
  private controller: AbortController | null = null;

  constructor(private url: string, private host: ChannelHost) {}

  open(): void {
    this.closed = false;
    this.host.onOpen();
    this.loop();
  }

  private async loop(): Promise<void> {
    while (!this.closed) {
      this.controller = new AbortController();
      const timeout = setTimeout(() => this.controller && this.controller.abort(), 32000);
      try {
        const res = await fetch(this.url + "&since=" + this.since, {
          signal: this.controller.signal,
          headers: { Accept: "application/json" },
          credentials: "omit",
        });
        clearTimeout(timeout);
        if (!res.ok) throw new Error("poll status " + res.status);
        const body = (await res.json()) as { events?: Envelope[]; now?: number };
        for (const env of body.events || []) {
          if (env.ts && env.ts > this.since) this.since = env.ts;
          this.host.onEvent(env);
        }
        if (body.now && body.now > this.since) this.since = body.now;
      } catch {
        clearTimeout(timeout);
        if (!this.closed) this.host.onFail();
        return;
      }
    }
  }

  send(_e: Envelope): Promise<void> {
    return Promise.reject(new Error("poll channel uses REST for uplink"));
  }

  close(): void {
    this.closed = true;
    if (this.controller) {
      try {
        this.controller.abort();
      } catch {
        /* ignore */
      }
      this.controller = null;
    }
  }
}
