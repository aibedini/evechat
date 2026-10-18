import type { Channel, ChannelHost, Envelope, Mode, TransportState } from "../types";

export interface TransportOptions {
  factories: Record<Mode, (url: string, host: ChannelHost) => Channel>;
  urls: Record<Mode, string>;
  onEvent(e: Envelope): void;
  onState(state: TransportState): void;
  onOpen(mode: Mode): void;
  /** Uplink for the degraded transports (SSE / long poll). */
  rest(e: Envelope): Promise<void>;
  /** Optional clock/timer injection for tests. */
  timers?: {
    setTimeout(fn: () => void, ms: number): any;
    clearTimeout(id: any): void;
    setInterval(fn: () => void, ms: number): any;
    clearInterval(id: any): void;
  };
}

const BACKOFF = [1000, 2000, 4000, 8000, 15000, 30000];
const UPGRADE_EVERY = 60000;

/** Exponential backoff with ±25% jitter, capped at 30s. */
export function backoff(attempt: number, random: () => number = Math.random): number {
  const base = BACKOFF[Math.min(Math.max(attempt, 0), BACKOFF.length - 1)];
  const jitter = base * 0.25;
  return Math.round(base - jitter + random() * jitter * 2);
}

/**
 * Transport state machine:
 *   CONNECTING → OPEN → RECONNECTING → DEGRADED → OFFLINE
 * Attempt order: WS → (2 failures) SSE+REST → (2 failures) LongPoll+REST.
 * While degraded, every 60s it tries to upgrade back to WS exactly once.
 */
export class TransportManager {
  state: TransportState = "offline";
  mode: Mode = "ws";
  private channel: Channel | null = null;
  private failures: Record<Mode, number> = { ws: 0, sse: 0, poll: 0 };
  private attempt = 0;
  private stopped = true;
  private timer: any = 0;
  private upgradeTimer: any = 0;
  private upgrading = false;
  private timers: NonNullable<TransportOptions["timers"]>;

  constructor(private opts: TransportOptions) {
    this.timers =
      opts.timers ||
      ({
        setTimeout: (fn, ms) => setTimeout(fn, ms),
        clearTimeout: (id) => clearTimeout(id),
        setInterval: (fn, ms) => setInterval(fn, ms),
        clearInterval: (id) => clearInterval(id),
      } as NonNullable<TransportOptions["timers"]>);
  }

  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    this.failures = { ws: 0, sse: 0, poll: 0 };
    this.attempt = 0;
    this.mode = "ws";
    this.connect();
  }

  stop(): void {
    this.stopped = true;
    this.timers.clearTimeout(this.timer);
    this.timers.clearInterval(this.upgradeTimer);
    if (this.channel) {
      this.channel.close();
      this.channel = null;
    }
    this.setState("offline");
  }

  private connect(): void {
    if (this.stopped) return;
    if (this.state !== "open") {
      this.setState(this.attempt === 0 ? "connecting" : this.mode === "ws" ? "reconnecting" : "degraded");
    }
    const host: ChannelHost = {
      onOpen: () => this.handleOpen(),
      onEvent: (e) => this.opts.onEvent(e),
      onFail: () => this.handleFail(),
    };
    const factory = this.opts.factories[this.mode];
    const channel = factory(this.opts.urls[this.mode], host);
    this.channel = channel;
    channel.open();
  }

  private handleOpen(): void {
    this.failures[this.mode] = 0;
    this.attempt = 0;
    this.upgrading = false;
    this.timers.clearInterval(this.upgradeTimer);
    this.setState("open");
    this.opts.onOpen(this.mode);
    if (this.mode !== "ws") this.scheduleUpgrade();
  }

  private scheduleUpgrade(): void {
    this.timers.clearInterval(this.upgradeTimer);
    this.upgradeTimer = this.timers.setInterval(() => this.tryUpgrade(), UPGRADE_EVERY);
  }

  private tryUpgrade(): void {
    if (this.stopped || this.mode === "ws" || this.upgrading) return;
    this.upgrading = true;
    if (this.channel) {
      this.channel.close();
      this.channel = null;
    }
    this.mode = "ws";
    this.attempt = 0;
    this.connect();
  }

  private handleFail(): void {
    if (this.stopped) return;
    const failed = this.mode;
    if (this.channel) {
      this.channel.close();
      this.channel = null;
    }
    this.failures[failed]++;

    if (this.upgrading) {
      // The upgrade probe failed: fall straight back to the previous degraded mode.
      this.upgrading = false;
      this.mode = this.failures.sse >= 2 ? "poll" : "sse";
      this.attempt = 0;
      this.timers.clearTimeout(this.timer);
      this.timer = this.timers.setTimeout(() => this.connect(), backoff(0));
      this.setState("degraded");
      return;
    }

    if (failed === "ws" && this.failures.ws >= 2) {
      this.mode = "sse";
    } else if (failed === "sse" && this.failures.sse >= 2) {
      this.mode = "poll";
    }

    const delay = backoff(this.attempt);
    this.attempt++;
    this.setState(this.mode === "ws" ? "reconnecting" : "degraded");
    this.timers.clearTimeout(this.timer);
    this.timer = this.timers.setTimeout(() => this.connect(), delay);
  }

  private setState(next: TransportState): void {
    if (this.state === next) return;
    this.state = next;
    this.opts.onState(next);
  }

  /** Sends through the active uplink; rejects so the caller can queue. */
  async send(e: Envelope): Promise<void> {
    if (this.mode === "ws" && this.state === "open" && this.channel) {
      try {
        await this.channel.send(e);
        return;
      } catch {
        /* fall through to REST */
      }
    }
    await this.opts.rest(e);
  }

  /** True when the current transport can only push over REST. */
  get degraded(): boolean {
    return this.mode !== "ws";
  }
}
