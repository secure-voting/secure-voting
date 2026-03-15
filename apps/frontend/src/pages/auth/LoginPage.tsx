import React, { useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { styles } from "../../shared/ui/styles";
import { useAuth } from "../../app/auth";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

type FieldErrors = {
  email?: string;
  password?: string;
  inviteCode?: string;
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
  const loc = useLocation() as any;
  const { setToken, authed } = useAuth();
  const { addNotification } = useNotifications();

  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [inviteCode, setInviteCode] = useState("");

  const [showPass, setShowPass] = useState(false);
  const [role, setRole] = useState<"voter" | "researcher" | "admin">("voter");

  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<unknown>(null);

  useEffect(() => {
    if (authed) nav("/elections", { replace: true });
  }, [authed, nav]);

  useEffect(() => {
    setErr(null);
    setRawResp(null);
    setFieldErrors({});
  }, [mode]);

  const roleDescription = useMemo(() => {
    if (role === "voter") return "Участник голосований";
    if (role === "researcher") return "Пользователь для экспериментов и наборов данных";
    return "Роль администратора должна использоваться только в рамках настроенного контура доступа";
  }, [role]);

  const submit = async () => {
    setLoading(true);
    setErr(null);
    setRawResp(null);

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
        const t = await api.auth.register(normalizedEmail, password, role, normalizedInviteCode);
        if (IS_DEV) setRawResp({ ok: true, mode: "register" });
        setToken(t);
        addNotification({
          kind: "success",
          title: "Регистрация завершена",
          message: `Пользователь ${normalizedEmail} успешно зарегистрирован`,
        });
      } else {
        const t = await api.auth.login(normalizedEmail, password, normalizedInviteCode);
        if (IS_DEV) setRawResp({ ok: true, mode: "login" });
        setToken(t);
        addNotification({
          kind: "success",
          title: "Вход выполнен",
          message: `Выполнен вход для ${normalizedEmail}`,
        });
      }

      const to =
        loc?.state?.from && typeof loc.state.from === "string" ? loc.state.from : "/elections";
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
          <button style={mode === "login" ? styles.btnPrimary : styles.btn} onClick={() => setMode("login")}>
            Вход
          </button>
          <button style={mode === "register" ? styles.btnPrimary : styles.btn} onClick={() => setMode("register")}>
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

        {mode === "register" ? (
          <>
            <div style={{ height: 10 }} />
            <label style={{ display: "block", marginBottom: 6 }}>Роль</label>
            <select style={styles.input} value={role} onChange={(e) => setRole(e.target.value as any)}>
              <option value="voter">voter</option>
              <option value="researcher">researcher</option>
              <option value="admin">admin</option>
            </select>
            <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>{roleDescription}</div>
          </>
        ) : null}

        <div style={{ height: 14 }} />

        <button style={styles.btnPrimary} onClick={submit} disabled={loading}>
          {loading ? "Загрузка…" : mode === "login" ? "Войти" : "Зарегистрироваться"}
        </button>
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug</h3>
          {rawResp ? <JsonBlock value={rawResp} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Информация</h3>
          <div style={{ ...styles.muted, display: "grid", gap: 8 }}>
            <div>• После успешного входа открывается рабочий раздел пользователя.</div>
            <div>• В режиме приглашений может понадобиться код приглашения.</div>
            <div>• Для регистрации исследователя и голосующего используется тот же защищённый интерфейс.</div>
          </div>
        </div>
      )}
    </div>
  );
}