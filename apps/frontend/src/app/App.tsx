import React from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth";
import { NotificationsProvider } from "./notifications";
import { I18nProvider } from "./i18n";
import { AppLayout } from "./layout/AppLayout";
import { RequireAuth } from "./routing/RequireAuth";
import { RequireRole } from "./routing/RequireRole";
import { RequireAnyRole } from "./routing/RequireAnyRole";

import { LoginPage } from "../pages/auth/LoginPage";
import { ElectionsPage } from "../pages/voter/ElectionsPage";
import { ElectionPage } from "../pages/voter/ElectionPage";
import { VotePage } from "../pages/voter/VotePage";
import { ResultsPage } from "../pages/voter/ResultsPage";
import { AdminCreateElectionPage } from "../pages/admin/AdminCreateElectionPage";
import { ElectionRulesPage } from "../pages/admin/ElectionRulesPage";
import { DatasetsPage } from "../pages/researcher/DatasetsPage";
import { ExperimentsPage } from "../pages/researcher/ExperimentsPage";
import { ExperimentCreatePage } from "../pages/researcher/ExperimentCreatePage";
import { ExperimentRunsPage } from "../pages/researcher/ExperimentRunsPage";
import { JobsPage } from "../pages/monitoring/JobsPage";
import { AuditLogPage } from "../pages/monitoring/AuditLogPage";
import { ProfilePage } from "../pages/profile/ProfilePage";
import { NotificationsPage } from "../pages/profile/NotificationsPage";
import { DashboardRedirectPage } from "../pages/dashboard/DashboardRedirectPage";
import { VoterDashboardPage } from "../pages/dashboard/VoterDashboardPage";
import { AdminDashboardPage } from "../pages/dashboard/AdminDashboardPage";
import { ResearcherDashboardPage } from "../pages/dashboard/ResearcherDashboardPage";

function HomeRedirect() {
  const { authed } = useAuth();
  return <Navigate to={authed ? "/dashboard" : "/login"} replace />;
}

export default function App() {
  return (
    <AuthProvider>
      <I18nProvider>
        <NotificationsProvider>
          <AppLayout>
            <Routes>
              <Route path="/" element={<HomeRedirect />} />
              <Route path="/login" element={<LoginPage />} />

              <Route
                path="/dashboard"
                element={
                  <RequireAuth>
                    <DashboardRedirectPage />
                  </RequireAuth>
                }
              />
              <Route
                path="/dashboard/voter"
                element={
                  <RequireAuth>
                    <VoterDashboardPage />
                  </RequireAuth>
                }
              />
              <Route
                path="/dashboard/admin"
                element={
                  <RequireAuth>
                    <RequireRole role="admin">
                      <AdminDashboardPage />
                    </RequireRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/dashboard/researcher"
                element={
                  <RequireAuth>
                    <RequireRole role="researcher">
                      <ResearcherDashboardPage />
                    </RequireRole>
                  </RequireAuth>
                }
              />

              <Route
                path="/elections"
                element={
                  <RequireAuth>
                    <ElectionsPage />
                  </RequireAuth>
                }
              />
              <Route
                path="/elections/:id"
                element={
                  <RequireAuth>
                    <ElectionPage />
                  </RequireAuth>
                }
              />
              <Route
                path="/elections/:id/vote"
                element={
                  <RequireAuth>
                    <RequireRole role="voter">
                      <VotePage />
                    </RequireRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/elections/:id/results"
                element={
                  <RequireAuth>
                    <ResultsPage />
                  </RequireAuth>
                }
              />

              <Route
                path="/admin/elections/create"
                element={
                  <RequireAuth>
                    <RequireRole role="admin">
                      <AdminCreateElectionPage />
                    </RequireRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/admin/elections/:id/rules"
                element={
                  <RequireAuth>
                    <RequireRole role="admin">
                      <ElectionRulesPage />
                    </RequireRole>
                  </RequireAuth>
                }
              />

              <Route
                path="/research/datasets"
                element={
                  <RequireAuth>
                    <RequireRole role="researcher">
                      <DatasetsPage />
                    </RequireRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/research/experiments"
                element={
                  <RequireAuth>
                    <RequireAnyRole roles={["admin", "researcher"]}>
                      <ExperimentsPage />
                    </RequireAnyRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/research/experiments/create"
                element={
                  <RequireAuth>
                    <RequireRole role="researcher">
                      <ExperimentCreatePage />
                    </RequireRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/research/runs"
                element={
                  <RequireAuth>
                    <RequireAnyRole roles={["admin", "researcher"]}>
                      <ExperimentRunsPage />
                    </RequireAnyRole>
                  </RequireAuth>
                }
              />

              <Route
                path="/monitor/jobs"
                element={
                  <RequireAuth>
                    <RequireAnyRole roles={["admin", "researcher"]}>
                      <JobsPage />
                    </RequireAnyRole>
                  </RequireAuth>
                }
              />
              <Route
                path="/monitor/audit"
                element={
                  <RequireAuth>
                    <RequireAnyRole roles={["admin", "researcher", "voter"]}>
                      <AuditLogPage />
                    </RequireAnyRole>
                  </RequireAuth>
                }
              />

              <Route
                path="/profile"
                element={
                  <RequireAuth>
                    <ProfilePage />
                  </RequireAuth>
                }
              />
              <Route
                path="/notifications"
                element={
                  <RequireAuth>
                    <NotificationsPage />
                  </RequireAuth>
                }
              />

              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </AppLayout>
        </NotificationsProvider>
      </I18nProvider>
    </AuthProvider>
  );
}