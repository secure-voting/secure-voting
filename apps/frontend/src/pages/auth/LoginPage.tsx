import React, { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";
import { useAuth } from "../../app/auth";

type FieldErrors = {
  email?: string;
  password?: string;
};

type LocationState = {
  from?: string;
};

function isEmailLike(s: string) {
  const v = s.trim();
  if (!v) return false;
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v);
}

function validateAuthFields(input: {
  mode: "login" | "register";
  email: string;
  password: string;
}): FieldErrors {
  const errors: FieldErrors = {};

  if (!isEmailLike(input.email)) {
    errors.email = "Введите корректный адрес электронной почты";
  }

  if (!input.password) {
    errors.password = "Введите пароль";
  } else if (input.mode === "register" && input.password.length < 8) {
    errors.password = "Пароль должен содержать не менее 8 символов";
  }

  return errors;
}

function fieldErrorText(v?: string) {
  return v ? (
    <div style={{ color: "#b91c1c", fontSize: 12, marginTop: 6 }}>{v}</div>
  ) : null;
}

function isActiveSessionConflict(e: unknown) {
  const err = e as { status?: number; code?: string } | null;
  return err?.status === 409 && err?.code === "active_session_exists";
}

export function LoginPage() {
  const nav = useNavigate();
  const loc = useLocation() as { state?: LocationState };
  const { setToken, authed } = useAuth();
  const { addNotification } = useNotifications();

  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const [showPass, setShowPass] = useState(false);

  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [activeSessionWarning, setActiveSessionWarning] = useState(false);

  useEffect(() => {
    if (authed) nav("/dashboard", { replace: true });
  }, [authed, nav]);

  useEffect(() => {
    setErr(null);
    setFieldErrors({});
    setActiveSessionWarning(false);
  }, [mode]);

  useEffect(() => {
    setActiveSessionWarning(false);
  }, [email, password]);

  const finishAuth = (normalizedEmail: string) => {
    const to =
      loc?.state?.from && typeof loc.state.from === "string" ? loc.state.from : "/dashboard";

    addNotification({
      kind: "success",
      title: mode === "register" ? "Регистрация завершена" : "Вход выполнен",
      message:
        mode === "register"
          ? `Учётная запись для ${normalizedEmail} успешно создана`
          : `Выполнен вход для ${normalizedEmail}`,
    });

    nav(to, { replace: true });
  };

  const submit = async (replaceExistingSession = false) => {
    setLoading(true);
    setErr(null);

    try {
      const nextErrors = validateAuthFields({
        mode,
        email,
        password,
      });

      setFieldErrors(nextErrors);

      if (Object.keys(nextErrors).length > 0) {
        throw new Error("Исправьте ошибки в форме");
      }

      const normalizedEmail = email.trim();

      if (mode === "register") {
        const tokens = await api.auth.register(normalizedEmail, password);
        setToken(tokens);
        finishAuth(normalizedEmail);
        return;
      }

      const tokens = await api.auth.login(
        normalizedEmail,
        password,
        replaceExistingSession
      );

      setToken(tokens);
      setActiveSessionWarning(false);
      finishAuth(normalizedEmail);
    } catch (e: any) {
      if (mode === "login" && isActiveSessionConflict(e)) {
        setErr(null);
        setActiveSessionWarning(true);
        return;
      }

      setErr(e?.message || "Ошибка авторизации");
    } finally {
      setLoading(false);
    }
  };

  const confirmReplaceSession = async () => {
    await submit(true);
  };

  const cancelReplaceSession = () => {
    setActiveSessionWarning(false);
    setErr(null);
  };

  return (
    <div style={styles.grid2}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>{mode === "login" ? "Вход в систему" : "Регистрация"}</h2>

        <div style={{ display: "flex", gap: 8, marginBottom: 12 }}>
          <button
            type="button"
            style={mode === "login" ? styles.btnPrimary : styles.btn}
            onClick={() => setMode("login")}
          >
            Вход
          </button>
          <button
            type="button"
            style={mode === "register" ? styles.btnPrimary : styles.btn}
            onClick={() => setMode("register")}
          >
            Регистрация
          </button>
        </div>

        <ErrorBanner error={err} />

        {activeSessionWarning ? (
          <div
            style={{
              ...styles.card,
              background: "#fff7ed",
              borderColor: "#fed7aa",
              marginBottom: 12,
            }}
          >
            <div style={{ fontWeight: 700 }}>Обнаружена активная сессия</div>
            <div style={{ marginTop: 6, ...styles.muted }}>
              Для этой учётной записи уже выполнен вход в другом окне, браузере или на другом
              устройстве. Можно продолжить вход здесь, тогда предыдущая сессия будет завершена.
            </div>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 12 }}>
              <button
                type="button"
                style={styles.btnDanger}
                onClick={confirmReplaceSession}
                disabled={loading}
              >
                Завершить предыдущую сессию и войти
              </button>
              <button
                type="button"
                style={styles.btn}
                onClick={cancelReplaceSession}
                disabled={loading}
              >
                Отмена
              </button>
            </div>
          </div>
        ) : null}

        <label style={{ display: "block", marginBottom: 6 }}>Email</label>
        <input
          style={styles.input}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          autoComplete="email"
          placeholder="user@example.com"
        />
        {fieldErrorText(fieldErrors.email)}

        <div style={{ height: 10 }} />

        <label style={{ display: "block", marginBottom: 6 }}>Пароль</label>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <input
            style={styles.input}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type={showPass ? "text" : "password"}
            autoComplete={mode === "login" ? "current-password" : "new-password"}
            placeholder={mode === "register" ? "Не менее 8 символов" : "Введите пароль"}
          />
          <button style={styles.btn} onClick={() => setShowPass((v) => !v)} type="button">
            {showPass ? "Скрыть" : "Показать"}
          </button>
        </div>
        {fieldErrorText(fieldErrors.password)}

        <div style={{ height: 10 }} />

        <button style={styles.btnPrimary} onClick={() => submit(false)} disabled={loading} type="button">
          {loading ? "Загрузка…" : mode === "login" ? "Войти" : "Зарегистрироваться"}
        </button>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Информация</h3>
        <div style={{ ...styles.muted, display: "grid", gap: 8 }}>
          <div>• После успешного входа открывается рабочий раздел пользователя.</div>
          <div>• Код приглашения вводится в разделе голосований после входа в систему.</div>
          <div>• Самостоятельная регистрация создаёт учётную запись голосующего.</div>
          <div>• Учётные записи администратора и исследователя настраиваются отдельно.</div>
          <div>• При входе с нового устройства предыдущую активную сессию можно завершить.</div>
        </div>
      </div>
    </div>
  );
}