import React from "react";
import { NavLink, useNavigate } from "react-router-dom";
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

function NavButton({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink to={to} style={{ textDecoration: "none" }}>
      {({ isActive }) => (
        <button style={isActive ? styles.btnPrimary : styles.btn}>{children}</button>
      )}
    </NavLink>
  );
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
        <div style={{ display: "flex", flexDirection: "column", gap: 10, width: "100%" }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 12,
              justifyContent: "space-between",
              flexWrap: "wrap",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <h1 style={styles.title}>Secure Voting</h1>
              {authed ? (
                <span style={styles.muted}>
                  {me?.email} <Badge text={roleLabel(me?.role)} />
                </span>
              ) : null}
            </div>

            {bootLoading ? <span style={styles.muted}>Проверка сессии</span> : null}
          </div>

          {authed ? (
            <>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <NavButton to="/dashboard">Дашборд</NavButton>
                <NavButton to="/elections">Голосования</NavButton>

                {isAdmin ? <NavButton to="/admin/elections/create">Создать голосование</NavButton> : null}
                {isAdmin ? <NavButton to="/monitor/jobs">Задачи</NavButton> : null}
                {isAdmin ? <NavButton to="/monitor/audit">Аудит</NavButton> : null}
                {isAdmin ? <NavButton to="/admin/users">Пользователи</NavButton> : null}

                {isResearcher ? <NavButton to="/research/datasets">Наборы данных</NavButton> : null}
                {isResearcher ? <NavButton to="/research/experiments">Эксперименты</NavButton> : null}
                {isResearcher ? <NavButton to="/research/runs">Запуски</NavButton> : null}
              </div>

              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <NavButton to="/notifications">
                  Уведомления{unreadCount > 0 ? ` (${unreadCount})` : ""}
                </NavButton>
                <NavButton to="/profile">Профиль</NavButton>
                <NavButton to="/admin/settings">Настройки</NavButton>

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
              </div>
            </>
          ) : (
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <NavButton to="/login">Войти</NavButton>
            </div>
          )}
        </div>
      </div>

      <ErrorBanner error={bootError} />

      {children}
    </div>
  );
}