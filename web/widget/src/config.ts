import type { HostConfig } from "./types";

const DEFAULT_API = "";

/** Reads window.EveChat and normalises every option. */
export function resolveConfig(): HostConfig | null {
  const raw = (window as any).EveChat || {};
  const projectKey = raw.projectKey || raw.project_key || "";
  if (!projectKey) return null;
  return {
    projectKey,
    apiUrl: String(raw.apiUrl || raw.api_url || DEFAULT_API).replace(/\/+$/, ""),
    locale: raw.locale === "en" ? "en" : "fa",
    position: raw.position === "bottom-right" ? "bottom-right" : "bottom-left",
    autoOpen: !!raw.autoOpen,
    visitor: raw.visitor || {},
    colors: raw.colors || {},
    theme: raw.theme === "light" || raw.theme === "dark" ? raw.theme : "auto",
    font: raw.font === "inherit" || raw.font === "none" ? raw.font : "vazirmatn",
  };
}

/**
 * Last-resort storage. Private-mode browsers, blocked third-party storage and
 * `file://` pages all throw on localStorage; without this the widget would lose
 * the session token and the outbox the moment the page reloads.
 */
const memoryStore = new Map<string, string>();
let localStorageUsable: boolean | null = null;

function localStorageOrNull(): Storage | null {
  if (localStorageUsable === false) return null;
  try {
    const storage = window.localStorage;
    if (!storage) return null;
    if (localStorageUsable === null) {
      // Touching the object is not enough; some browsers only throw on write.
      storage.setItem("evechat:probe", "1");
      storage.removeItem("evechat:probe");
      localStorageUsable = true;
    }
    return storage;
  } catch {
    localStorageUsable = false;
    return null;
  }
}

export function storageGet(key: string): string | null {
  const storage = localStorageOrNull();
  if (storage) {
    try {
      return storage.getItem(key);
    } catch {
      localStorageUsable = false;
    }
  }
  const value = memoryStore.get(key);
  return value === undefined ? null : value;
}

export function storageSet(key: string, value: string): void {
  // Mirror into memory first so a storage failure never loses the value.
  memoryStore.set(key, value);
  const storage = localStorageOrNull();
  if (!storage) return;
  try {
    storage.setItem(key, value);
  } catch {
    localStorageUsable = false;
  }
}

export function storageRemove(key: string): void {
  memoryStore.delete(key);
  const storage = localStorageOrNull();
  if (!storage) return;
  try {
    storage.removeItem(key);
  } catch {
    localStorageUsable = false;
  }
}

export function sessionKey(projectKey: string): string {
  return "evechat:token:" + projectKey;
}

export function draftKey(projectKey: string): string {
  return "evechat:draft:" + projectKey;
}

export function openKey(projectKey: string): string {
  return "evechat:open:" + projectKey;
}

/** Small debounce helper used for typing + draft persistence. */
export function debounce<T extends (...args: any[]) => void>(fn: T, ms: number): T {
  let timer: any = 0;
  return function (this: any, ...args: any[]) {
    clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), ms);
  } as T;
}

/** Replaces {placeholder} tokens in a translated string. */
export function fmt(template: string, vars: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_, key) => vars[key] || "");
}
