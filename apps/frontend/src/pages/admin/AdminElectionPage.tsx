import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type {
  AuditLogItem,
  ElectionDetail,
  Invite,
  InviteCreated,
  InviteImportResponse,
  SystemStatusResponse,
} from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { ActionMenu } from "../../shared/ui/ActionMenu";
import { downloadCsvFile, downloadJsonFile } from "../../shared/utils/export";
import { isValidEmail, parseEmailsFromText, uniqueEmails } from "../../shared/utils/email";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function compact(value: unknown) {
  if (value == null) return "—";
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function inviteCsvRows(invites: Invite[]) {
  return invites.map((item) => ({
    id: item.id,
    email: item.email,
    status: item.status,
    sent_at: item.sent_at ?? "",
    accepted_at: item.accepted_at ?? "",
    created_at: item.created_at,
  }));
}

type BulkSummary = {
  total: number;
  valid: number;
  inviteCreated: string[];
  registrationRequired: string[];
  skipped: string[];
  failed: Array<{ email: string; reason: string }>;
};

function auditDetails(value: AuditLogItem): Record<string, unknown> | null {
  if (!value || typeof value !== "object") return null;
  const raw = (value as Record<string, unknown>).details;
  if (!raw || typeof raw !== "object") return null;
  return raw as Record<string, unknown>;
}

function isElectionAuditItem(item: AuditLogItem, electionId: string) {
  const details = auditDetails(item);
  if (!details) return false;

  const targetType = typeof details.target_type === "string" ? details.target_type : "";
  const targetID = typeof details.target_id === "string" ? details.target_id : "";

  if (targetType === "election" && targetID === electionId) return true;

  const after =
    details.after && typeof details.after === "object"
      ? (details.after as Record<string, unknown>)
      : null;

  const electionID =
    after && typeof after.election_id === "string"
      ? after.election_id
      : "";

  return electionID === electionId;
}

function auditItemLabel(item: AuditLogItem) {
  const eventType = typeof item?.event_type === "string" ? item.event_type : "event";

  const labels: Record<string, string> = {
    election_created: "Голосование создано",
    election_scheduled: "Голосование запланировано",
    election_opened: "Голосование открыто",
    election_paused: "Голосование приостановлено",
    election_resumed: "Голосование возобновлено",
    election_closed: "Голосование завершено",
    election_published: "Результаты опубликованы",
    invite_created: "Создано приглашение",
    invite_registration_required: "Требуется регистрация для приглашения",
  };

  return labels[eventType] || eventType;
}

function auditItemDescription(item: AuditLogItem) {
  const details = auditDetails(item);
  if (!details) return "";

  const after =
    details.after && typeof details.after === "object"
      ? (details.after as Record<string, unknown>)
      : null;

  const email = after && typeof after.email === "string" ? after.email : "";
  const status = after && typeof after.status === "string" ? after.status : "";

  if (email && status) return `${email} · ${status}`;
  if (email) return email;
  if (status) return status;

  return "";
}

function statusCardTone(ok: boolean): React.CSSProperties {
  if (ok) {
    return {
      background: "#f0fdf4",
      borderColor: "#bbf7d0",
    };
  }

  return {
    background: "#fff1f2",
    borderColor: "#fecaca",
  };
}

function electionStatusLabel(value: unknown) {
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

function boolLabel(value: boolean | null | undefined) {
  return value ? "Да" : "Нет";
}

function lifecycleActionLabel(action: string) {
  const labels: Record<string, string> = {
    schedule: "Запланировать",
    open: "Открыть",
    pause: "Приостановить",
    resume: "Возобновить",
    close: "Завершить",
    publish: "Опубликовать результаты",
  };

  return labels[action] || action;
}

function statusLabel(value: string) {
  const normalized = value.trim().toLowerCase();

  if (normalized === "ready") return "ready";
  if (normalized === "idle") return "idle";
  if (normalized === "connecting") return "connecting";
  if (normalized === "transientfailure") return "transient_failure";
  if (normalized === "shutdown") return "shutdown";
  if (normalized === "unavailable") return "unavailable";

  return normalized || "unknown";
}

export function AdminElectionPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [item, setItem] = useState<ElectionDetail | null>(null);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatusResponse | null>(null);

  const [singleInviteEmail, setSingleInviteEmail] = useState("");
  const [bulkInviteText, setBulkInviteText] = useState("");
  const [lastInviteCode, setLastInviteCode] = useState<string | null>(null);

  const [bulkSummary, setBulkSummary] = useState<BulkSummary | null>(null);
  const [recentAuditItems, setRecentAuditItems] = useState<AuditLogItem[]>([]);

  const [lastImportedInvitesFileName, setLastImportedInvitesFileName] = useState("");

  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [inviteLoading, setInviteLoading] = useState(false);
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
      const detail = await api.elections.get(token, electionId, ac.signal);
      setItem(detail);

      try {
        const status = await api.system.status(token, ac.signal);
        setSystemStatus(status);
      } catch (e: any) {
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setSystemStatus(null);
      }

      if (detail.access_mode === "invite") {
        try {
          const inviteItems = await api.elections.listInvites(token, electionId, ac.signal);
          setInvites(inviteItems);
        } catch (e: any) {
          if (e?.status === 401) {
            setToken(null);
            return;
          }
          setInvites([]);
        }
      } else {
        setInvites([]);
      }

      try {
        const auditItems = await api.audit.list(token, { limit: 100 }, ac.signal);
        setRecentAuditItems(
          auditItems
            .filter((entry) => isElectionAuditItem(entry, electionId))
            .slice(0, 10)
        );
      } catch (e: any) {
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setRecentAuditItems([]);
      }
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить карточку голосования");
      }
      setItem(null);
      setInvites([]);
      setRecentAuditItems([]);
      setSystemStatus(null);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  const doAction = async (action: string) => {
    if (!token) return;

    setActionLoading(true);
    setErr(null);

    try {
      await api.elections.action(token, electionId, action);
      addNotification({
        kind: "success",
        title: "Действие выполнено",
        message: `Операция "${lifecycleActionLabel(action)}" успешно применена`,
      });
      await load();
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось выполнить действие");
      }
    } finally {
      setActionLoading(false);
    }
  };

  const handleCreateSingleInvite = async () => {
    if (!token) return;

    const email = singleInviteEmail.trim();
    if (!email) {
      setErr("Введите email");
      return;
    }
    if (!isValidEmail(email)) {
      setErr("Введите корректный email");
      return;
    }

    setInviteLoading(true);
    setErr(null);
    setLastInviteCode(null);
    setBulkSummary(null);

    try {
      const result: InviteCreated = await api.elections.createInvite(token, electionId, email);

      setLastInviteCode(result.invite_code || null);
      setSingleInviteEmail("");

      addNotification({
        kind: "success",
        title: "Приглашение создано",
        message: `${email}: код приглашения создан`,
      });

      await load();
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        const code = e?.code || e?.error?.code || "";
        if (code === "registration_required") {
          setErr(`${email}: пользователь ещё не зарегистрирован`);
        } else {
          setErr(e?.message || "Не удалось создать приглашение");
        }
      }
    } finally {
      setInviteLoading(false);
    }
  };

  const handleCreateBulkInvites = async () => {
    if (!token) return;

    const parsed = parseEmailsFromText(bulkInviteText);
    const unique = uniqueEmails(parsed);
    const valid = unique.filter(isValidEmail);
    const invalid = unique.filter((email) => !isValidEmail(email));

    if (unique.length === 0) {
      setErr("Введите хотя бы один email");
      return;
    }

    setInviteLoading(true);
    setErr(null);
    setLastInviteCode(null);

    const inviteCreated: string[] = [];
    const registrationRequired: string[] = [];
    const skipped: string[] = [...invalid];
    const failed: Array<{ email: string; reason: string }> = [];

    try {
      for (const email of valid) {
        try {
          await api.elections.createInvite(token, electionId, email);
          inviteCreated.push(email);
        } catch (e: any) {
          if (e?.status === 401) {
            setToken(null);
            throw e;
          }

          const code = e?.code || e?.error?.code || "";
          const message = e?.message || "create invite failed";

          if (code === "registration_required") {
            registrationRequired.push(email);
          } else if (code === "email_already_invited" || message.toLowerCase().includes("already invited")) {
            skipped.push(email);
          } else {
            failed.push({ email, reason: message });
          }
        }
      }

      setBulkSummary({
        total: unique.length,
        valid: valid.length,
        inviteCreated,
        registrationRequired,
        skipped,
        failed,
      });

      addNotification({
        kind: failed.length === 0 ? "success" : "info",
        title: "Массовое создание приглашений завершено",
        message: `создано: ${inviteCreated.length}, нужна регистрация: ${registrationRequired.length}, пропущено: ${skipped.length}, ошибок: ${failed.length}`,
      });

      await load();
    } catch (e: any) {
      if (e?.status !== 401) {
        setErr(e?.message || "Ошибка при массовом создании приглашений");
      }
    } finally {
      setInviteLoading(false);
    }
  };

  const handleImportInvitesFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] ?? null;
    e.target.value = "";

    if (!file || !token) return;

    setInviteLoading(true);
    setErr(null);
    setLastInviteCode(null);

    try {
      const result: InviteImportResponse = await api.elections.importInvites(token, electionId, file);
      setLastImportedInvitesFileName(file.name);

      const invalidFailedCount = result.failed.filter((item) => item.code === "invalid_email").length;

      setBulkSummary({
        total: result.total,
        valid: Math.max(0, result.parsed - invalidFailedCount),
        inviteCreated: result.created.map((item) => item.email),
        registrationRequired: result.registration_required.map((item) => item.email),
        skipped: result.skipped.map((item) => item.email),
        failed: result.failed.map((item) => ({
          email: item.email,
          reason: item.code || "import_failed",
        })),
      });

      addNotification({
        kind: result.failed.length === 0 ? "success" : "info",
        title: "Импорт приглашений завершён",
        message: `создано: ${result.created.length}, нужна регистрация: ${result.registration_required.length}, пропущено: ${result.skipped.length}, ошибок: ${result.failed.length}`,
      });

      await load();
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось импортировать приглашения");
      }
    } finally {
      setInviteLoading(false);
    }
  };

  const inviteCounters = useMemo(() => {
    const counters: Record<string, number> = {};
    for (const invite of invites) {
      counters[invite.status] = (counters[invite.status] || 0) + 1;
    }
    return counters;
  }, [invites]);

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
          <h2 style={{ margin: 0 }}>Управление голосованием</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/dashboard/admin" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Назад</button>
            </Link>
            <Link to={`/elections/${electionId}`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Карточка участника</button>
            </Link>
            <Link to={`/elections/${electionId}/results`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Результаты</button>
            </Link>
            <Link to={`/admin/elections/${electionId}/rules`} style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Настройки</button>
            </Link>
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
            {item ? (
              <ActionMenu
                label="Дополнительно"
                items={[
                  {
                    label: "Экспорт JSON",
                    onClick: () => downloadJsonFile(`election-${electionId}.json`, item),
                  },
                ]}
              />
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {item ? (
          <>
            <div style={{ marginTop: 10 }}>
              <div style={{ fontWeight: 800, fontSize: 18 }}>{item.title}</div>
              <div style={styles.muted}>{item.description || "Описание отсутствует"}</div>
            </div>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={electionStatusLabel(item.status)} />
              <Badge text={accessModeLabel(item.access_mode)} />
              <Badge text={ballotFormatLabel(item.ballot_format)} />
              <Badge text={tallyRuleLabel(item.tally_rule)} />
            </div>

            <div style={{ marginTop: 12 }}>
              <SummaryGrid
                items={[
                  { label: "Организатор", value: item.organizer_email ?? item.created_by ?? "—" },
                  { label: "Создано", value: formatDateTime(item.created_at) },
                  { label: "Начало", value: formatDateTime(item.start_at) },
                  { label: "Окончание", value: formatDateTime(item.end_at) },
                  { label: "Запланированная публикация", value: formatDateTime(item.publish_at) },
                  { label: "Фактическая публикация", value: formatDateTime(item.published_at) },
                  { label: "Размер комитета", value: String(item.committee_size ?? "—") },
                  { label: "Квота", value: item.quota_type ?? "—" },
                  { label: "Показывать агрегаты", value: boolLabel(item.show_aggregates) },
                  { label: "Кандидаты", value: String(item.candidates.length) },
                  {
                    label: "Максимум одобрений",
                    value: item.ballot_format === "approval"
                      ? String(item.approval_max_choices ?? "—")
                      : "—",
                  },
                  {
                    label: "Глубина ранжирования",
                    value: item.ballot_format === "ranking"
                      ? String(item.ranking_top_k ?? "—")
                      : "—",
                  },
                  {
                    label: "Диапазон оценок",
                    value: item.ballot_format === "score"
                      ? `${item.score_min ?? "—"}..${item.score_max ?? "—"}`
                      : "—",
                  },
                  {
                    label: "Шаг оценки",
                    value: item.ballot_format === "score"
                      ? String(item.score_step ?? "—")
                      : "—",
                  },
                  {
                    label: "Можно пропускать оценки",
                    value: item.ballot_format === "score"
                      ? boolLabel(item.score_allow_skip)
                      : "—",
                  },
                  { label: "Подано бюллетеней", value: String(item.submitted_ballots_count ?? "—") },
                  {
                    label: "Приглашений всего",
                    value: item.access_mode === "invite" ? String(item.invites_total_count ?? "—") : "—",
                  },
                  {
                    label: "Принято приглашений",
                    value: item.access_mode === "invite" ? String(item.invites_accepted_count ?? "—") : "—",
                  },
                  {
                    label: "Ожидают ответа",
                    value: item.access_mode === "invite" ? String(item.invites_pending_count ?? "—") : "—",
                  },
                  {
                    label: "Отозвано",
                    value: item.access_mode === "invite" ? String(item.invites_revoked_count ?? "—") : "—",
                  },
                  {
                    label: "Ошибок отправки",
                    value: item.access_mode === "invite" ? String(item.invites_failed_count ?? "—") : "—",
                  },
                  {
                    label: "Требуется регистрация",
                    value: item.access_mode === "invite"
                      ? String(item.invites_registration_required_count ?? "—")
                      : "—",
                  },
                ]}
              />

              <details style={{ marginTop: 12 }}>
                <summary style={{ cursor: "pointer", ...styles.muted }}>
                  Технические сведения
                </summary>
                <div style={{ marginTop: 10 }}>
                  <KeyValueList
                    items={[
                      { label: "ID голосования", value: item.id },
                      { label: "ID создателя", value: item.created_by ?? "—" },
                      { label: "Статус", value: item.status },
                      { label: "Режим доступа", value: item.access_mode },
                      { label: "Формат бюллетеня", value: item.ballot_format },
                      { label: "Правило подсчета", value: item.tally_rule },
                      { label: "Создано", value: item.created_at ?? "—" },
                      { label: "Начало", value: item.start_at },
                      { label: "Окончание", value: item.end_at },
                      { label: "Публикация", value: item.publish_at ?? "—" },
                      { label: "Опубликовано", value: item.published_at ?? "—" },
                    ]}
                  />
                </div>
              </details>
            </div>

            <div style={{ marginTop: 12 }}>
              <h3 style={{ marginTop: 0 }}>Состояние сервисов</h3>

              {systemStatus ? (
                <div style={{ display: "grid", gap: 10 }}>
                  <div
                    style={{
                      ...styles.card,
                      ...statusCardTone(systemStatus.backend.ok),
                      padding: 12,
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        gap: 10,
                        alignItems: "baseline",
                        flexWrap: "wrap",
                      }}
                    >
                      <div>
                        <div style={{ fontWeight: 700 }}>Сервер приложения</div>
                        <div style={styles.muted}>Проверка доступности HTTP API</div>
                      </div>
                      <Badge text={statusLabel(systemStatus.backend.status)} />
                    </div>
                  </div>

                  <div
                    style={{
                      ...styles.card,
                      ...statusCardTone(systemStatus.compute.ok),
                      padding: 12,
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        gap: 10,
                        alignItems: "baseline",
                        flexWrap: "wrap",
                      }}
                    >
                      <div>
                        <div style={{ fontWeight: 700 }}>Вычислительный сервис</div>
                        <div style={styles.muted}>Проверка gRPC-соединения</div>
                      </div>
                      <Badge text={statusLabel(systemStatus.compute.status)} />
                    </div>

                    <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                      Проверено: {formatDateTime(systemStatus.checked_at)}
                    </div>

                    {systemStatus.compute.details ? (
                      <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                        Адрес: {compact(systemStatus.compute.details.addr)} · TLS: {compact(systemStatus.compute.details.tls)}
                      </div>
                    ) : null}
                  </div>
                </div>
              ) : (
                <div style={styles.muted}>Статус сервисов пока недоступен</div>
              )}
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
                      {description ? <div style={{ ...styles.muted, marginTop: 4 }}>{description}</div> : null}
                      <details style={{ marginTop: 4 }}>
                        <summary style={{ cursor: "pointer", ...styles.muted }}>
                          Технические сведения
                        </summary>
                        <div style={{ marginTop: 4, ...styles.muted }}>ID кандидата: {candidate.id}</div>
                      </details>
                    </div>
                    {candidate.meta ? <Badge text="meta" /> : null}
                  </div>
                );
              })}
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Управление жизненным циклом</h3>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <button style={styles.btn} onClick={() => doAction("schedule")} disabled={actionLoading}>
                {lifecycleActionLabel("schedule")}
              </button>
              <button style={styles.btn} onClick={() => doAction("open")} disabled={actionLoading}>
                {lifecycleActionLabel("open")}
              </button>
              <button style={styles.btn} onClick={() => doAction("pause")} disabled={actionLoading}>
                {lifecycleActionLabel("pause")}
              </button>
              <button style={styles.btn} onClick={() => doAction("resume")} disabled={actionLoading}>
                {lifecycleActionLabel("resume")}
              </button>
              <button style={styles.btnDanger} onClick={() => doAction("close")} disabled={actionLoading}>
                {lifecycleActionLabel("close")}
              </button>
              <button style={styles.btnPrimary} onClick={() => doAction("publish")} disabled={actionLoading}>
                {lifecycleActionLabel("publish")}
              </button>
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Последние действия</h3>

            {recentAuditItems.length === 0 ? (
              <div style={styles.muted}>События по этому голосованию пока отсутствуют</div>
            ) : (
              <div style={{ display: "grid", gap: 8 }}>
                {recentAuditItems.map((entry) => (
                  <div key={`${entry.id ?? entry.occurred_at}-${entry.event_type}`} style={{ ...styles.card, padding: 10 }}>
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        gap: 10,
                        alignItems: "baseline",
                        flexWrap: "wrap",
                      }}
                    >
                      <div>
                        <div style={{ fontWeight: 700 }}>{auditItemLabel(entry)}</div>
                        {auditItemDescription(entry) ? (
                          <div style={{ ...styles.muted, marginTop: 4 }}>
                            {auditItemDescription(entry)}
                          </div>
                        ) : null}
                      </div>
                      <Badge text={typeof entry.event_type === "string" ? entry.event_type : "event"} />
                    </div>

                    <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                      {entry.occurred_at || "—"}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </>
        ) : null}
      </div>

      {item?.access_mode === "invite" ? (
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
              <h3 style={{ marginTop: 0 }}>Приглашения</h3>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <button
                  style={styles.btn}
                  onClick={() => downloadCsvFile(`election-invites-${electionId}.csv`, inviteCsvRows(invites))}
                  disabled={invites.length === 0}
                >
                  Экспорт CSV
                </button>
              </div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 12 }}>
              {Object.entries(inviteCounters).map(([status, count]) => (
                <Badge key={status} text={`${status}: ${count}`} />
              ))}
              {typeof item?.invites_registration_required_count === "number" ? (
                <Badge text={`требуется регистрация: ${item.invites_registration_required_count}`} />
              ) : null}
            </div>

            <div style={styles.grid2}>
              <div style={{ ...styles.card, padding: 12 }}>
                <h4 style={{ marginTop: 0 }}>Одно приглашение</h4>

                <label>Email</label>
                <input
                  style={styles.input}
                  value={singleInviteEmail}
                  onChange={(e) => setSingleInviteEmail(e.target.value)}
                  placeholder="user@example.com"
                />

                <div style={{ height: 12 }} />

                <button
                  style={styles.btnPrimary}
                  onClick={handleCreateSingleInvite}
                  disabled={inviteLoading}
                >
                  {inviteLoading ? "Создание…" : "Создать приглашение"}
                </button>

                {lastInviteCode ? (
                  <div style={{ marginTop: 12, ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                    <div style={{ fontWeight: 700 }}>Код приглашения</div>
                    <div style={{ marginTop: 6, fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>
                      {lastInviteCode}
                    </div>
                  </div>
                ) : null}
              </div>

              <div style={{ ...styles.card, padding: 12 }}>
                <h4 style={{ marginTop: 0 }}>Массовое создание приглашений</h4>

                <label>Список email</label>
                <textarea
                  style={{
                    ...styles.input,
                    minHeight: 180,
                    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
                  }}
                  value={bulkInviteText}
                  onChange={(e) => setBulkInviteText(e.target.value)}
                  placeholder={"user1@example.com\nuser2@example.com\nuser3@example.com"}
                />

                <div style={{ marginTop: 8, ...styles.muted }}>
                  Можно разделять email новой строкой, запятой или точкой с запятой.
                </div>

                <div style={{ marginTop: 12, ...styles.card, background: "#f9fafb" }}>
                  <div style={{ fontWeight: 600, marginBottom: 8 }}>
                    Импорт email из файла
                  </div>

                  <div style={{ ...styles.muted, marginBottom: 12 }}>
                    Поддерживаются CSV и JSON. CSV: колонка email.
                    JSON: массив строк, объект с items или массив объектов с полем email.
                  </div>

                  <input
                    type="file"
                    accept=".csv,.json,application/json,text/csv"
                    onChange={handleImportInvitesFileChange}
                    disabled={inviteLoading}
                  />

                  {lastImportedInvitesFileName ? (
                    <div style={{ marginTop: 8, fontSize: 13, color: "#667085" }}>
                      Последний импорт: {lastImportedInvitesFileName}
                    </div>
                  ) : null}
                </div>

                <div style={{ height: 12 }} />

                <button
                  style={styles.btnPrimary}
                  onClick={handleCreateBulkInvites}
                  disabled={inviteLoading}
                >
                  {inviteLoading ? "Создание…" : "Создать приглашения"}
                </button>
              </div>
            </div>

            {bulkSummary ? (
              <div style={{ marginTop: 12, display: "grid", gap: 12 }}>
                <SummaryGrid
                  items={[
                    { label: "Всего строк", value: String(bulkSummary.total) },
                    { label: "Корректных email", value: String(bulkSummary.valid) },
                    { label: "Создано приглашений", value: String(bulkSummary.inviteCreated.length) },
                    { label: "Требуется регистрация", value: String(bulkSummary.registrationRequired.length) },
                    { label: "Пропущено", value: String(bulkSummary.skipped.length) },
                    { label: "Ошибок", value: String(bulkSummary.failed.length) },
                  ]}
                />

                {bulkSummary.inviteCreated.length > 0 ? (
                  <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                    <div style={{ fontWeight: 700 }}>Invite sent</div>
                    <div style={{ marginTop: 8, display: "grid", gap: 4 }}>
                      {bulkSummary.inviteCreated.map((email) => (
                        <div key={email}>{email}</div>
                      ))}
                    </div>
                  </div>
                ) : null}

                {bulkSummary.registrationRequired.length > 0 ? (
                  <div style={{ ...styles.card, background: "#eff8ff", borderColor: "#b2ddff" }}>
                    <div style={{ fontWeight: 700 }}>Registration required</div>
                    <div style={{ marginTop: 8, display: "grid", gap: 4 }}>
                      {bulkSummary.registrationRequired.map((email) => (
                        <div key={email}>{email}</div>
                      ))}
                    </div>
                  </div>
                ) : null}

                {bulkSummary.skipped.length > 0 ? (
                  <div style={{ ...styles.card, background: "#f9fafb" }}>
                    <div style={{ fontWeight: 700 }}>Skipped</div>
                    <div style={{ marginTop: 8, display: "grid", gap: 4 }}>
                      {bulkSummary.skipped.map((email) => (
                        <div key={email}>{email}</div>
                      ))}
                    </div>
                  </div>
                ) : null}

                {bulkSummary.failed.length > 0 ? (
                  <div style={{ ...styles.card, background: "#fff1f2", borderColor: "#fecaca" }}>
                    <div style={{ fontWeight: 700, color: "#7f1d1d" }}>Failed</div>
                    <div style={{ marginTop: 8, display: "grid", gap: 6 }}>
                      {bulkSummary.failed.map((item) => (
                        <div key={`${item.email}-${item.reason}`}>
                          <b>{item.email}</b>: {item.reason}
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

          <div style={styles.card}>
            <h3 style={{ marginTop: 0 }}>Список приглашений</h3>

            {invites.length === 0 ? (
              <div style={styles.muted}>Приглашения пока отсутствуют</div>
            ) : (
              <div style={{ display: "grid", gap: 8 }}>
                {invites.map((invite) => (
                  <div key={invite.id} style={{ ...styles.card, padding: 10 }}>
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        gap: 10,
                        alignItems: "baseline",
                        flexWrap: "wrap",
                      }}
                    >
                      <div>
                        <div style={{ fontWeight: 700 }}>{invite.email}</div>
                        <div style={styles.muted}>{invite.id}</div>
                      </div>
                      <Badge text={invite.status} />
                    </div>

                    <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                      created_at: {compact(invite.created_at)} · sent_at: {compact(invite.sent_at)} · accepted_at: {compact(invite.accepted_at)}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      ) : null}

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug JSON</h3>
          {item ? <JsonBlock value={item} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}