import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

const page = copy.pages.reset;

export function ResetPage() {
  const [params] = useSearchParams();
  const token = params.get("token") || "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [done, setDone] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (password.length < 8) {
      setError(page.tooShort);
      return;
    }
    if (password !== confirm) {
      setError(page.mismatch);
      return;
    }
    setLoading(true);
    try {
      await api("/api/v1/auth/reset", {
        method: "POST",
        body: { token, password },
        retry: false,
      });
      setDone(true);
    } catch (err: any) {
      setError(err?.message || t("error"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 p-4">
      <div className="w-full max-w-sm rounded-3xl bg-white p-7 shadow-2xl">
        <h1 className="mb-1 text-lg font-bold">{page.title}</h1>
        <p className="mb-5 text-xs leading-5 text-slate-400">{page.intro}</p>
        {!token && <div className="rounded-xl bg-red-50 p-3 text-xs text-red-700">{page.invalidLink}</div>}
        {done ? (
          <div className="space-y-4">
            <div className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-700">{page.changed}</div>
            <Link to="/login" className="block text-center text-xs text-brand-600 hover:underline">
              {page.loginWithNew}
            </Link>
          </div>
        ) : (
          token && (
            <form onSubmit={submit} className="space-y-4">
              <div>
                <Label htmlFor="pw">{page.newPassword}</Label>
                <Input id="pw" type="password" dir="ltr" value={password} onChange={(e) => setPassword(e.target.value)} />
              </div>
              <div>
                <Label htmlFor="pw2">{page.repeat}</Label>
                <Input id="pw2" type="password" dir="ltr" value={confirm} onChange={(e) => setConfirm(e.target.value)} />
              </div>
              {error && <div className="rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? t("loading") : t("save")}
              </Button>
            </form>
          )
        )}
      </div>
    </div>
  );
}
