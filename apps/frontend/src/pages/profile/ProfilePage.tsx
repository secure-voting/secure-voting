import React, { useEffect, useMemo, useState } from "react";
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
  const { me, authed, token, updateMe } = useAuth();
  const { unreadCount, items } = useNotifications();

  const [fullName, setFullName] = useState("");
  const [phone, setPhone] = useState("");

  const [profileSubmitting, setProfileSubmitting] = useState(false);
  const [profileError, setProfileError] = useState("");
  const [profileSuccess, setProfileSuccess] = useState("");
  const [savedFullName, setSavedFullName] = useState("");
  const [savedPhone, setSavedPhone] = useState("");

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string>("");
  const [formSuccess, setFormSuccess] = useState<string>("");

  useEffect(() => {
    const nextFullName = (me?.full_name || "").trim();
    const nextPhone = (me?.phone || "").trim();
    setFullName(nextFullName);
    setPhone(nextPhone);
    setSavedFullName(nextFullName);
    setSavedPhone(nextPhone);
  }, [me?.full_name, me?.phone]);

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

  const canSaveProfile = useMemo(() => {
    return (
      !!token &&
      !profileSubmitting &&
      (fullName.trim() !== savedFullName || phone.trim() !== savedPhone)
    );
  }, [token, profileSubmitting, fullName, phone, savedFullName, savedPhone]);

  async function onSaveProfile(e: React.FormEvent) {
    e.preventDefault();
    setProfileError("");
    setProfileSuccess("");

    if (!token) {
      setProfileError("Сессия недействительна. Выполните вход заново.");
      return;
    }
    if (fullName.trim().length > 120) {
      setProfileError("Поле ФИО не должно превышать 120 символов.");
      return;
    }
    if (phone.trim() && !/^\+?[0-9 ()-]{5,32}$/.test(phone.trim())) {
      setProfileError("Укажите телефон в корректном формате.");
      return;
    }

    setProfileSubmitting(true);
    try {
      const updated = await api.auth.updateProfile(token, fullName, phone);
      const nextFullName = (updated.full_name || "").trim();
      const nextPhone = (updated.phone || "").trim();

      setFullName(nextFullName);
      setPhone(nextPhone);
      setSavedFullName(nextFullName);
      setSavedPhone(nextPhone);
      updateMe(updated);
      setProfileSuccess("Контактные данные успешно сохранены.");
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "invalid_full_name") {
          setProfileError("Поле ФИО не должно превышать 120 символов.");
        } else if (err.code === "invalid_phone") {
          setProfileError("Укажите телефон в корректном формате.");
        } else {
          setProfileError(err.message || "Не удалось сохранить контактные данные.");
        }
      } else {
        setProfileError("Не удалось сохранить контактные данные. Повторите попытку позже.");
      }
    } finally {
      setProfileSubmitting(false);
    }
  }

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
          <Badge text={me?.email_verified ? "Почта подтверждена" : "Почта не подтверждена"} />
          <Badge text={`Непрочитанных уведомлений: ${unreadCount}`} />
        </div>

        <KeyValueList
          items={[
            { label: "ID пользователя", value: me?.id || "—" },
            { label: "Электронная почта", value: me?.email || "—" },
            {
              label: "Статус почты",
              value: me?.email_verified
                ? `Подтверждена${me.email_verified_at ? `: ${me.email_verified_at}` : ""}`
                : "Не подтверждена",
            },
            { label: "Роль", value: roleLabel(me?.role) },
            { label: "ФИО", value: savedFullName || "—" },
            { label: "Телефон", value: savedPhone || "—" },
          ]}
        />
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Контактные данные</h3>

        <form onSubmit={onSaveProfile} style={{ display: "grid", gap: 10, maxWidth: 480 }}>
          <label style={{ display: "grid", gap: 6 }}>
            <span>ФИО</span>
            <input
              type="text"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              style={styles.input}
              maxLength={120}
            />
          </label>

          <label style={{ display: "grid", gap: 6 }}>
            <span>Телефон</span>
            <input
              type="text"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              style={styles.input}
              placeholder="+7 (999) 000-00-00"
            />
          </label>

          {profileError ? (
            <div style={{ color: "#b91c1c", fontSize: 14 }}>{profileError}</div>
          ) : profileSuccess ? (
            <div style={{ color: "#15803d", fontSize: 14 }}>{profileSuccess}</div>
          ) : (
            <div style={styles.muted}>ФИО и телефон можно оставить пустыми.</div>
          )}

          <div>
            <button type="submit" disabled={!canSaveProfile} style={styles.buttonPrimary}>
              {profileSubmitting ? "Сохранение..." : "Сохранить контактные данные"}
            </button>
          </div>
        </form>
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