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

function displayName(me: ReturnType<typeof useAuth>["me"]) {
  const fullName = typeof me?.full_name === "string" ? me.full_name.trim() : "";
  if (fullName) return fullName;
  return me?.email || "Пользователь";
}

function NavButton({ to, children }: { to: string; children: React.ReactNode }) {
  return (
    <NavLink to={to} style={{ textDecoration: "none" }}>
      {({ isActive }) => (
        <button style={isActive ? styles.btnPrimary : styles.btn} type="button">
          {children}
        </button>
      )}
    </NavLink>
  );
}

function NavGroup({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ display: "grid", gap: 6 }}>
      <div style={{ fontSize: 12, fontWeight: 700, color: "#6b7280" }}>{title}</div>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>{children}</div>
    </div>
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
        <div style={{ display: "grid", gap: 14, width: "100%" }}>
          <div
            style={{
              display: "flex",
              alignItems: "flex-start",
              justifyContent: "space-between",
              gap: 14,
              flexWrap: "wrap",
            }}
          >
            <div style={{ display: "grid", gap: 4 }}>
              <h1 style={styles.title}>Secure Voting</h1>
              <div style={styles.muted}>
                Клиент-серверное приложение электронных голосований
              </div>
            </div>

            {authed ? (
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "flex-end",
                  gap: 8,
                  flexWrap: "wrap",
                }}
              >
                <span style={{ ...styles.muted, fontSize: 13 }}>{displayName(me)}</span>
                <Badge text={roleLabel(me?.role)} />

                <NavButton to="/notifications">
                  Уведомления{unreadCount > 0 ? ` (${unreadCount})` : ""}
                </NavButton>

                <NavButton to="/profile">Профиль</NavButton>

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
            ) : (
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <NavButton to="/login">Войти</NavButton>
              </div>
            )}
          </div>

          {bootLoading ? <div style={styles.muted}>Проверка сессии…</div> : null}

          {authed ? (
            <div style={{ display: "grid", gap: 12 }}>
              <NavGroup title="Основные разделы">
                <NavButton to="/dashboard">Дашборд</NavButton>
                <NavButton to="/elections">Голосования</NavButton>
              </NavGroup>

              {isAdmin ? (
                <NavGroup title="Администрирование">
                  <NavButton to="/admin/elections/create">Создать голосование</NavButton>
                  <NavButton to="/monitor/jobs">Задачи</NavButton>
                  <NavButton to="/monitor/audit">Аудит</NavButton>
                  <NavButton to="/admin/users">Пользователи</NavButton>
                  <NavButton to="/admin/settings">Настройки</NavButton>
                </NavGroup>
              ) : null}

              {isResearcher ? (
                <NavGroup title="Исследования">
                  <NavButton to="/research/datasets">Наборы данных</NavButton>
                  <NavButton to="/research/experiments">Эксперименты</NavButton>
                  <NavButton to="/research/runs">Запуски</NavButton>
                </NavGroup>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>

      <ErrorBanner error={bootError} />

      {children}
    </div>
  );
}