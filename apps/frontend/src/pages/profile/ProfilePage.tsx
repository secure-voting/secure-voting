import React, { useMemo, useState } from "react";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { api, ApiError } from "../../shared/api/client";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";

function roleLabel(role?: string) {
  if (role === "admin") return "Администратор";
  if (role === "researcher") return "Исследователь";
  if (role === "voter") return "Голосующий";
  return role || "—";
}

function kindLabel(kind?: string) {
  if (kind === "success") return "Успех";
  if (kind === "warning") return "Предупреждение";
  if (kind === "error") return "Ошибка";
  return kind || "Уведомление";
}

export function ProfilePage() {
  const { me, authed, token } = useAuth();
  const { unreadCount, items } = useNotifications();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string>("");
  const [formSuccess, setFormSuccess] = useState<string>("");

  const lastItems = items.slice(0, 5);

  const canSubmit = useMemo(() => {
    return (
      !submitting &&
      !!token &&
      currentPassword.trim().length > 0 &&
      newPassword.trim().length >= 8 &&
      confirmPassword.trim().length > 0
    );
  }, [submitting, token, currentPassword, newPassword, confirmPassword]);

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setFormSuccess("");

    if (!token) {
      setFormError("Сессия недействительна. Выполните вход заново.");
      return;
    }
    if (!currentPassword.trim()) {
      setFormError("Введите текущий пароль.");
      return;
    }
    if (newPassword.trim().length < 8) {
      setFormError("Новый пароль должен содержать не менее 8 символов.");
      return;
    }
    if (newPassword !== confirmPassword) {
      setFormError("Подтверждение пароля не совпадает.");
      return;
    }

    setSubmitting(true);
    try {
      await api.auth.changePassword(token, currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setFormSuccess("Новый пароль успешно сохранен.");
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "invalid_current_password") {
          setFormError("Текущий пароль указан неверно.");
        } else if (err.code === "invalid_password") {
          setFormError("Новый пароль должен содержать не менее 8 символов.");
        } else if (err.code === "password_unchanged") {
          setFormError("Новый пароль должен отличаться от текущего.");
        } else {
          setFormError(err.message || "Не удалось изменить пароль.");
        }
      } else {
        setFormError("Не удалось изменить пароль. Повторите попытку позже.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>Профиль пользователя</h2>

        <div style={{ marginBottom: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          {me?.role ? <Badge text={`Роль: ${roleLabel(me.role)}`} /> : null}
          <Badge text={authed ? "Сессия активна" : "Сессия неактивна"} />
          <Badge text={`Непрочитанных уведомлений: ${unreadCount}`} />
        </div>

        <KeyValueList
          items={[
            { label: "ID пользователя", value: me?.id || "—" },
            { label: "Электронная почта", value: me?.email || "—" },
            { label: "Роль", value: roleLabel(me?.role) },
          ]}
        />
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Смена пароля</h3>

        <form onSubmit={onChangePassword} style={{ display: "grid", gap: 10, maxWidth: 480 }}>
          <label style={{ display: "grid", gap: 6 }}>
            <span>Текущий пароль</span>
            <input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              style={styles.input}
              autoComplete="current-password"
            />
          </label>

          <label style={{ display: "grid", gap: 6 }}>
            <span>Новый пароль</span>
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              style={styles.input}
              autoComplete="new-password"
            />
          </label>

          <label style={{ display: "grid", gap: 6 }}>
            <span>Подтвердите новый пароль</span>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              style={styles.input}
              autoComplete="new-password"
            />
          </label>

          {formError ? (
            <div style={{ color: "#b91c1c", fontSize: 14 }}>{formError}</div>
          ) : formSuccess ? (
            <div style={{ color: "#15803d", fontSize: 14 }}>{formSuccess}</div>
          ) : (
            <div style={styles.muted}>Минимальная длина нового пароля: 8 символов.</div>
          )}

          <div>
            <button type="submit" disabled={!canSubmit} style={styles.buttonPrimary}>
              {submitting ? "Сохранение..." : "Сменить пароль"}
            </button>
          </div>
        </form>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Последние уведомления</h3>

        {lastItems.length === 0 ? (
          <div style={styles.muted}>Пока уведомлений нет</div>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {lastItems.map((item) => (
              <div
                key={item.id}
                style={{
                  ...styles.card,
                  padding: 10,
                  background: item.read ? "white" : "#f9fafb",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <div>
                    <div style={{ fontWeight: 700 }}>{item.title}</div>
                    <div style={styles.muted}>{item.message}</div>
                  </div>
                  <Badge text={kindLabel(item.kind)} />
                </div>
                <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>{item.created_at}</div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}