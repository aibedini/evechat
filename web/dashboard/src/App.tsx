import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
import { DiagnosticsPage } from "@/pages/settings/Diagnostics";
import { ActivityPage } from "@/pages/settings/Activity";
import { SecurityPage } from "@/pages/settings/Security";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: true,
      staleTime: 10000,
    },
  },
});

function AppLayout() {
  const { state } = useWebSocket({});
  return (
    <div className="flex h-screen overflow-hidden bg-slate-50">
      <Sidebar online={state === "open"} />
      <main className="flex min-w-0 flex-1 flex-col">
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
