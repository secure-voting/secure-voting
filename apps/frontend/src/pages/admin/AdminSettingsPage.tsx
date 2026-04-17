import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { api, ApiError } from "../../shared/api/client";
import type { AdminSettings } from "../../shared/api/types";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

const emptySettings: AdminSettings = {
  tls_mode: "disabled",
  backup_enabled: false,
  has_unsaved_warning: true,
};

function toText(v?: string | null) {
  return v || "";
}

function toNumberOrNull(v: string) {
  const s = v.trim();
  if (!s) return null;
  const n = Number(s);
  return Number.isFinite(n) ? n : null;
}

export function AdminSettingsPage() {
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [item, setItem] = useState<AdminSettings>(emptySettings);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");

  const [publicBaseURL, setPublicBaseURL] = useState("");
  const [tlsDomain, setTLSDomain] = useState("");
  const [tlsContactEmail, setTLSContactEmail] = useState("");
  const [backupSchedule, setBackupSchedule] = useState("");
  const [backupRetentionDays, setBackupRetentionDays] = useState("");
  const [databaseHost, setDatabaseHost] = useState("");
  const [databaseName, setDatabaseName] = useState("");

  useEffect(() => {
    if (!token) return;

    setLoading(true);
    setErr("");

    api.adminSettings
      .get(token)
      .then((data) => {
        setItem(data);
        setPublicBaseURL(toText(data.public_base_url));
        setTLSDomain(toText(data.tls_domain));
        setTLSContactEmail(toText(data.tls_contact_email));
        setBackupSchedule(toText(data.backup_schedule));
        setBackupRetentionDays(data.backup_retention_days ? String(data.backup_retention_days) : "");
        setDatabaseHost(toText(data.database_host));
        setDatabaseName(toText(data.database_name));
      })
      .catch((e: any) => {
        if (e?.status === 401) {
          setToken(null);
        } else {
          setErr(e?.message || "Не удалось загрузить настройки.");
        }
      })
      .finally(() => setLoading(false));
  }, [token, setToken]);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    if (!token) return;

    setSaving(true);
    setErr("");
    setOk("");

    try {
      const saved = await api.adminSettings.update(token, {
        public_base_url: publicBaseURL,
        tls_mode: item.tls_mode,
        tls_domain: tlsDomain,
        tls_contact_email: tlsContactEmail,
        backup_enabled: item.backup_enabled,
        backup_schedule: backupSchedule,
        backup_retention_days: toNumberOrNull(backupRetentionDays),
        database_host: databaseHost,
        database_name: databaseName,
      });

      setItem(saved);
      setOk("Настройки успешно сохранены.");
      addNotification({
        kind: "success",
        title: "Настройки сохранены",
        message: "Параметры self-hosted установки обновлены.",
      });
    } catch (e) {
      if (e instanceof ApiError) {
        setErr(e.message || "Не удалось сохранить настройки.");
      } else {
        setErr("Не удалось сохранить настройки.");
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 8, flexWrap: "wrap" }}>
          <h2 style={{ margin: 0 }}>Настройки self-hosted развертывания</h2>
          <Link to="/dashboard/admin" style={{ textDecoration: "none" }}>
            <button style={styles.btn}>К дашборду</button>
          </Link>
        </div>
        <div style={{ ...styles.muted, marginTop: 8 }}>
          Этот раздел сохраняет прикладные параметры self-hosted установки. Их фактическое применение может требовать отдельной настройки инфраструктуры.
        </div>
      </div>

      <div style={styles.card}>
        <ErrorBanner error={err} />
        {ok ? <div style={{ color: "#166534", marginBottom: 10 }}>{ok}</div> : null}
        {loading ? <div style={styles.muted}>Загрузка...</div> : null}

        {!loading ? (
          <form onSubmit={onSave} style={{ display: "grid", gap: 12 }}>
            <div style={{ display: "grid", gap: 6 }}>
              <label>Публичный адрес доступа</label>
              <input style={styles.input} value={publicBaseURL} onChange={(e) => setPublicBaseURL(e.target.value)} />
            </div>

            <div style={{ display: "grid", gap: 6 }}>
              <label>Режим TLS / сертификата</label>
              <select
                style={styles.input}
                value={item.tls_mode}
                onChange={(e) => setItem((prev) => ({ ...prev, tls_mode: e.target.value }))}
              >
                <option value="disabled">Отключено</option>
                <option value="lets_encrypt">Let's Encrypt</option>
                <option value="custom">Пользовательский сертификат</option>
              </select>
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <div style={{ display: "grid", gap: 6 }}>
                <label>Домен TLS</label>
                <input style={styles.input} value={tlsDomain} onChange={(e) => setTLSDomain(e.target.value)} />
              </div>
              <div style={{ display: "grid", gap: 6 }}>
                <label>Контактный e-mail TLS</label>
                <input
                  style={styles.input}
                  value={tlsContactEmail}
                  onChange={(e) => setTLSContactEmail(e.target.value)}
                />
              </div>
            </div>

            <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
              <input
                id="backup_enabled"
                type="checkbox"
                checked={item.backup_enabled}
                onChange={(e) => setItem((prev) => ({ ...prev, backup_enabled: e.target.checked }))}
              />
              <label htmlFor="backup_enabled">Включить резервное копирование</label>
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 220px", gap: 12 }}>
              <div style={{ display: "grid", gap: 6 }}>
                <label>Расписание резервного копирования</label>
                <input
                  style={styles.input}
                  value={backupSchedule}
                  onChange={(e) => setBackupSchedule(e.target.value)}
                  placeholder="Например: daily 02:00"
                />
              </div>
              <div style={{ display: "grid", gap: 6 }}>
                <label>Хранить резервные копии, дней</label>
                <input
                  style={styles.input}
                  value={backupRetentionDays}
                  onChange={(e) => setBackupRetentionDays(e.target.value)}
                  placeholder="7"
                />
              </div>
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <div style={{ display: "grid", gap: 6 }}>
                <label>Хост базы данных</label>
                <input style={styles.input} value={databaseHost} onChange={(e) => setDatabaseHost(e.target.value)} />
              </div>
              <div style={{ display: "grid", gap: 6 }}>
                <label>Имя базы данных</label>
                <input style={styles.input} value={databaseName} onChange={(e) => setDatabaseName(e.target.value)} />
              </div>
            </div>

            <div style={{ ...styles.muted, fontSize: 13 }}>
              Перед сохранением проверьте параметры TLS и резервного копирования. Изменения в self-hosted режиме могут требовать отдельного применения на уровне инфраструктуры.
            </div>

            <div>
              <button type="submit" style={styles.btnPrimary} disabled={saving}>
                {saving ? "Сохранение..." : "Сохранить настройки"}
              </button>
            </div>
          </form>
        ) : null}
      </div>
    </div>
  );
}