import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionSummary } from "../../shared/api/types";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { Badge } from "../../shared/ui/Badge";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { useAuth } from "../../app/auth";
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

function formatDateTime(value: unknown) {
  if (typeof value !== "string" || !value.trim()) return "—";

  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;

  return d.toLocaleString("ru-RU", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
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
                    { label: "Начало", value: e.start_at },
                    { label: "Окончание", value: e.end_at },
                    { label: "Опубликовано", value: e.published_at ?? "—" },
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