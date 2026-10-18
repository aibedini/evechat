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
  };
}

export function storageGet(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function storageSet(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    /* private mode: drafts simply do not persist */
  }
}

export function storageRemove(key: string): void {
  try {
    window.localStorage.removeItem(key);
  } catch {
    /* ignore */
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
