import { api, clearTokens, getAccessToken, setTokens } from "./api";
import type { Agent, Project } from "./types";

export interface Session {
  agent: Agent;
  project: Project;
}

const CACHE_KEY = "evechat:session";

export function cachedSession(): Session | null {
  try {
    const raw = localStorage.getItem(CACHE_KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

export function cacheSession(session: Session | null): void {
  if (!session) localStorage.removeItem(CACHE_KEY);
  else localStorage.setItem(CACHE_KEY, JSON.stringify(session));
}

export function isAuthenticated(): boolean {
  return !!getAccessToken();
}

export async function login(email: string, password: string, projectKey?: string): Promise<Session> {
  const data = await api<{
    access_token: string;
    refresh_token: string;
    agent: Agent;
    project: Project;
  }>("/api/v1/auth/login", {
    method: "POST",
    body: { email, password, project_key: projectKey || undefined },
    retry: false,
  });
  setTokens(data.access_token, data.refresh_token);
  const session: Session = { agent: data.agent, project: data.project };
  cacheSession(session);
  return session;
}

export async function me(): Promise<Session> {
  const data = await api<{ agent: Agent; project: Project }>("/api/v1/auth/me");
  const session: Session = { agent: data.agent, project: data.project };
  cacheSession(session);
  return session;
}

export async function logout(): Promise<void> {
  try {
    await api("/api/v1/auth/logout", { method: "POST" });
  } catch {
    /* the token may already be invalid */
  }
  clearTokens();
  cacheSession(null);
  location.assign("/login");
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await api("/api/v1/auth/password", {
    method: "PUT",
    body: { current_password: currentPassword, new_password: newPassword },
  });
}
