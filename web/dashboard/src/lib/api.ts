import { copy } from "./copy.fa";

const ACCESS_KEY = "evechat:access";
const REFRESH_KEY = "evechat:refresh";

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public details?: any) {
    super(message);
  }
}

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_KEY) || "";
}

export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_KEY) || "";
}

export function setTokens(access: string, refresh?: string): void {
  localStorage.setItem(ACCESS_KEY, access);
  if (refresh) localStorage.setItem(REFRESH_KEY, refresh);
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

const BASE = ((import.meta as any).env?.VITE_API_URL as string) || "";

let refreshing: Promise<boolean> | null = null;

async function doRefresh(): Promise<boolean> {
  const refresh = getRefreshToken();
  if (!refresh) return false;
  if (!refreshing) {
    refreshing = fetch(BASE + "/api/v1/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refresh }),
    })
      .then(async (res) => {
        if (!res.ok) return false;
        const data = await res.json();
        setTokens(data.access_token, data.refresh_token);
        return true;
      })
      .catch(() => false)
      .finally(() => {
        setTimeout(() => (refreshing = null), 0);
      });
  }
  return refreshing;
}

type Options = {
  method?: string;
  body?: any;
  query?: Record<string, string | number | boolean | undefined | null>;
  form?: FormData;
  retry?: boolean;
};

function buildQuery(query?: Options["query"]): string {
  if (!query) return "";
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === "") continue;
    params.set(k, String(v));
  }
  const s = params.toString();
  return s ? "?" + s : "";
}

/** Typed fetch wrapper with automatic refresh-on-401. */
export async function api<T = any>(path: string, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = {};
  const token = getAccessToken();
  if (token) headers["Authorization"] = "Bearer " + token;
  let body: BodyInit | undefined;
  if (opts.form) {
    body = opts.form;
  } else if (opts.body !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(opts.body);
  }

  const res = await fetch(BASE + path + buildQuery(opts.query), {
    method: opts.method || (body ? "POST" : "GET"),
    headers,
    body,
  });

  if (res.status === 401 && opts.retry !== false) {
    if (await doRefresh()) {
      return api<T>(path, { ...opts, retry: false });
    }
    clearTokens();
    if (location.pathname !== "/login") location.assign("/login");
    throw new ApiError(401, "unauthorized", copy.errors.sessionExpired);
  }

  const text = await res.text();
  const data = text ? safeJSON(text) : null;
  if (!res.ok) {
    throw new ApiError(res.status, data?.code || "error", data?.message || copy.errors.unknownError, data?.details);
  }
  return data as T;
}

function safeJSON(text: string): any {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export { BASE as apiBase };
