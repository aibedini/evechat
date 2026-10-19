import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { t } from "@/lib/i18n";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { AuthProvider, RequireAuth } from "@/hooks/useAuth";
import { useWebSocket } from "@/hooks/useWebSocket";
import { Sidebar } from "@/components/Sidebar";
import { LoginPage } from "@/pages/Login";
import { ForgotPage } from "@/pages/Forgot";
import { ResetPage } from "@/pages/Reset";
import { InboxPage } from "@/pages/Inbox";
import { DashboardPage } from "@/pages/Dashboard";
import { VisitorsPage } from "@/pages/Visitors";
import { TicketsPage } from "@/pages/Tickets";
import { KnowledgeBasePage } from "@/pages/KnowledgeBase";
import { WidgetSettingsPage } from "@/pages/settings/Widget";
import { RoutingSettingsPage } from "@/pages/settings/Routing";
import { TeamSettingsPage } from "@/pages/settings/Team";
import { IntegrationsSettingsPage } from "@/pages/settings/Integrations";
import { WebhooksSettingsPage } from "@/pages/settings/Webhooks";
import { ChannelsSettingsPage } from "@/pages/settings/Channels";
import { WallboardPage } from "@/pages/Wallboard";
import { DiagnosticsPage } from "@/pages/settings/Diagnostics";
import { ActivityPage } from "@/pages/settings/Activity";
import { SecurityPage } from "@/pages/settings/Security";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      // Realtime events keep the cache fresh; refetching every query on every
      // tab focus caused a fetch storm that made the inbox feel stuck (BUG-4).
      refetchOnWindowFocus: false,
      staleTime: 10000,
    },
  },
});

function AppLayout() {
  const { state } = useWebSocket({});
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className="app-shell bg-slate-50">
      <div className="sidebar-backdrop" data-open={menuOpen} onClick={() => setMenuOpen(false)} />
      <div className="sidebar-shell" data-open={menuOpen}>
        <Sidebar online={state === "open"} onNavigate={() => setMenuOpen(false)} />
      </div>
      <main className="flex min-w-0 flex-1 flex-col">
        <div className="mobile-bar">
          <button
            type="button"
            className="rounded-lg border border-slate-200 px-3 py-2 text-sm"
            aria-label={t("menu")}
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((v) => !v)}
          >
            ☰
          </button>
          <span className="text-sm font-semibold">{t("appName")}</span>
        </div>
        <Outlet />
      </main>
    </div>
  );
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/wallboard" element={<WallboardPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/forgot" element={<ForgotPage />} />
            <Route path="/reset" element={<ResetPage />} />
            <Route
              element={
                <RequireAuth>
                  <AppLayout />
                </RequireAuth>
              }
            >
              <Route path="/" element={<DashboardPage />} />
              <Route path="/inbox" element={<InboxPage />} />
              <Route path="/visitors" element={<VisitorsPage />} />
              <Route path="/tickets" element={<TicketsPage />} />
              <Route path="/knowledge-base" element={<KnowledgeBasePage />} />
              <Route path="/settings/widget" element={<WidgetSettingsPage />} />
              <Route path="/settings/routing" element={<RoutingSettingsPage />} />
              <Route path="/settings/team" element={<TeamSettingsPage />} />
              <Route path="/settings/integrations" element={<IntegrationsSettingsPage />} />
              <Route path="/settings/webhooks" element={<WebhooksSettingsPage />} />
          <Route path="/settings/channels" element={<ChannelsSettingsPage />} />
              <Route path="/settings/security" element={<SecurityPage />} />
              <Route path="/settings/activity" element={<ActivityPage />} />
              <Route path="/settings/diagnostics" element={<DiagnosticsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}