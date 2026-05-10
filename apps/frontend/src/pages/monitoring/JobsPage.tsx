import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../shared/api/client";
import type { JobItem } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { ActionMenu } from "../../shared/ui/ActionMenu";
import {
  downloadCsvFile,
  downloadJsonFile,
  downloadPdfTextFile,
  downloadXlsxFile,
} from "../../shared/utils/export";

function str(v: unknown) {
  if (v == null) return "";
  return String(v);
}

function statusOf(job: JobItem) {
  return typeof job.status === "string" ? job.status : "unknown";
}

function kindOf(job: JobItem) {
  return typeof job.kind === "string" ? job.kind : "unknown";
}

function idOf(job: JobItem, index: number) {
  const id = (job as any)?.id;
  return typeof id === "string" && id.trim() ? id.trim() : `job-${index}`;
}

function nowTimeLabel() {
  const d = new Date();
  return d.toLocaleTimeString();
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

function shortId(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
}

function kindLabel(kind: string) {
  switch (kind) {
    case "tally":
      return "Расчет результата";
    case "experiment_run":
      return "Запуск эксперимента";
    case "import_dataset":
      return "Импорт набора данных";
    case "generate_dataset":
      return "Генерация набора данных";
    case "report":
      return "Формирование отчета";
    default:
      return kind;
  }
}

function statusLabel(status: string) {
  switch (status) {
    case "queued":
      return "В очереди";
    case "running":
      return "Выполняется";
    case "done":
      return "Завершена";
    case "error":
      return "Ошибка";
    default:
      return status;
  }
}

function jobsCsvRows(items: JobItem[]) {
  return items.map((job, index) => ({
    title: kindLabel(kindOf(job)),
    status_label: statusLabel(statusOf(job)),
    progress:
      typeof (job as any)?.progress === "number" && Number.isFinite((job as any)?.progress)
        ? String((job as any).progress)
        : "",
    created_at: str((job as any)?.created_at),
    started_at: str((job as any)?.started_at),
    finished_at: str((job as any)?.finished_at),
    id: idOf(job, index),
    kind: kindOf(job),
    status: statusOf(job),
    election_id: str((job as any)?.election_id),
    experiment_id: str((job as any)?.experiment_id),
    experiment_run_id: str((job as any)?.experiment_run_id),
    error_text: str((job as any)?.error_text),
  }));
}

function buildJobsReportText(
  items: JobItem[],
  filters: {
    statusFilter: string;
    kindFilter: string;
  }
) {
  const lines: string[] = [];

  lines.push("Отчет по задачам");
  lines.push("");
  lines.push("Фильтры:");
  lines.push(`- статус: ${filters.statusFilter || "—"}`);
  lines.push(`- тип: ${filters.kindFilter || "—"}`);
  lines.push("");
  lines.push(`Всего задач: ${items.length}`);
  lines.push("");

  items.forEach((job, index) => {
    lines.push(
      `${index + 1}. ${kindLabel(kindOf(job))}; статус=${statusLabel(
        statusOf(job)
      )}; создано=${formatDateTime((job as any)?.created_at)}; ID=${idOf(job, index)}`
    );
  });

  lines.push("");
  return `${lines.join("\n")}`;
}

export function JobsPage() {
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [items, setItems] = useState<JobItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const [statusFilter, setStatusFilter] = useState("");
  const [kindFilter, setKindFilter] = useState("");

  const [pollingOn, setPollingOn] = useState(true);
  const [pollEverySec, setPollEverySec] = useState(5);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);
  const timerRef = useRef<number | null>(null);
  const prevStatusRef = useRef<Map<string, string>>(new Map());

  const load = useCallback(
    async (silent?: boolean) => {
      if (!token) return;

      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;

      if (!silent) setLoading(true);
      setErr(null);

      try {
        const list = await api.jobs.list(
          token,
          {
            limit: 50,
            offset: 0,
            status: statusFilter.trim() || undefined,
            kind: kindFilter.trim() || undefined,
          },
          ac.signal
        );

        const prev = prevStatusRef.current;
        const nextMap = new Map<string, string>();

        list.forEach((job, index) => {
          const id = idOf(job, index);
          const nextStatus = statusOf(job);
          nextMap.set(id, nextStatus);

          const prevStatus = prev.get(id);
          if (prevStatus && prevStatus !== nextStatus) {
            if (nextStatus === "done") {
              addNotification({
                kind: "success",
                title: "Задача завершена",
                message: `${kindLabel(kindOf(job))} · задача ${shortId(id)}`,
              });
            } else if (nextStatus === "error") {
              addNotification({
                kind: "error",
                title: "Ошибка выполнения задачи",
                message: `${kindLabel(kindOf(job))} · задача ${shortId(id)}`,
              });
            }
          }
        });

        prevStatusRef.current = nextMap;
        setItems(list);
        setLastUpdatedAt(nowTimeLabel());
      } catch (e: any) {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) {
          setToken(null);
        } else {
          setErr(e?.message || "Не удалось загрузить список задач");
        }
        setItems([]);
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [token, statusFilter, kindFilter, setToken, addNotification]
  );

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  useEffect(() => {
    if (!pollingOn) {
      if (timerRef.current) window.clearInterval(timerRef.current);
      timerRef.current = null;
      return;
    }

    const intervalMs = Math.max(2, Number.isFinite(pollEverySec) ? pollEverySec : 5) * 1000;

    if (timerRef.current) window.clearInterval(timerRef.current);
    timerRef.current = window.setInterval(() => {
      load(true);
    }, intervalMs);

    return () => {
      if (timerRef.current) window.clearInterval(timerRef.current);
      timerRef.current = null;
    };
  }, [pollingOn, pollEverySec, load]);

  const counters = useMemo(() => {
    const m: Record<string, number> = {};
    for (const it of items) {
      const s = statusOf(it);
      m[s] = (m[s] || 0) + 1;
    }
    return m;
  }, [items]);

  const exportCsv = useCallback(() => {
    downloadCsvFile("jobs.csv", jobsCsvRows(items));
  }, [items]);

  const exportXlsx = useCallback(() => {
    downloadXlsxFile("jobs.xlsx", jobsCsvRows(items), "Задачи");
  }, [items]);

  const exportPdf = useCallback(() => {
    downloadPdfTextFile(
      "jobs-report.pdf",
      "Отчет по задачам",
      buildJobsReportText(items, {
        statusFilter,
        kindFilter,
      })
    );
  }, [items, statusFilter, kindFilter]);

  const exportJson = useCallback(() => {
    downloadJsonFile("jobs.json", items);
  }, [items]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline", flexWrap: "wrap" }}>
          <h2 style={{ margin: 0 }}>Задачи</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => load(false)} disabled={loading}>
              Обновить
            </button>

            <ActionMenu
              label="Экспорт"
              items={[
                { label: "CSV", onClick: exportCsv, disabled: items.length === 0 },
                { label: "XLSX", onClick: exportXlsx, disabled: items.length === 0 },
                { label: "JSON", onClick: exportJson, disabled: items.length === 0 },
                { label: "PDF", onClick: exportPdf, disabled: items.length === 0 },
              ]}
            />
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 12, ...styles.grid2 }}>
          <div>
            <label>Фильтр по статусу</label>
            <input
              style={styles.input}
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              placeholder="queued, running, done, error"
            />
          </div>
          <div>
            <label>Фильтр по типу</label>
            <input
              style={styles.input}
              value={kindFilter}
              onChange={(e) => setKindFilter(e.target.value)}
              placeholder="tally, experiment_run, import_dataset"
            />
          </div>
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 10, flexWrap: "wrap", alignItems: "center" }}>
          <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <input
              type="checkbox"
              checked={pollingOn}
              onChange={(e) => setPollingOn(e.target.checked)}
            />
            Автообновление
          </label>

          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={styles.muted}>каждые</span>
            <input
              style={{ ...styles.input, width: 90 }}
              type="number"
              min={2}
              value={pollEverySec}
              onChange={(e) => setPollEverySec(Number(e.target.value))}
            />
            <span style={styles.muted}>сек</span>
          </div>

          {lastUpdatedAt ? <span style={styles.muted}>обновлено: {lastUpdatedAt}</span> : null}
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          {Object.entries(counters).map(([k, v]) => (
            <Badge key={k} text={`${statusLabel(k)}: ${v}`} />
          ))}
        </div>

        {loading ? <div style={{ marginTop: 10, ...styles.muted }}>Загрузка…</div> : null}
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Список задач</h3>

        {items.length === 0 ? (
          <div style={styles.muted}>Список пуст</div>
        ) : (
          <div style={{ display: "grid", gap: 10 }}>
            {items.map((job, index) => {
              const id = idOf(job, index);
              const status = statusOf(job);
              const kind = kindOf(job);

              const progress = (job as any)?.progress;
              const hasProgress = typeof progress === "number" && Number.isFinite(progress);

              const createdAt = str((job as any)?.created_at);
              const startedAt = str((job as any)?.started_at);
              const finishedAt = str((job as any)?.finished_at);
              const errorText = str((job as any)?.error_text);

              return (
                <div key={id} style={{ ...styles.card, padding: 12 }}>
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
                      <div style={{ fontWeight: 800 }}>{kindLabel(kind)}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        {hasProgress ? `Прогресс выполнения: ${progress}%` : "Задача ожидает обновления статуса"}
                      </div>
                    </div>

                    <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                      <Badge text={statusLabel(status)} />
                      {hasProgress ? <Badge text={`${progress}%`} /> : null}
                    </div>
                  </div>

                  <div style={{ marginTop: 10 }}>
                    <KeyValueList
                      items={[
                        { label: "Создана", value: formatDateTime(createdAt) },
                        { label: "Запущена", value: formatDateTime(startedAt) },
                        { label: "Завершена", value: formatDateTime(finishedAt) },
                      ]}
                    />
                  </div>

                  {kind === "tally" && status === "done" ? (
                    <div style={{ marginTop: 10, ...styles.muted }}>
                      Расчет результата завершен. Если голосование еще не опубликовано, результат готов к публикации.
                    </div>
                  ) : null}

                  {status === "error" && errorText ? (
                    <div
                      style={{
                        marginTop: 10,
                        ...styles.card,
                        background: "#fff1f2",
                        borderColor: "#fecaca",
                        color: "#7f1d1d",
                      }}
                    >
                      <b>Ошибка:</b> {errorText}
                    </div>
                  ) : null}

                  <details style={{ marginTop: 10 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 10 }}>
                      <KeyValueList
                        items={[
                          { label: "ID задачи", value: id },
                          { label: "Тип", value: kind },
                          { label: "Статус", value: status },
                          { label: "ID голосования", value: str((job as any)?.election_id) || "—" },
                          { label: "ID эксперимента", value: str((job as any)?.experiment_id) || "—" },
                          { label: "ID запуска эксперимента", value: str((job as any)?.experiment_run_id) || "—" },
                        ]}
                      />
                    </div>
                  </details>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}