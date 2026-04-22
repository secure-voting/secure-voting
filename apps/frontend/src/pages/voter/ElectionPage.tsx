import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionDetail } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { styles } from "../../shared/ui/styles";
import { downloadJsonFile } from "../../shared/utils/export";

function statusLabel(status?: string) {
  switch (status) {
    case "draft":
      return "Черновик";
    case "scheduled":
      return "Запланировано";
    case "active":
      return "Активно";
    case "paused":
      return "Приостановлено";
    case "closed":
      return "Закрыто";
    case "results_ready":
      return "Результаты готовы";
    case "published":
      return "Опубликовано";
    default:
      return status || "—";
  }
}

function accessLabel(access?: string) {
  switch (access) {
    case "open":
      return "Открытый доступ";
    case "invite":
      return "По приглашениям";
    default:
      return access || "—";
  }
}

function formatLabel(format?: string) {
  switch (format) {
    case "approval":
      return "Approval";
    case "ranking":
      return "Ranking";
    case "score":
      return "Score";
    default:
      return format || "—";
  }
}

function yesNo(v?: boolean | null) {
  return v ? "Да" : "Нет";
}

type ActionState = {
  busy: boolean;
  error: string | null;
  info: string | null;
};

export function ElectionPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken, me } = useAuth();

  const [item, setItem] = useState<ElectionDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [actionState, setActionState] = useState<ActionState>({
    busy: false,
    error: null,
    info: null,
  });

  const abortRef = useRef<AbortController | null>(null);

  const isVoter = me?.role === "voter";
  const isAdmin = me?.role === "admin";

  const load = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const detail = await api.elections.get(token, electionId, ac.signal);
      setItem(detail);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить карточку голосования");
      }
      setItem(null);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  const runAdminAction = useCallback(
    async (action: "schedule" | "open" | "pause" | "resume" | "close" | "publish") => {
      if (!token || !isAdmin) return;

      setActionState({ busy: true, error: null, info: null });
      try {
        await api.elections.action(token, electionId, action);
        await load();

        const actionInfo =
          action === "schedule"
            ? "Голосование запланировано"
            : action === "open"
              ? "Голосование открыто"
              : action === "pause"
                ? "Голосование приостановлено"
                : action === "resume"
                  ? "Голосование возобновлено"
                  : action === "close"
                    ? "Голосование закрыто, расчет результата поставлен в очередь"
                    : "Результаты опубликованы";

        setActionState({ busy: false, error: null, info: actionInfo });
      } catch (e: any) {
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setActionState({
          busy: false,
          error: e?.message || "Не удалось выполнить действие",
          info: null,
        });
      }
    },
    [token, isAdmin, electionId, load, setToken]
  );

  const adminButtons = useMemo(() => {
    if (!item || !isAdmin) return null;

    const status = item.status;

    return (
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 12 }}>
        {status === "draft" ? (
          <>
            <button
              style={styles.btn}
              onClick={() => runAdminAction("schedule")}
              disabled={actionState.busy}
            >
              Запланировать
            </button>
            <button
              style={styles.btnPrimary}
              onClick={() => runAdminAction("open")}
              disabled={actionState.busy}
            >
              Открыть
            </button>
          </>
        ) : null}

        {status === "scheduled" ? (
          <button
            style={styles.btnPrimary}
            onClick={() => runAdminAction("open")}
            disabled={actionState.busy}
          >
            Открыть
          </button>
        ) : null}

        {status === "active" ? (
          <>
            <button
              style={styles.btn}
              onClick={() => runAdminAction("pause")}
              disabled={actionState.busy}
            >
              Приостановить
            </button>
            <button
              style={styles.btnDanger}
              onClick={() => runAdminAction("close")}
              disabled={actionState.busy}
            >
              Закрыть
            </button>
          </>
        ) : null}

        {status === "paused" ? (
          <>
            <button
              style={styles.btnPrimary}
              onClick={() => runAdminAction("resume")}
              disabled={actionState.busy}
            >
              Возобновить
            </button>
            <button
              style={styles.btnDanger}
              onClick={() => runAdminAction("close")}
              disabled={actionState.busy}
            >
              Закрыть
            </button>
          </>
        ) : null}

        {status === "results_ready" ? (
          <button
            style={styles.btnPrimary}
            onClick={() => runAdminAction("publish")}
            disabled={actionState.busy}
          >
            Опубликовать результаты
          </button>
        ) : null}
      </div>
    );
  }, [item, isAdmin, runAdminAction, actionState.busy]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: 10,
            alignItems: "baseline",
            flexWrap: "wrap",
          }}
        >
          <h2 style={{ margin: 0 }}>
            {isAdmin ? "Управление голосованием" : "Карточка голосования"}
          </h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Назад</button>
            </Link>
            {isVoter ? (
              <Link to={`/elections/${electionId}/vote`} style={{ textDecoration: "none" }}>
                <button style={styles.btnPrimary}>Голосовать</button>
              </Link>
            ) : null}
            <Link to={`/elections/${electionId}/results`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Результаты</button>
            </Link>
            <button style={styles.btn} onClick={load} disabled={loading || actionState.busy}>
              Обновить
            </button>
            {item ? (
              <button
                style={styles.btn}
                onClick={() => downloadJsonFile(`election-${electionId}.json`, item)}
              >
                Экспорт JSON
              </button>
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />
        <ErrorBanner error={actionState.error} />

        {actionState.info ? (
          <div style={{ ...styles.card, background: "#f9fafb", borderColor: "#e5e7eb", marginTop: 12 }}>
            {actionState.info}
          </div>
        ) : null}

        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {item ? (
          <>
            <div style={{ marginTop: 10 }}>
              <div style={{ fontWeight: 800, fontSize: 18 }}>{item.title}</div>
              <div style={styles.muted}>{item.description || "Описание отсутствует"}</div>
            </div>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`Статус: ${statusLabel(item.status)}`} />
              <Badge text={`Доступ: ${accessLabel(item.access_mode)}`} />
              <Badge text={`Формат: ${formatLabel(item.ballot_format)}`} />
              <Badge text={`Правило: ${item.tally_rule}`} />
            </div>

            {isAdmin && item.status === "results_ready" ? (
              <div style={{ marginTop: 12, ...styles.card, background: "#f9fafb" }}>
                Результат рассчитан и готов к публикации.
              </div>
            ) : null}

            {adminButtons}

            <div style={{ marginTop: 12 }}>
              <SummaryGrid
                items={[
                  { label: "Организатор", value: item.organizer_email ?? item.created_by ?? "—" },
                  { label: "Создано", value: item.created_at ?? "—" },
                  { label: "Начало", value: item.start_at },
                  { label: "Окончание", value: item.end_at },
                  { label: "Публиковать не ранее", value: item.publish_at ?? "—" },
                  { label: "Опубликовано", value: item.published_at ?? "—" },
                  { label: "Размер комитета", value: String(item.committee_size ?? "—") },
                  { label: "Тип квоты", value: item.quota_type ?? "—" },
                  { label: "Показывать агрегаты", value: yesNo(item.show_aggregates) },
                  { label: "Кандидатов", value: String(item.candidates.length) },
                  { label: "Подано бюллетеней", value: String(item.submitted_ballots_count ?? "—") },
                  { label: "Всего приглашений", value: String(item.invites_total_count ?? "—") },
                  { label: "Принято приглашений", value: String(item.invites_accepted_count ?? "—") },
                  { label: "Ожидают", value: String(item.invites_pending_count ?? "—") },
                  {
                    label: "Лимит выбора",
                    value:
                      item.ballot_format === "approval"
                        ? String(item.approval_max_choices ?? "—")
                        : "—",
                  },
                  {
                    label: "Ограничение top-k",
                    value:
                      item.ballot_format === "ranking"
                        ? String(item.ranking_top_k ?? "—")
                        : "—",
                  },
                  {
                    label: "Диапазон оценок",
                    value:
                      item.ballot_format === "score"
                        ? `${item.score_min ?? "—"}..${item.score_max ?? "—"}`
                        : "—",
                  },
                  {
                    label: "Шаг оценки",
                    value:
                      item.ballot_format === "score"
                        ? String(item.score_step ?? "—")
                        : "—",
                  },
                  {
                    label: "Разрешить пропуск",
                    value:
                      item.ballot_format === "score"
                        ? yesNo(item.score_allow_skip)
                        : "—",
                  },
                ]}
              />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Кандидаты</h3>
            <div style={{ display: "grid", gap: 8 }}>
              {item.candidates.map((candidate) => {
                const description =
                  candidate.meta &&
                  typeof candidate.meta === "object" &&
                  typeof (candidate.meta as any).description === "string"
                    ? String((candidate.meta as any).description)
                    : "";

                return (
                  <div
                    key={candidate.id}
                    style={{
                      ...styles.card,
                      padding: 10,
                      display: "flex",
                      justifyContent: "space-between",
                      gap: 10,
                      alignItems: "baseline",
                    }}
                  >
                    <div>
                      <div style={{ fontWeight: 700 }}>{candidate.name}</div>
                      {description ? (
                        <div style={{ ...styles.muted, marginTop: 4 }}>{description}</div>
                      ) : null}
                    </div>
                  </div>
                );
              })}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}