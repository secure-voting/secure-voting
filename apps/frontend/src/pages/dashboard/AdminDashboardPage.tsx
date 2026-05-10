import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { AuditLogItem, ElectionSummary, JobItem, SystemStatusResponse } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";

function displayName(me: ReturnType<typeof useAuth>["me"]) {
  const fullName = typeof me?.full_name === "string" ? me.full_name.trim() : "";
  if (fullName) return fullName;
  return me?.email || "администратор";
}

function electionStatusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    draft: "Черновики",
    scheduled: "Запланированы",
    active: "Открыты",
    paused: "Приостановлены",
    closed: "Завершены",
    results_ready: "Результаты готовы",
    published: "Опубликованы",
  };

  return labels[raw] || raw || "Статус неизвестен";
}

function electionStatusBadge(value: unknown) {
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

function jobKindLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    tally: "Подсчет голосования",
    import_dataset: "Импорт набора данных",
    generate_dataset: "Генерация набора данных",
    experiment_run: "Запуск эксперимента",
    report: "Формирование отчета",
  };

  return labels[raw] || raw || "Задача";
}

function jobStatusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    queued: "В очереди",
    running: "Выполняется",
    done: "Завершена",
    error: "Ошибка",
  };

  return labels[raw] || raw || "Статус неизвестен";
}

function auditEventLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    user_registered: "Пользователь зарегистрирован",
    user_logged_in: "Пользователь вошел",
    user_logged_out: "Пользователь вышел",
    user_password_changed: "Пароль изменен",
    user_profile_updated: "Профиль обновлен",
    auth_token_refreshed: "Токены обновлены",

    election_created: "Голосование создано",
    election_rules_updated: "Правила обновлены",
    election_scheduled: "Голосование запланировано",
    election_opened: "Голосование открыто",
    election_paused: "Голосование приостановлено",
    election_resumed: "Голосование возобновлено",
    election_closed: "Голосование завершено",
    election_published: "Результаты опубликованы",

    invite_created: "Приглашение создано",
    invite_accepted: "Приглашение принято",
    invite_registration_required: "Требуется регистрация",

    ballot_submitted: "Бюллетень отправлен",
    experiment_created: "Эксперимент создан",
    experiment_run_created: "Запуск эксперимента создан",
    dataset_imported: "Набор данных импортирован",
    dataset_generated: "Набор данных сгенерирован",
  };

  return labels[raw] || raw || "Событие";
}

function componentNameLabel(value: string) {
  const labels: Record<string, string> = {
    backend: "Сервер приложения",
    compute: "Вычислительный сервис",
    worker: "Фоновый обработчик",
  };

  return labels[value] || value;
}

function componentStatusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    ok: "Доступен",
    degraded: "Частичная деградация",
    error: "Ошибка",
    unknown: "Неизвестно",
  };

  return labels[raw] || raw || "Неизвестно";
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

function jobStatusValue(item: JobItem) {
  return typeof item.status === "string" ? item.status : "unknown";
}

function auditTypeValue(item: AuditLogItem) {
  return typeof item.event_type === "string" ? item.event_type : "unknown";
}

