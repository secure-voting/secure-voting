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
  inviteCode?: string;
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
  inviteCode: string;
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

  if (input.inviteCode.trim() && input.inviteCode.trim().length < 3) {
    errors.inviteCode = "Код приглашения выглядит некорректно";
  }

  return errors;
}

function fieldErrorText(v?: string) {
  return v ? (
    <div style={{ color: "#b91c1c", fontSize: 12, marginTop: 6 }}>{v}</div>
  ) : null;
}

export function LoginPage() {
  const nav = useNavigate();
  const loc = useLocation() as { state?: LocationState };
  const { setToken, authed } = useAuth();
  const { addNotification } = useNotifications();

  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [inviteCode, setInviteCode] = useState("");

  const [showPass, setShowPass] = useState(false);

  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (authed) nav("/dashboard", { replace: true });
  }, [authed, nav]);

  useEffect(() => {
    setErr(null);
    setFieldErrors({});
  }, [mode]);

  const submit = async () => {
    setLoading(true);
    setErr(null);

    try {
      const nextErrors = validateAuthFields({
        mode,
        email,
        password,
        inviteCode,
      });

      setFieldErrors(nextErrors);

      if (Object.keys(nextErrors).length > 0) {
        throw new Error("Исправьте ошибки в форме");
      }

      const normalizedEmail = email.trim();
      const normalizedInviteCode = inviteCode.trim() ? inviteCode.trim() : null;

      if (mode === "register") {
        const t = await api.auth.register(normalizedEmail, password, normalizedInviteCode);
        setToken(t);
        addNotification({
          kind: "success",
          title: "Регистрация завершена",
          message: `Учётная запись для ${normalizedEmail} успешно создана`,
        });
      } else {
        const t = await api.auth.login(normalizedEmail, password, normalizedInviteCode);
        setToken(t);
        addNotification({
          kind: "success",
          title: "Вход выполнен",
          message: `Выполнен вход для ${normalizedEmail}`,
        });
      }

      const to =
        loc?.state?.from && typeof loc.state.from === "string" ? loc.state.from : "/dashboard";
      nav(to, { replace: true });
    } catch (e: any) {
      setErr(e?.message || "Ошибка авторизации");
    } finally {
      setLoading(false);
    }
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

        <label style={{ display: "block", marginBottom: 6 }}>
          Код приглашения <span style={styles.muted}>(если требуется)</span>
        </label>
        <input
          style={styles.input}
          value={inviteCode}
          onChange={(e) => setInviteCode(e.target.value)}
          autoComplete="one-time-code"
          placeholder="Введите код приглашения"
        />
        {fieldErrorText(fieldErrors.inviteCode)}

        <div style={{ height: 14 }} />

        <button style={styles.btnPrimary} onClick={submit} disabled={loading} type="button">
          {loading ? "Загрузка…" : mode === "login" ? "Войти" : "Зарегистрироваться"}
        </button>
      </div>

      
      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Информация</h3>
        <div style={{ ...styles.muted, display: "grid", gap: 8 }}>
          <div>• После успешного входа открывается рабочий раздел пользователя.</div>
          <div>• В голосованиях с доступом по приглашению может понадобиться код приглашения.</div>
          <div>• Самостоятельная регистрация создаёт учётную запись голосующего.</div>
          <div>• Учётные записи администратора и исследователя настраиваются отдельно.</div>
        </div>
      </div>
    </div>
  );
}