import { useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

export function LoginPage() {
  const { signIn, session } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  if (session) navigate("/inbox", { replace: true });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await signIn(email, password, projectKey || undefined);
      if (!remember) sessionStorage.setItem("evechat:session-only", "1");
      const from = (location.state as any)?.from?.pathname || "/inbox";
      navigate(from, { replace: true });
    } catch (err: any) {
      setError(err?.message || "ورود ناموفق بود");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 p-4">
      <div className="w-full max-w-sm rounded-3xl bg-white p-7 shadow-2xl">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-brand-600 text-2xl text-white">
            ⚡
          </div>
          <h1 className="text-lg font-bold">ورود به {t("appName")}</h1>
          <p className="mt-1 text-xs text-slate-400">پنل پشتیبانی مشتریان</p>
        </div>

        <form onSubmit={submit} className="space-y-4">
          <div>
            <Label htmlFor="email">{t("email")}</Label>
            <Input
              id="email"
              type="email"
              dir="ltr"
              autoComplete="username"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@evechat.local"
              required
            />
          </div>
          <div>
            <Label htmlFor="password">{t("password")}</Label>
            <Input
              id="password"
              type="password"
              dir="ltr"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          <div>
            <Label htmlFor="projectKey">کلید پروژه (اختیاری)</Label>
            <Input
              id="projectKey"
              dir="ltr"
              value={projectKey}
              onChange={(e) => setProjectKey(e.target.value)}
              placeholder="pk_..."
            />
          </div>
          <label className="flex items-center gap-2 text-xs text-slate-600">
            <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
            {t("rememberMe")}
          </label>

          {error && <div className="rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}

          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? t("loading") : t("login")}
          </Button>
          <div className="text-center text-xs">
            <Link to="/forgot" className="text-slate-500 hover:text-brand-600">
              رمز عبور را فراموش کرده‌اید؟
            </Link>
          </div>
        </form>
      </div>
    </div>
  );
}
