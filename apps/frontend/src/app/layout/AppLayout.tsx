import React from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { useNotifications } from "../notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

function roleLabel(role?: string) {
  if (role === "admin") return "Администратор";
  if (role === "researcher") return "Исследователь";
  if (role === "voter") return "Голосующий";
  return role || "Неизвестно";
}

export function AppLayout({ children }: { children: React.ReactNode }) {
  const { authed, me, bootLoading, bootError, logout } = useAuth();
  const { unreadCount } = useNotifications();
  const nav = useNavigate();

  const isAdmin = me?.role === "admin";
  const isResearcher = me?.role === "researcher";

  return (
    <div style={styles.page}>
      <div style={styles.topbar}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <h1 style={styles.title}>Secure Voting</h1>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          {bootLoading ? <span style={styles.muted}>Проверка сессии</span> : null}

          {authed ? (
            <>
              <span style={styles.muted}>
                {me?.email} <Badge text={roleLabel(me?.role)} />
              </span>

              <Link to="/dashboard" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Дашборд</button>
              </Link>

              <Link to="/elections" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Голосования</button>
              </Link>

              {isAdmin ? (
                <Link to="/admin/elections/create" style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Создать голосование</button>
                </Link>
              ) : null}

              {isResearcher ? (
                <>
                  <Link to="/research/datasets" style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Наборы данных</button>
                  </Link>
                  <Link to="/research/experiments" style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Эксперименты</button>
                  </Link>
                  <Link to="/research/runs" style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Запуски</button>
                  </Link>
                </>
              ) : null}

              {(isAdmin || isResearcher) ? (
                <Link to="/monitor/jobs" style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Задачи</button>
                </Link>
              ) : null}

              {isAdmin ? (
                <Link to="/monitor/audit" style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Аудит</button>
                </Link>
              ) : null}

              <Link to="/notifications" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>
                  Уведомления{unreadCount > 0 ? ` (${unreadCount})` : ""}
                </button>
              </Link>

              <Link to="/profile" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Профиль</button>
              </Link>

              <button
                style={styles.btnDanger}
                onClick={async () => {
                  await logout();
                  nav("/login", { replace: true });
                }}
                type="button"
              >
                Выйти
              </button>
            </>
          ) : (
            <Link to="/login" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Войти</button>
            </Link>
          )}
        </div>
      </div>

      <ErrorBanner error={bootError} />

      {children}
    </div>
  );
}