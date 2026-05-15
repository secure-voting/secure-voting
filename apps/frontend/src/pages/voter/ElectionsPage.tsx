import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
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

function shortId(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
}

function normalizeSearch(value: unknown) {
  return String(value ?? "").trim().toLowerCase();
}

function dateStartMs(value: string) {
  if (!value.trim()) return null;
  const ms = Date.parse(`${value}T00:00:00`);
  return Number.isFinite(ms) ? ms : null;
}

function dateEndMs(value: string) {
  if (!value.trim()) return null;
  const ms = Date.parse(`${value}T23:59:59.999`);
  return Number.isFinite(ms) ? ms : null;
}

function electionStartMs(value: ElectionSummary) {
  const raw = value.start_at;
  if (typeof raw !== "string" || !raw.trim()) return NaN;
  return Date.parse(raw);
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
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [ballotFormatFilter, setBallotFormatFilter] = useState("");
  const [ruleFilter, setRuleFilter] = useState("");
  const [accessModeFilter, setAccessModeFilter] = useState("");
  const [startFrom, setStartFrom] = useState("");
  const [startTo, setStartTo] = useState("");
  const [publishedOnly, setPublishedOnly] = useState(false);
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

  const ruleOptions = useMemo(() => {
    const map = new Map<string, string>();

    for (const item of items) {
      const rule = typeof item.tally_rule === "string" ? item.tally_rule.trim() : "";
      if (rule) {
        map.set(rule, tallyRuleLabel(rule));
      }
    }

    return Array.from(map.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label, "ru"));
  }, [items]);

  const filteredItems = useMemo(() => {
    const query = normalizeSearch(searchQuery);
    const fromMs = dateStartMs(startFrom);
    const toMs = dateEndMs(startTo);

    return items.filter((item) => {
      const status = String(item.status ?? "").trim();
      const format = String(item.ballot_format ?? "").trim();
      const rule = String(item.tally_rule ?? "").trim();
      const accessMode = String(item.access_mode ?? "").trim();
      const startMs = electionStartMs(item);

      if (publishedOnly && status !== "published") return false;
      if (statusFilter && status !== statusFilter) return false;
      if (ballotFormatFilter && format !== ballotFormatFilter) return false;
      if (ruleFilter && rule !== ruleFilter) return false;
      if (accessModeFilter && accessMode !== accessModeFilter) return false;

      if (fromMs != null && (!Number.isFinite(startMs) || startMs < fromMs)) return false;
      if (toMs != null && (!Number.isFinite(startMs) || startMs > toMs)) return false;

      if (query) {
        const text = [
          item.id,
          shortId(item.id),
          item.title,
          item.description,
          status,
          statusLabel(status),
          format,
          ballotFormatLabel(format),
          rule,
          tallyRuleLabel(rule),
          accessMode,
          accessModeLabel(accessMode),
          item.organizer_email,
        ]
          .map(normalizeSearch)
          .join(" ");

        if (!text.includes(query)) return false;
      }

      return true;
    });
  }, [
    items,
    searchQuery,
    statusFilter,
    ballotFormatFilter,
    ruleFilter,
    accessModeFilter,
    startFrom,
    startTo,
    publishedOnly,
  ]);

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

      <div style={{ marginTop: 12, ...styles.card, background: "#f9fafb" }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Фильтры голосований</div>

        <div style={styles.grid2}>
          <div>
            <label>Поиск</label>
            <input
              style={styles.input}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Название, правило, ID, организатор"
            />
          </div>

          <div>
            <label>Статус</label>
            <select
              style={styles.input}
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
            >
              <option value="">Все статусы</option>
              <option value="draft">Черновик</option>
              <option value="scheduled">Запланировано</option>
              <option value="active">Открыто</option>
              <option value="paused">Приостановлено</option>
              <option value="closed">Завершено</option>
              <option value="results_ready">Результаты готовы</option>
              <option value="published">Опубликовано</option>
            </select>
          </div>

          <div>
            <label>Формат бюллетеня</label>
            <select
              style={styles.input}
              value={ballotFormatFilter}
              onChange={(e) => setBallotFormatFilter(e.target.value)}
            >
              <option value="">Все форматы</option>
              <option value="approval">Одобрение</option>
              <option value="ranking">Ранжирование</option>
              <option value="score">Оценивание</option>
            </select>
          </div>

          <div>
            <label>Правило подсчета</label>
            <select
              style={styles.input}
              value={ruleFilter}
              onChange={(e) => setRuleFilter(e.target.value)}
            >
              <option value="">Все правила</option>
              {ruleOptions.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label>Режим доступа</label>
            <select
              style={styles.input}
              value={accessModeFilter}
              onChange={(e) => setAccessModeFilter(e.target.value)}
            >
              <option value="">Все режимы</option>
              <option value="open">Открытый доступ</option>
              <option value="invite">По приглашениям</option>
            </select>
          </div>

          <div>
            <label>Начало с</label>
            <input
              style={styles.input}
              type="date"
              value={startFrom}
              onChange={(e) => setStartFrom(e.target.value)}
            />
          </div>

          <div>
            <label>Начало по</label>
            <input
              style={styles.input}
              type="date"
              value={startTo}
              onChange={(e) => setStartTo(e.target.value)}
            />
          </div>

          <div style={{ display: "flex", alignItems: "end" }}>
            <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <input
                type="checkbox"
                checked={publishedOnly}
                onChange={(e) => setPublishedOnly(e.target.checked)}
              />
              Только опубликованные
            </label>
          </div>

          <div style={{ display: "flex", alignItems: "end" }}>
            <button
              type="button"
              style={styles.btn}
              onClick={() => {
                setSearchQuery("");
                setStatusFilter("");
                setBallotFormatFilter("");
                setRuleFilter("");
                setAccessModeFilter("");
                setStartFrom("");
                setStartTo("");
                setPublishedOnly(false);
              }}
            >
              Сбросить фильтры
            </button>
          </div>
        </div>

        <div style={{ marginTop: 10, ...styles.muted }}>
          Показано: {filteredItems.length} из {items.length}
        </div>
      </div>

      {loading ? <div style={{ marginTop: 10, ...styles.muted }}>Загрузка…</div> : null}
      {!loading && items.length === 0 ? (
        <div style={{ marginTop: 10, ...styles.muted }}>Нет голосований</div>
      ) : null}
      {!loading && items.length > 0 && filteredItems.length === 0 ? (
        <div style={{ marginTop: 10, ...styles.muted }}>
          По заданным фильтрам ничего не найдено
        </div>
      ) : null}

      <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
        {filteredItems.map((e) => (
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