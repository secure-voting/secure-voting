import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { AdminUser } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

function roleLabel(role?: string) {
  if (role === "admin") return "Администратор";
  if (role === "researcher") return "Исследователь";
  if (role === "voter") return "Голосующий";
  return role || "—";
}

const ROLE_OPTIONS = [
  { value: "admin", label: "Администратор" },
  { value: "researcher", label: "Исследователь" },
  { value: "voter", label: "Голосующий" },
];

export function AdminUsersPage() {
  const { token, me, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [items, setItems] = useState<AdminUser[]>([]);
  const [draftRoles, setDraftRoles] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [savingUserID, setSavingUserID] = useState<string>("");
  const [err, setErr] = useState<string>("");

  const load = useCallback(async () => {
    if (!token) return;

    setLoading(true);
    setErr("");

    try {
      const users = await api.adminUsers.list(token, { limit: 200 });
      setItems(users);
      setDraftRoles(
        Object.fromEntries(users.map((item) => [item.id, item.role]))
      );
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить список пользователей");
      }
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    load();
  }, [load]);

  const stats = useMemo(() => {
    const counters: Record<string, number> = {};
    for (const item of items) {
      counters[item.role] = (counters[item.role] || 0) + 1;
    }
    return counters;
  }, [items]);

  async function saveRole(item: AdminUser) {
    if (!token) return;

    const nextRole = draftRoles[item.id] || item.role;
    if (nextRole === item.role) return;

    setSavingUserID(item.id);
    setErr("");

    try {
      const updated = await api.adminUsers.updateRole(token, item.id, nextRole);
      setItems((prev) => prev.map((x) => (x.id === updated.id ? updated : x)));
      setDraftRoles((prev) => ({ ...prev, [updated.id]: updated.role }));

      addNotification({
        kind: "success",
        title: "Роль пользователя обновлена",
        message: `${updated.email}: ${roleLabel(updated.role)}`,
      });
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else if (e?.code === "self_role_change_forbidden") {
        setErr("Нельзя изменить собственную роль администратора.");
      } else {
        setErr(e?.message || "Не удалось изменить роль пользователя");
      }
    } finally {
      setSavingUserID("");
    }
  }

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 10,
            alignItems: "baseline",
            flexWrap: "wrap",
          }}
        >
          <h2 style={{ margin: 0 }}>Управление пользователями и ролями</h2>

          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/dashboard/admin" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К дашборду</button>
            </Link>
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
          </div>
        </div>

        <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Badge text={`Всего: ${items.length}`} />
          <Badge text={`Администраторы: ${stats.admin || 0}`} />
          <Badge text={`Исследователи: ${stats.researcher || 0}`} />
          <Badge text={`Голосующие: ${stats.voter || 0}`} />
        </div>
      </div>

      <div style={styles.card}>
        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {!loading && items.length === 0 ? (
          <div style={styles.muted}>Пользователи не найдены</div>
        ) : null}

        <div style={{ display: "grid", gap: 10 }}>
          {items.map((item) => {
            const draftRole = draftRoles[item.id] || item.role;
            const changed = draftRole !== item.role;
            const isSelf = me?.id === item.id;

            return (
              <div key={item.id} style={styles.card}>
                <div
                  style={{
                    display: "grid",
                    gap: 10,
                    gridTemplateColumns: "minmax(260px, 1.5fr) minmax(220px, 1fr) auto",
                    alignItems: "end",
                  }}
                >
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontWeight: 700 }}>{item.full_name || item.email}</div>
                    <div style={styles.muted}>{item.email}</div>
                    <div style={{ marginTop: 6, display: "flex", gap: 8, flexWrap: "wrap" }}>
                      <Badge text={`Текущая роль: ${roleLabel(item.role)}`} />
                      {item.phone ? <Badge text={`Телефон: ${item.phone}`} /> : null}
                      {isSelf ? <Badge text="Это вы" /> : null}
                    </div>
                    <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                      Создан: {item.created_at}
                    </div>
                  </div>

                  <div>
                    <label>Новая роль</label>
                    <select
                      style={styles.input}
                      value={draftRole}
                      onChange={(e) =>
                        setDraftRoles((prev) => ({ ...prev, [item.id]: e.target.value }))
                      }
                      disabled={savingUserID === item.id}
                    >
                      {ROLE_OPTIONS.map((opt) => (
                        <option key={opt.value} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <button
                      style={styles.btnPrimary}
                      onClick={() => saveRole(item)}
                      disabled={!changed || savingUserID === item.id}
                    >
                      {savingUserID === item.id ? "Сохранение…" : "Сохранить"}
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}