import { useState } from "react";
import { Link } from "react-router-dom";
import { api } from "@/lib/api";
import { t } from "@/lib/i18n";
import { copy } from "@/lib/copy.fa";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

const page = copy.pages.forgot;

export function ForgotPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await api("/api/v1/auth/forgot", { method: "POST", body: { email }, retry: false });
      setSent(true);
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
        <p className="mb-5 text-xs text-slate-400">{page.body}</p>
        {sent ? (
          <div className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-700">
            {page.sent}
          </div>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            <div>
              <Label htmlFor="email">{t("email")}</Label>
              <Input
                id="email"
                type="email"
                dir="ltr"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            {error && <div className="rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? t("loading") : page.submit}
            </Button>
          </form>
        )}
        <div className="mt-4 text-center text-xs">
          <Link to="/login" className="text-brand-600 hover:underline">
            {page.back}
          </Link>
        </div>
      </div>
    </div>
  );
}