export function AdminDashboardPage() {
  const { token, setToken, me } = useAuth();

  const [elections, setElections] = useState<ElectionSummary[]>([]);
  const [jobs, setJobs] = useState<JobItem[]>([]);
  const [audit, setAudit] = useState<AuditLogItem[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatusResponse | null>(null);

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
      const [electionList, jobsList, auditList, status] = await Promise.all([
        api.elections.list(token, ac.signal),
        api.jobs.list(token, { limit: 5, offset: 0 }, ac.signal),
        api.audit.list(token, { limit: 5, offset: 0 }, ac.signal),
        api.system.status(token, ac.signal),
      ]);

      setElections(electionList);
      setJobs(jobsList);
      setAudit(auditList);
      setSystemStatus(status);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить рабочий стол администратора");
      }
      setElections([]);
      setJobs([]);
      setAudit([]);
      setSystemStatus(null);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  const byStatus = useMemo(() => {
    const counters: Record<string, number> = {};
    for (const item of elections) {
      counters[item.status] = (counters[item.status] || 0) + 1;
    }
    return counters;
  }, [elections]);

  const runningJobs = useMemo(
    () => jobs.filter((item) => jobStatusValue(item) === "running" || jobStatusValue(item) === "queued").length,
    [jobs]
  );

  const failedJobs = useMemo(
    () => jobs.filter((item) => jobStatusValue(item) === "error").length,
    [jobs]
  );

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
            <h2 style={{ margin: 0 }}>Рабочий стол администратора</h2>
            <div style={{ ...styles.muted, marginTop: 4 }}>
              Добро пожаловать, {displayName(me)}. Здесь собраны быстрые действия, состояние голосований и мониторинг системы.
            </div>
          </div>

          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
            <Link to="/admin/elections/create" style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Создать голосование</button>
            </Link>
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 14 }}>
          <KeyValueList
            items={[
              { label: "Голосований", value: String(elections.length) },
              { label: "Активных задач", value: String(runningJobs) },
              { label: "Ошибок задач", value: String(failedJobs) },
              { label: "Последних событий аудита", value: String(audit.length) },
            ]}
          />
        </div>
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Быстрые действия</h3>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/admin/elections/create" style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Создать голосование</button>
            </Link>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Все голосования</button>
            </Link>
            <Link to="/monitor/jobs" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Задачи</button>
            </Link>
            <Link to="/monitor/audit" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Журнал аудита</button>
            </Link>
            <Link to="/admin/users" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Пользователи</button>
            </Link>
            <Link to="/admin/settings" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Настройки</button>
            </Link>
          </div>
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Состояния голосований</h3>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {Object.keys(byStatus).length === 0 ? (
            <div style={styles.muted}>Нет данных</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {Object.entries(byStatus).map(([status, count]) => (
                <div key={status} style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <span>{electionStatusLabel(status)}</span>
                  <Badge text={String(count)} />
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Последние голосования</h3>
        {elections.length === 0 ? (
          <div style={styles.muted}>Пока нет голосований</div>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {elections.slice(0, 6).map((item) => (
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
                      {item.description || "Описание отсутствует"}
                    </div>
                  </div>

                  <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <Badge text={electionStatusBadge(item.status)} />
                    <Badge text={accessModeLabel(item.access_mode)} />
                  </div>
                </div>

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <Link to={`/admin/elections/${item.id}`} style={{ textDecoration: "none" }}>
                    <button style={styles.btnPrimary}>Управление</button>
                  </Link>
                  <Link to={`/admin/elections/${item.id}/rules`} style={{ textDecoration: "none" }}>
                    <button style={styles.btn}>Настройки правил</button>
                  </Link>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Состояние компонентов</h3>
        {!systemStatus ? (
          <div style={styles.muted}>Нет данных</div>
        ) : (
          <div style={{ display: "grid", gap: 10 }}>
            {[
              { name: "backend", item: systemStatus.backend },
              { name: "compute", item: systemStatus.compute },
              { name: "worker", item: systemStatus.worker },
            ].map(({ name, item }) => (
              <div key={name} style={{ ...styles.card, padding: 10 }}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                  <strong>{componentNameLabel(name)}</strong>
                  <Badge text={componentStatusLabel(item.status)} />
                </div>

                <div style={{ marginTop: 6, ...styles.muted, fontSize: 13 }}>
                  {item.ok ? "Компонент отвечает штатно" : "Компонент требует внимания"}
                </div>

                {item.details ? (
                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <pre
                      style={{
                        marginTop: 8,
                        padding: 8,
                        borderRadius: 8,
                        background: "#f8fafc",
                        overflowX: "auto",
                        fontSize: 12,
                      }}
                    >
                      {JSON.stringify(item.details, null, 2)}
                    </pre>
                  </details>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              gap: 10,
              flexWrap: "wrap",
              alignItems: "center",
            }}
          >
            <h3 style={{ marginTop: 0 }}>Последние задачи</h3>
            <Link to="/monitor/jobs" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Все задачи</button>
            </Link>
          </div>

          {jobs.length === 0 ? (
            <div style={styles.muted}>Задачи не найдены</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {jobs.map((item, index) => (
                <div key={String(item.id ?? index)} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                    <div>
                      <div style={{ fontWeight: 800 }}>{jobKindLabel(item.kind)}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        Создана: {formatDateTime(item.created_at)}
                      </div>
                    </div>
                    <div style={{ display: "flex", alignItems: "center", minHeight: 36 }}>
                      <Badge text={jobStatusLabel(jobStatusValue(item))} />
                    </div>
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID задачи", value: String(item.id ?? "—") },
                          { label: "Тип", value: String(item.kind ?? "—") },
                          { label: "Статус", value: String(item.status ?? "—") },
                        ]}
                      />
                    </div>
                  </details>
                </div>
              ))}
            </div>
          )}
        </div>

        <div style={styles.card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
            <h3 style={{ marginTop: 0 }}>Последние события аудита</h3>
            <Link to="/monitor/audit" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Весь журнал</button>
            </Link>
          </div>

          {audit.length === 0 ? (
            <div style={styles.muted}>События не найдены</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {audit.map((item, index) => (
                <div key={String(item.id ?? index)} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                    <div>
                      <div style={{ fontWeight: 800 }}>{auditEventLabel(auditTypeValue(item))}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        Время: {formatDateTime(item.occurred_at)}
                      </div>
                    </div>
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID события", value: String(item.id ?? "—") },
                          { label: "Тип события", value: String(item.event_type ?? "—") },
                          { label: "Время", value: String(item.occurred_at ?? "—") },
                        ]}
                      />
                    </div>
                  </details>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}