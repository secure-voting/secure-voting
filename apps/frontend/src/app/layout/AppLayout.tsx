import React from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { useNotifications } from "../notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

export function AppLayout({ children }: { children: React.ReactNode }) {
  const { authed, me, bootLoading, bootError, logout } = useAuth();
  const { unreadCount } = useNotifications();
  const nav = useNavigate();

  const isResearchOrAdmin = me?.role === "researcher" || me?.role === "admin";

  return (
    <div style={styles.page}>
      <div style={styles.topbar}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <h1 style={styles.title}>secure voting</h1>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          {bootLoading ? <span style={styles.muted}>Проверка сессии</span> : null}

          {authed ? (
            <>
              <span style={styles.muted}>
                {me?.email} <Badge text={String(me?.role || "unknown")} />
              </span>

              <Link to="/dashboard" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Dashboard</button>
              </Link>

              <Link to="/elections" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Elections</button>
              </Link>

              {me?.role === "admin" ? (
                <Link to="/admin/elections/create" style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Create</button>
                </Link>
              ) : null}

              {me?.role === "researcher" ? (
                <Link to="/research/datasets" style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Datasets</button>
                </Link>
              ) : null}

              {isResearchOrAdmin ? (
                <>
                  <Link to="/research/experiments" style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Experiments</button>
                  </Link>
                  <Link to="/research/runs" style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Runs</button>
                  </Link>
                  <Link to="/monitor/jobs" style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Jobs</button>
                  </Link>
                </>
              ) : null}

              <Link to="/monitor/audit" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Audit</button>
              </Link>

              <Link to="/notifications" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>
                  Notifications{unreadCount > 0 ? ` (${unreadCount})` : ""}
                </button>
              </Link>

              <Link to="/profile" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Profile</button>
              </Link>

              <button
                style={styles.btnDanger}
                onClick={async () => {
                  await logout();
                  nav("/login", { replace: true });
                }}
              >
                Logout
              </button>
            </>
          ) : (
            <Link to="/login" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Login</button>
            </Link>
          )}
        </div>
      </div>

      <ErrorBanner error={bootError} />

      {children}
    </div>
  );
}