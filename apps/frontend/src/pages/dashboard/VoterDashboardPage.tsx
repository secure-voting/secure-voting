import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionSummary } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { formatDateTime } from "../../shared/utils/dateTime";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

function displayName(me: ReturnType<typeof useAuth>["me"]) {
  const fullName = typeof me?.full_name === "string" ? me.full_name.trim() : "";
  if (fullName) return fullName;
  return me?.email || "голосующий";
}

function isActiveLike(status: string) {
  return status === "active" || status === "scheduled" || status === "paused";
}

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
    invite: "По приглашению",
  };

  return labels[raw] || raw || "Доступ не указан";
}

function ballotFormatLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    approval: "Одобрение",
    ranking: "Ранжирование",
    score: "Оценивание",
  };

  return labels[raw] || raw || "Формат не указан";
}

function electionSubtitle(item: ElectionSummary) {
  const parts: string[] = [];

  if (item.ballot_format) {
    parts.push(ballotFormatLabel(item.ballot_format));
  }

  if (item.tally_rule) {
    parts.push(tallyRuleLabel(item.tally_rule));
  }

  if (typeof item.candidate_count === "number") {
    parts.push(`${item.candidate_count} кандидатов`);
  }

  return parts.length > 0 ? parts.join(" · ") : "Параметры голосования";
}

export function VoterDashboardPage() {
  const { token, setToken, me } = useAuth();

  const [items, setItems] = useState<ElectionSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const elections = await api.elections.list(token, ac.signal);
      setItems(elections);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить рабочий стол голосующего");
      }
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  const activeItems = useMemo(() => items.filter((item) => isActiveLike(item.status)), [items]);
  const publishedItems = useMemo(() => items.filter((item) => Boolean(item.published_at)), [items]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 12,
            alignItems: "flex-start",
            flexWrap: "wrap",
          }}
        >
          <div>
            <h2 style={{ margin: 0 }}>Рабочий стол голосующего</h2>
            <div style={{ ...styles.muted, marginTop: 4 }}>
              Добро пожаловать, {displayName(me)}. Здесь собраны доступные голосования и опубликованные результаты.
            </div>
          </div>

          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Все голосования</button>
            </Link>
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 14 }}>
          <KeyValueList
            items={[
              { label: "Доступных голосований", value: String(items.length) },
              { label: "Открытых или запланированных", value: String(activeItems.length) },
              { label: "Опубликованных результатов", value: String(publishedItems.length) },
            ]}
          />
        </div>
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Доступные голосования</h3>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && activeItems.length === 0 ? (
            <div style={styles.muted}>Сейчас нет доступных голосований</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {activeItems.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
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
                      <div style={{ fontWeight: 800 }}>{item.title}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        {item.description || electionSubtitle(item)}
                      </div>
                    </div>

                    <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                      <Badge text={statusLabel(item.status)} />
                      <Badge text={accessModeLabel(item.access_mode)} />
                    </div>
                  </div>

                  <div style={{ marginTop: 10 }}>
                    <KeyValueList
                      items={[
                        { label: "Начало", value: formatDateTime(item.start_at) },
                        { label: "Окончание", value: formatDateTime(item.end_at) },
                        { label: "Формат", value: ballotFormatLabel(item.ballot_format) },
                        { label: "Правило", value: tallyRuleLabel(item.tally_rule) },
                      ]}
                    />
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID голосования", value: item.id },
                          { label: "Статус", value: item.status },
                          { label: "Режим доступа", value: item.access_mode },
                          { label: "Начало", value: formatDateTime(item.start_at) },
                          { label: "Окончание", value: formatDateTime(item.end_at) },
                        ]}
                      />
                    </div>
                  </details>

                  <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <Link to={`/elections/${item.id}`} style={{ textDecoration: "none" }}>
                      <button style={styles.btn}>Карточка</button>
                    </Link>
                    <Link to={`/elections/${item.id}/vote`} style={{ textDecoration: "none" }}>
                      <button style={styles.btnPrimary}>Голосовать</button>
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Опубликованные результаты</h3>
          {publishedItems.length === 0 ? (
            <div style={styles.muted}>Пока нет опубликованных результатов</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {publishedItems.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ fontWeight: 800 }}>{item.title}</div>
                  <div style={{ marginTop: 6, ...styles.muted }}>
                    Опубликовано: {formatDateTime(item.published_at)}
                  </div>

                  <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <Badge text={ballotFormatLabel(item.ballot_format)} />
                    <Badge text={tallyRuleLabel(item.tally_rule)} />
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID голосования", value: item.id },
                          { label: "Опубликовано", value: formatDateTime(item.published_at) },
                          { label: "Статус", value: item.status },
                        ]}
                      />
                    </div>
                  </details>

                  <div style={{ marginTop: 10 }}>
                    <Link to={`/elections/${item.id}/results`} style={{ textDecoration: "none" }}>
                      <button style={styles.btnPrimary}>Открыть результаты</button>
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}