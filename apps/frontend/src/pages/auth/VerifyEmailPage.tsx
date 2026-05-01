import React, { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useAuth } from "../../app/auth";
import { api, ApiError } from "../../shared/api/client";
import { Badge } from "../../shared/ui/Badge";
import { styles } from "../../shared/ui/styles";

function verificationErrorText(err: unknown) {
  if (err instanceof ApiError) {
    if (err.code === "invalid_verification_token") {
      return "Ссылка подтверждения недействительна.";
    }
    if (err.code === "verification_token_used") {
      return "Эта ссылка уже была использована.";
    }
    if (err.code === "verification_token_expired") {
      return "Срок действия ссылки подтверждения истек.";
    }
    return err.message || "Не удалось подтвердить почту.";
  }

  return "Не удалось подтвердить почту. Повторите попытку позже.";
}

export function VerifyEmailPage() {
  const [params] = useSearchParams();
  const { updateMe, authed } = useAuth();

  const token = useMemo(() => (params.get("token") || "").trim(), [params]);

  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState("");
  const [error, setError] = useState("");

  async function onConfirm() {
    setSuccess("");
    setError("");

    if (!token) {
      setError("В ссылке отсутствует токен подтверждения.");
      return;
    }

    setSubmitting(true);
    try {
      const updated = await api.auth.confirmEmailVerification(token);
      if (authed) {
        updateMe(updated);
      }
      setSuccess("Почта успешно подтверждена.");
    } catch (err) {
      setError(verificationErrorText(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={{ ...styles.card, maxWidth: 720 }}>
        <h2 style={{ marginTop: 0 }}>Подтверждение почты</h2>

        <div style={{ marginBottom: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Badge text={success ? "Почта подтверждена" : "Ожидает подтверждения"} />
        </div>

        <p style={styles.muted}>
          Нажмите кнопку ниже, чтобы завершить подтверждение адреса электронной почты.
        </p>

        {!token ? (
          <div style={{ color: "#b91c1c", fontSize: 14 }}>
            В адресной строке нет токена подтверждения.
          </div>
        ) : null}

        {error ? (
          <div style={{ marginTop: 10, color: "#b91c1c", fontSize: 14 }}>{error}</div>
        ) : null}

        {success ? (
          <div style={{ marginTop: 10, color: "#15803d", fontSize: 14 }}>{success}</div>
        ) : null}

        <div style={{ marginTop: 16, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button
            type="button"
            style={styles.buttonPrimary}
            onClick={onConfirm}
            disabled={submitting || !token || Boolean(success)}
          >
            {submitting ? "Подтверждение..." : "Подтвердить почту"}
          </button>

          <Link to="/profile" style={styles.button}>
            Перейти в профиль
          </Link>
        </div>
      </div>
    </div>
  );
}