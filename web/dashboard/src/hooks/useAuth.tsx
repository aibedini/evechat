import React, { createContext, useContext, useEffect, useState } from "react";
import { Navigate, useLocation } from "react-router-dom";
import { cachedSession, login as doLogin, logout as doLogout, me, type Session } from "@/lib/auth";
import { isAuthenticated } from "@/lib/auth";
import { t } from "@/lib/i18n";

interface AuthContextValue {
  session: Session | null;
  loading: boolean;
  signIn: (email: string, password: string, projectKey?: string) => Promise<void>;
  signOut: () => Promise<void>;
  reload: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(cachedSession());
  const [loading, setLoading] = useState<boolean>(isAuthenticated());

  useEffect(() => {
    let alive = true;
    if (!isAuthenticated()) {
      setLoading(false);
      return;
    }
    me()
      .then((s) => alive && setSession(s))
      .catch(() => alive && setSession(null))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, []);

  const value: AuthContextValue = {
    session,
    loading,
    signIn: async (email, password, projectKey) => {
      const s = await doLogin(email, password, projectKey);
      setSession(s);
    },
    signOut: async () => {
      await doLogout();
      setSession(null);
    },
    reload: async () => {
      const s = await me();
      setSession(s);
    },
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { session, loading } = useAuth();
  const location = useLocation();
  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center text-sm text-slate-500">{t("loading")}</div>
    );
  }
  if (!session) return <Navigate to="/login" state={{ from: location }} replace />;
  return <>{children}</>;
}
