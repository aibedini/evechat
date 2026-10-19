import { NavLink, useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { t } from "@/lib/i18n";
import { cn, initials } from "@/lib/utils";
import { resolvedTheme, toggleTheme } from "@/lib/theme";
import { useState } from "react";

const links = [
  { to: "/inbox", label: t("inbox"), icon: "💬" },
  { to: "/", label: t("dashboard"), icon: "📊" },
  { to: "/visitors", label: t("visitors"), icon: "👥" },
  { to: "/tickets", label: t("tickets"), icon: "🎫" },
  { to: "/knowledge-base", label: t("knowledgeBase"), icon: "📚" },
];

const settings = [
  { to: "/settings/widget", label: t("widgetSettings"), icon: "🎨" },
  { to: "/settings/routing", label: t("routing"), icon: "🧭" },
  { to: "/settings/team", label: t("team"), icon: "👤" },
  { to: "/settings/integrations", label: t("integrations"), icon: "🔌" },
  { to: "/settings/webhooks", label: t("webhooks"), icon: "🪝" },
  { to: "/settings/channels", label: t("channels"), icon: "🔀" },
  { to: "/settings/security", label: t("security"), icon: "🛡️" },
  { to: "/settings/activity", label: t("activity"), icon: "📜" },
  { to: "/settings/diagnostics", label: t("health"), icon: "🩺" },
];

export function Sidebar({ online, onNavigate }: { online: boolean; onNavigate?: () => void }) {
  const { session, signOut } = useAuth();
  const navigate = useNavigate();
  const [dark, setDark] = useState(resolvedTheme() === "dark");

  return (
    <aside className="flex w-60 shrink-0 flex-col bg-slate-900 text-white">
      <div className="flex items-center gap-2 px-5 py-5">
        <span className="grid h-9 w-9 place-items-center rounded-xl bg-brand-600 text-lg">⚡</span>
        <div>
          <div className="text-sm font-bold">{t("appName")}</div>
          <div className="text-[11px] text-slate-400">{session?.project?.name || ""}</div>
        </div>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3">
        {links.map((l) => (
          <NavLink key={l.to} to={l.to} end={l.to === "/"} onClick={onNavigate} className={({ isActive }) => cn("nav-link", isActive && "active")}>
            <span>{l.icon}</span>
            <span>{l.label}</span>
          </NavLink>
        ))}
        <div className="px-3 pb-1 pt-5 text-[11px] font-semibold uppercase tracking-wide text-slate-500">
          {t("settings")}
        </div>
        {settings.map((l) => (
          <NavLink key={l.to} to={l.to} onClick={onNavigate} className={({ isActive }) => cn("nav-link", isActive && "active")}>
            <span>{l.icon}</span>
            <span>{l.label}</span>
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-slate-800 p-3">
        <div className="mb-2 flex items-center gap-2 rounded-xl bg-slate-800/60 px-3 py-2">
          <span className="grid h-8 w-8 place-items-center rounded-full bg-brand-600 text-xs font-bold">
            {initials(session?.agent?.name)}
          </span>
          <div className="min-w-0 flex-1">
            <div className="truncate text-xs font-medium">{session?.agent?.name}</div>
            <div className="flex items-center gap-1 text-[10px] text-slate-400">
              <span className={cn("h-1.5 w-1.5 rounded-full", online ? "bg-emerald-400" : "bg-amber-400")} />
              {online ? t("online") : t("offline")}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setDark(toggleTheme() === "dark")}
            className="flex-1 rounded-xl px-3 py-2 text-xs text-slate-400 hover:bg-slate-800 hover:text-white"
            aria-label={dark ? t("lightMode") : t("darkMode")}
          >
            {dark ? t("themeLight") : t("themeDark")}
          </button>
          <button
            onClick={async () => {
              await signOut();
              navigate("/login");
            }}
            className="flex-1 rounded-xl px-3 py-2 text-xs text-slate-400 hover:bg-slate-800 hover:text-white"
          >
            {t("logout")}
          </button>
        </div>
      </div>
    </aside>
  );
}