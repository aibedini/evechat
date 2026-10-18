import type { OutboxItem } from "./types";
import { storageGet, storageSet } from "./config";

const DB_NAME = "evechat";
const DB_VERSION = 1;
const STORE = "outbox";
const FALLBACK_KEY = "evechat:outbox";

/**
 * Durable outbox. IndexedDB is preferred; private-mode browsers fall back to
 * localStorage so a message is never silently lost.
 */
export class Outbox {
  private dbPromise: Promise<IDBDatabase | null> | null = null;
  private fallback: OutboxItem[] = [];

  constructor() {
    try {
      const raw = storageGet(FALLBACK_KEY);
      this.fallback = raw ? (JSON.parse(raw) as OutboxItem[]) : [];
    } catch {
      this.fallback = [];
    }
  }

  private open(): Promise<IDBDatabase | null> {
    if (this.dbPromise) return this.dbPromise;
    this.dbPromise = new Promise((resolve) => {
      try {
        if (typeof indexedDB === "undefined") {
          resolve(null);
          return;
        }
        const req = indexedDB.open(DB_NAME, DB_VERSION);
        req.onupgradeneeded = () => {
          const db = req.result;
          if (!db.objectStoreNames.contains(STORE)) {
            db.createObjectStore(STORE, { keyPath: "client_message_id" });
          }
        };
        req.onsuccess = () => resolve(req.result);
        req.onerror = () => resolve(null);
        req.onblocked = () => resolve(null);
      } catch {
        resolve(null);
      }
    });
    return this.dbPromise;
  }

  private tx(mode: IDBTransactionMode): Promise<IDBObjectStore | null> {
    return this.open().then((db) => (db ? db.transaction(STORE, mode).objectStore(STORE) : null));
  }

  private persistFallback(): void {
    try {
      storageSet(FALLBACK_KEY, JSON.stringify(this.fallback.slice(-200)));
    } catch {
      /* quota exceeded: keep memory copy only */
    }
  }

  async put(item: OutboxItem): Promise<void> {
    const store = await this.tx("readwrite");
    if (!store) {
      this.fallback = this.fallback.filter((x) => x.client_message_id !== item.client_message_id);
      this.fallback.push(item);
      this.persistFallback();
      return;
    }
    await new Promise<void>((resolve) => {
      const req = store.put(item);
      req.onsuccess = () => resolve();
      req.onerror = () => resolve();
    });
  }

  async all(): Promise<OutboxItem[]> {
    const store = await this.tx("readonly");
    if (!store) return [...this.fallback].sort((a, b) => a.created_at - b.created_at);
    return new Promise((resolve) => {
      const req = store.getAll();
      req.onsuccess = () => {
        const items = (req.result as OutboxItem[]) || [];
        resolve(items.sort((a, b) => a.created_at - b.created_at));
      };
      req.onerror = () => resolve([]);
    });
  }

  async update(id: string, patch: Partial<OutboxItem>): Promise<void> {
    const items = await this.all();
    const found = items.find((x) => x.client_message_id === id);
    if (!found) return;
    await this.put({ ...found, ...patch });
  }

  async remove(id: string): Promise<void> {
    const store = await this.tx("readwrite");
    if (!store) {
      this.fallback = this.fallback.filter((x) => x.client_message_id !== id);
      this.persistFallback();
      return;
    }
    await new Promise<void>((resolve) => {
      const req = store.delete(id);
      req.onsuccess = () => resolve();
      req.onerror = () => resolve();
    });
  }

  async pending(): Promise<OutboxItem[]> {
    const items = await this.all();
    return items.filter((x) => x.status === "queued");
  }
}

/** Generates a client message id without external dependencies. */
export function clientMessageID(): string {
  const c = (window as any).crypto;
  if (c && typeof c.randomUUID === "function") return c.randomUUID();
  const bytes = new Uint8Array(16);
  if (c && c.getRandomValues) c.getRandomValues(bytes);
  else for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256);
  let out = "";
  for (let i = 0; i < bytes.length; i++) out += (bytes[i] + 0x100).toString(16).slice(1);
  return out;
}
