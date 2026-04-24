import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { AuditLogItem, ElectionSummary, JobItem, SystemStatusResponse } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

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

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>Рабочий стол администратора</h2>
        <div style={styles.muted}>
          Добро пожаловать{me?.email ? `, ${me.email}` : ""}. Здесь собраны основные объекты управления голосованиями и мониторингом.
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Badge text={`elections: ${elections.length}`} />
          <Badge text={`jobs: ${jobs.length}`} />
          <Badge text={`audit: ${audit.length}`} />
          <Badge text={`backend: ${systemStatus?.backend?.status ?? "unknown"}`} />
          <Badge text={`compute: ${systemStatus?.compute?.status ?? "unknown"}`} />
          <Badge text={`worker: ${systemStatus?.worker?.status ?? "unknown"}`} />
          <button style={styles.btn} onClick={load} disabled={loading}>
            Обновить
          </button>
          <Link to="/admin/elections/create" style={{ textDecoration: "none" }}>
            <button style={styles.btnPrimary}>Создать голосование</button>
          </Link>

          <Link to="/admin/users" style={{ textDecoration: "none" }}>
            <button style={styles.btn}>Пользователи</button>
          </Link>

          <Link to="/admin/settings" style={{ textDecoration: "none" }}>
            <button style={styles.btn}>Настройки</button>
          </Link>
        </div>

        <ErrorBanner error={err} />
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Состояния голосований</h3>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {Object.keys(byStatus).length === 0 ? (
            <div style={styles.muted}>Нет данных</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {Object.entries(byStatus).map(([status, count]) => (
                <div key={status} style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <span>{status}</span>
                  <Badge text={String(count)} />
                </div>
              ))}
            </div>
          )}
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Последние голосования</h3>
          {elections.length === 0 ? (
            <div style={styles.muted}>Пока нет голосований</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {elections.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ fontWeight: 700 }}>{item.title}</div>
                  <div style={styles.muted}>{item.description || "Описание отсутствует"}</div>

                  <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <Badge text={item.status} />
                    <Badge text={item.access_mode} />
                  </div>

                  <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <Link to={`/admin/elections/${item.id}`} style={{ textDecoration: "none" }}>
                      <button style={styles.btn}>Карточка</button>
                    </Link>
                    <Link to={`/admin/elections/${item.id}/rules`} style={{ textDecoration: "none" }}>
                      <button style={styles.btnPrimary}>Настройки</button>
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
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
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <strong>{name}</strong>
                  <Badge text={String(item.status)} />
                </div>
                <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                  ok: {String(item.ok)}
                </div>
                {item.details ? (
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
                ) : null}
              </div>
            ))}
          </div>
        )}
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
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
                  <div style={{ fontWeight: 700 }}>{String(item.kind ?? "unknown")}</div>
                  <div style={styles.muted}>status: {jobStatusValue(item)}</div>
                  <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                    created_at: {String(item.created_at ?? "—")}
                  </div>
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
                  <div style={{ fontWeight: 700 }}>{auditTypeValue(item)}</div>
                  <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                    occurred_at: {String(item.occurred_at ?? "—")}
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