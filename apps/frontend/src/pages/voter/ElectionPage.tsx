import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type {
  AuditLogItem,
  ElectionDetail,
  Invite,
  InviteCreated,
  InviteImportResponse,
} from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { styles } from "../../shared/ui/styles";
import { downloadCsvFile, downloadJsonFile } from "../../shared/utils/export";
import { isValidEmail, parseEmailsFromText, uniqueEmails } from "../../shared/utils/email";

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

  const after = details.after && typeof details.after === "object"
    ? (details.after as Record<string, unknown>)
    : null;

  const electionID =
    after && typeof after.election_id === "string"
      ? after.election_id
      : "";

  return electionID === electionId;
}

function auditItemLabel(item: AuditLogItem) {
  const eventType =
    typeof item?.event_type === "string" ? item.event_type : "event";

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

  const after = details.after && typeof details.after === "object"
    ? (details.after as Record<string, unknown>)
    : null;

  const email = after && typeof after.email === "string" ? after.email : "";
  const status = after && typeof after.status === "string" ? after.status : "";

  if (email && status) return `${email} · ${status}`;
  if (email) return email;
  if (status) return status;

  return "";
}


export function ElectionPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken, me } = useAuth();
  const { addNotification } = useNotifications();

  const [item, setItem] = useState<ElectionDetail | null>(null);
  const [invites, setInvites] = useState<Invite[]>([]);

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

      if (isAdmin && detail.access_mode === "invite") {
        try {
          const inviteItems = await api.elections.listInvites(token, electionId, ac.signal);
          setInvites(inviteItems);
        } catch (e: any) {
          if (e?.status === 401) {
            setToken(null);
          } else {
            setInvites([]);
          }
        }
      } else {
        setInvites([]);
      }

      try {
        const auditItems = await api.audit.list(
          token,
          { limit: 100 },
          ac.signal
        );

        setRecentAuditItems(
          auditItems
            .filter((entry) => isElectionAuditItem(entry, electionId))
            .slice(0, 10)
        );
      } catch (e: any) {
        if (e?.status === 401) {
          setToken(null);
        } else {
          setRecentAuditItems([]);
        }
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
    } finally {
      setLoading(false);
    }
  }, [token, electionId, isAdmin, setToken]);

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
        message: `Операция ${action} успешно применена`,
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
          <h2 style={{ margin: 0 }}>Карточка голосования</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Назад</button>
            </Link>
            <Link to={`/elections/${electionId}/vote`} style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Голосовать</button>
            </Link>
            <Link to={`/elections/${electionId}/results`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Результаты</button>
            </Link>
            {isAdmin ? (
              <Link to={`/admin/elections/${electionId}/rules`} style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Настройки</button>
              </Link>
            ) : null}
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
            {item ? (
              <button
                style={styles.btn}
                onClick={() => downloadJsonFile(`election-${electionId}.json`, item)}
              >
                Export JSON
              </button>
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
              <Badge text={`status: ${item.status}`} />
              <Badge text={`access: ${item.access_mode}`} />
              <Badge text={`format: ${item.ballot_format}`} />
              <Badge text={`rule: ${item.tally_rule}`} />
            </div>

            <div style={{ marginTop: 12 }}>
              <SummaryGrid
                items={[
                  { label: "Organizer", value: item.organizer_email ?? item.created_by ?? "—" },
                  { label: "Created at", value: item.created_at ?? "—" },
                  { label: "Start at", value: item.start_at },
                  { label: "End at", value: item.end_at },
                  { label: "Publish at", value: item.publish_at ?? "—" },
                  { label: "Published at", value: item.published_at ?? "—" },
                  { label: "Committee size", value: String(item.committee_size ?? "—") },
                  { label: "Quota type", value: item.quota_type ?? "—" },
                  { label: "Show aggregates", value: item.show_aggregates ? "yes" : "no" },
                  { label: "Candidates", value: String(item.candidates.length) },
                  {
                    label: "Approval max choices",
                    value: item.ballot_format === "approval"
                      ? String(item.approval_max_choices ?? "—")
                      : "—",
                  },
                  {
                    label: "Ranking top-k",
                    value: item.ballot_format === "ranking"
                      ? String(item.ranking_top_k ?? "—")
                      : "—",
                  },
                  {
                    label: "Score range",
                    value: item.ballot_format === "score"
                      ? `${item.score_min ?? "—"}..${item.score_max ?? "—"}`
                      : "—",
                  },
                  {
                    label: "Score step",
                    value: item.ballot_format === "score"
                      ? String(item.score_step ?? "—")
                      : "—",
                  },
                  {
                    label: "Allow skip",
                    value: item.ballot_format === "score"
                      ? (item.score_allow_skip ? "yes" : "no")
                      : "—",
                  },
                  { label: "Submitted ballots", value: String(item.submitted_ballots_count ?? "—") },
                  { label: "Invites total", value: item.access_mode === "invite" ? String(item.invites_total_count ?? "—") : "—" },
                  { label: "Invites accepted", value: item.access_mode === "invite" ? String(item.invites_accepted_count ?? "—") : "—" },
                  { label: "Invites pending", value: item.access_mode === "invite" ? String(item.invites_pending_count ?? "—") : "—" },
                  { label: "Invites revoked", value: item.access_mode === "invite" ? String(item.invites_revoked_count ?? "—") : "—" },
                  { label: "Invites failed", value: item.access_mode === "invite" ? String(item.invites_failed_count ?? "—") : "—" },
                  { label: "Registration required", value: item.access_mode === "invite" ? String(item.invites_registration_required_count ?? "—") : "—" },
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
                      {description ? <div style={{ ...styles.muted, marginTop: 4 }}>{description}</div> : null}
                      <div style={{ ...styles.muted, marginTop: 4 }}>{candidate.id}</div>
                    </div>
                    {candidate.meta ? <Badge text="meta" /> : null}
                  </div>
                );
              })}
            </div>

            {isAdmin ? (
              <>
                <hr style={styles.hr} />

                <h3 style={{ marginTop: 0 }}>Admin actions</h3>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btn} onClick={() => doAction("schedule")} disabled={actionLoading}>
                    schedule
                  </button>
                  <button style={styles.btn} onClick={() => doAction("open")} disabled={actionLoading}>
                    open
                  </button>
                  <button style={styles.btn} onClick={() => doAction("pause")} disabled={actionLoading}>
                    pause
                  </button>
                  <button style={styles.btn} onClick={() => doAction("resume")} disabled={actionLoading}>
                    resume
                  </button>
                  <button style={styles.btnDanger} onClick={() => doAction("close")} disabled={actionLoading}>
                    close
                  </button>
                  <button style={styles.btnPrimary} onClick={() => doAction("publish")} disabled={actionLoading}>
                    publish
                  </button>
                </div>
              </>
            ) : null}
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
                        <div style={{ fontWeight: 700 }}>
                          {auditItemLabel(entry)}
                        </div>
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

      {isAdmin && item?.access_mode === "invite" ? (
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
                  Export CSV
                </button>
              </div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 12 }}>
              {Object.entries(inviteCounters).map(([status, count]) => (
                <Badge key={status} text={`${status}: ${count}`} />
              ))}
              {typeof item?.invites_registration_required_count === "number" ? (
                <Badge text={`registration_required: ${item.invites_registration_required_count}`} />
              ) : null}
            </div>

            <div style={styles.grid2}>
              <div style={{ ...styles.card, padding: 12 }}>
                <h4 style={{ marginTop: 0 }}>Одиночное приглашение</h4>

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
                  {inviteLoading ? "Создание…" : "Создать invite"}
                </button>

                {lastInviteCode ? (
                  <div style={{ marginTop: 12, ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                    <div style={{ fontWeight: 700 }}>Invite code</div>
                    <div style={{ marginTop: 6, fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>
                      {lastInviteCode}
                    </div>
                  </div>
                ) : null}
              </div>

              <div style={{ ...styles.card, padding: 12 }}>
                <h4 style={{ marginTop: 0 }}>Массовое создание приглашений</h4>

                <label>Email list</label>
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
                  {inviteLoading ? "Создание…" : "Создать invites bulk"}
                </button>
              </div>
            </div>

            {bulkSummary ? (
              <div style={{ marginTop: 12, display: "grid", gap: 12 }}>
                <SummaryGrid
                  items={[
                    { label: "Total parsed", value: String(bulkSummary.total) },
                    { label: "Valid emails", value: String(bulkSummary.valid) },
                    { label: "Invite created", value: String(bulkSummary.inviteCreated.length) },
                    { label: "Registration required", value: String(bulkSummary.registrationRequired.length) },
                    { label: "Skipped", value: String(bulkSummary.skipped.length) },
                    { label: "Failed", value: String(bulkSummary.failed.length) },

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