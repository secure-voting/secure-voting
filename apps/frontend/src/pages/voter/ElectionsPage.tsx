import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionSummary } from "../../shared/api/types";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { Badge } from "../../shared/ui/Badge";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { useAuth } from "../../app/auth";
import { formatDateTime } from "../../shared/utils/dateTime";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

function statusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    draft: "Черновик",
    scheduled: "Запланировано",
    active: "Открыто",
    paused: "Приостановлено",
    closed: "Завершено",
    results_ready: "Результаты готовы",
    published: "Опубликовано",
  };

  return labels[raw] || raw || "Статус неизвестен";
}

function accessModeLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    open: "Открытый доступ",
    invite: "По приглашениям",
  };

  return labels[raw] || raw || "Доступ не указан";
}

function ballotFormatLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    approval: "Approval",
    ranking: "Ranking",
    score: "Score",
  };

  return labels[raw] || raw || "Формат не указан";
}

function electionSubtitle(e: ElectionSummary) {
  const parts: string[] = [];

  if (e.ballot_format) {
    parts.push(ballotFormatLabel(e.ballot_format));
  }

  if (e.tally_rule) {
    parts.push(tallyRuleLabel(e.tally_rule));
  }

  if (typeof e.candidate_count === "number") {
    parts.push(`${e.candidate_count} кандидатов`);
  }

  return parts.length > 0 ? parts.join(" · ") : "Параметры голосования";
}

export function ElectionsPage() {
  const { token, me, setToken } = useAuth();
  const [items, setItems] = useState<ElectionSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const nav = useNavigate();
  const [inviteCode, setInviteCode] = useState("");
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteErr, setInviteErr] = useState<string | null>(null);
  const [inviteInfo, setInviteInfo] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const list = await api.elections.list(token, ac.signal);
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить список голосований");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  const isVoter = me?.role === "voter";
  const isAdmin = me?.role === "admin";

  const acceptInvite = async () => {
    if (!token) return;

    const code = inviteCode.trim();
    setInviteErr(null);
    setInviteInfo(null);

    if (code.length < 3) {
      setInviteErr("Введите корректный код приглашения");
      return;
    }

    setInviteLoading(true);

    try {
      const res = await api.auth.acceptInvite(token, code);

      setInviteCode("");
      setInviteInfo("Приглашение принято. Открываем страницу голосования.");

      if (res.election_id) {
        nav(`/elections/${res.election_id}/vote`);
        return;
      }
      await reload();
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
        return;
      }

      if (e?.code === "invite_email_mismatch") {
        setInviteErr("Код приглашения выписан на другой email");
      } else if (e?.code === "invite_code_inactive") {
        setInviteErr("Код приглашения уже использован или больше не активен");
      } else if (e?.code === "invalid_invite_code") {
        setInviteErr("Код приглашения не найден");
      } else {
        setInviteErr(e?.message || "Не удалось принять приглашение");
      }
    } finally {
      setInviteLoading(false);
    }
  };

  return (
    <div style={styles.card}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 10,
          flexWrap: "wrap",
        }}
      >
        <h2 style={{ margin: 0 }}>Список голосований</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button style={styles.btn} onClick={reload} disabled={loading}>
            Обновить
          </button>
        </div>
      </div>

      {isVoter ? (
        <div
          style={{
            ...styles.card,
            marginTop: 12,
            background: "#f9fafb",
          }}
        >
          <div style={{ fontWeight: 800 }}>Доступ по приглашению</div>
          <div style={{ marginTop: 6, ...styles.muted }}>
            Введите код приглашения, чтобы получить доступ к закрытому голосованию.
          </div>

          {inviteErr ? (
            <div style={{ color: "#b91c1c", marginTop: 10 }}>{inviteErr}</div>
          ) : null}

          {inviteInfo ? (
            <div style={{ color: "#166534", marginTop: 10 }}>{inviteInfo}</div>
          ) : null}

          <div
            style={{
              marginTop: 10,
              display: "grid",
              gridTemplateColumns: "minmax(220px, 1fr) auto",
              gap: 8,
              alignItems: "center",
            }}
          >
            <input
              style={styles.input}
              value={inviteCode}
              onChange={(e) => setInviteCode(e.target.value)}
              autoComplete="one-time-code"
              placeholder="Код приглашения"
              disabled={inviteLoading}
            />
            <button
              style={styles.btnPrimary}
              onClick={acceptInvite}
              disabled={inviteLoading}
              type="button"
            >
              {inviteLoading ? "Проверка…" : "Перейти к голосованию"}
            </button>
          </div>
        </div>
      ) : null}

      <ErrorBanner error={err} />

      {loading ? <div style={{ marginTop: 10, ...styles.muted }}>Загрузка…</div> : null}
      {!loading && items.length === 0 ? (
        <div style={{ marginTop: 10, ...styles.muted }}>Нет голосований</div>
      ) : null}

      <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
        {items.map((e) => (
          <div key={e.id} style={{ ...styles.card, padding: 12 }}>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                gap: 10,
                alignItems: "flex-start",
                flexWrap: "wrap",
              }}
            >
              <div>
                <div style={{ fontWeight: 800, fontSize: 16 }}>{e.title}</div>
                <div style={{ ...styles.muted, marginTop: 4 }}>
                  {e.description || electionSubtitle(e)}
                </div>
              </div>

              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <Badge text={statusLabel(e.status)} />
                <Badge text={accessModeLabel(e.access_mode)} />
              </div>
            </div>

            <div style={{ marginTop: 10 }}>
              <SummaryGrid
                items={[
                  { label: "Формат бюллетеня", value: ballotFormatLabel(e.ballot_format) },
                  { label: "Правило подсчета", value: tallyRuleLabel(e.tally_rule) },
                  {
                    label: "Кандидаты",
                    value: typeof e.candidate_count === "number" ? String(e.candidate_count) : "—",
                  },
                  { label: "Организатор", value: e.organizer_email ?? "—" },
                  { label: "Начало", value: formatDateTime(e.start_at) },
                  { label: "Окончание", value: formatDateTime(e.end_at) },
                  { label: "Публикация", value: formatDateTime(e.published_at) },
                ]}
              />
            </div>

            <details style={{ marginTop: 10 }}>
              <summary style={{ cursor: "pointer", ...styles.muted }}>
                Технические сведения
              </summary>
              <div style={{ marginTop: 8 }}>
                <KeyValueList
                  items={[
                    { label: "ID голосования", value: e.id },
                    { label: "Статус", value: e.status },
                    { label: "Режим доступа", value: e.access_mode },
                    { label: "Формат бюллетеня", value: e.ballot_format ?? "—" },
                    { label: "Правило подсчета", value: e.tally_rule ?? "—" },
                    { label: "Начало", value: formatDateTime(e.start_at) },
                    { label: "Окончание", value: formatDateTime(e.end_at) },
                    { label: "Опубликовано", value: formatDateTime(e.published_at) },
                  ]}
                />
              </div>
            </details>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Link
                to={isAdmin ? `/admin/elections/${e.id}` : `/elections/${e.id}`}
                style={{ textDecoration: "none" }}
              >
                <button style={styles.btnPrimary}>{isAdmin ? "Управление" : "Открыть"}</button>
              </Link>

              {isAdmin ? (
                <Link to={`/admin/elections/${e.id}/rules`} style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Настройки</button>
                </Link>
              ) : null}

              {isVoter ? (
                <Link to={`/elections/${e.id}/vote`} style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Голосовать</button>
                </Link>
              ) : null}

              <Link to={`/elections/${e.id}/results`} style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Результаты</button>
              </Link>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}