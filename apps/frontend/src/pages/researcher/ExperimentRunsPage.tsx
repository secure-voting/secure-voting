import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../shared/api/client";
import type { ExperimentRunItem } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { ProtocolTimeline } from "../../shared/ui/ProtocolTimeline";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { SimpleBarChart } from "../../shared/ui/SimpleBarChart";
import { styles } from "../../shared/ui/styles";
import { downloadCsvFile, downloadJsonFile, downloadTextFile } from "../../shared/utils/export";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function prettyValue(value: unknown) {
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

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function runId(item: ExperimentRunItem, index: number) {
  const id = (item as any)?.id;
  return typeof id === "string" && id.trim() ? id.trim() : `run-${index}`;
}

function runStatus(item: ExperimentRunItem) {
  const s = (item as any)?.status;
  return typeof s === "string" ? s : "unknown";
}

function nowTimeLabel() {
  const d = new Date();
  return d.toLocaleTimeString();
}

function summaryItems(value: unknown): Array<{ label: string; value: React.ReactNode }> {
  if (!isObject(value)) return [];
  return Object.entries(value)
    .slice(0, 12)
    .map(([key, val]) => ({
      label: key,
      value: prettyValue(val),
    }));
}

function numericItems(value: unknown): Array<{ label: string; value: number }> {
  if (!isObject(value)) return [];
  return Object.entries(value)
    .filter(([, val]) => typeof val === "number" && Number.isFinite(val))
    .slice(0, 12)
    .map(([key, val]) => ({
      label: key,
      value: Number(val),
    }));
}

function numberFromRecordByKeys(value: unknown, keys: string[]): number | null {
  if (!isObject(value)) return null;

  const wanted = new Set(keys.map((key) => key.toLowerCase()));
  for (const [key, raw] of Object.entries(value)) {
    if (!wanted.has(key.toLowerCase())) continue;
    if (typeof raw === "number" && Number.isFinite(raw)) {
      return raw;
    }
  }

  return null;
}

function canonicalIndicators(result: unknown) {
  const rec = isObject(result) ? result : null;
  const metrics = rec && isObject(rec.metrics) ? rec.metrics : null;
  const timings = rec && isObject(rec.timings) ? rec.timings : null;

  const timeSecondsDirect =
    numberFromRecordByKeys(timings, ["total_seconds", "duration_seconds", "elapsed_seconds", "time_seconds"]) ??
    numberFromRecordByKeys(metrics, ["total_seconds", "duration_seconds", "elapsed_seconds", "time_seconds"]);

  const timeMilliseconds =
    numberFromRecordByKeys(timings, ["total_ms", "duration_ms", "elapsed_ms", "time_ms"]) ??
    numberFromRecordByKeys(metrics, ["total_ms", "duration_ms", "elapsed_ms", "time_ms"]);

  const memoryBytesDirect =
    numberFromRecordByKeys(metrics, ["peak_memory_bytes", "max_memory_bytes", "memory_bytes", "ram_bytes"]);

  const memoryMb =
    numberFromRecordByKeys(metrics, ["peak_memory_mb", "max_memory_mb", "memory_mb", "ram_mb"]);

  const speedPerSecond =
    numberFromRecordByKeys(metrics, ["throughput_ops_per_sec", "ops_per_sec", "ballots_per_sec", "speed_per_sec", "throughput"]);

  return {
    timeSeconds: timeSecondsDirect ?? (timeMilliseconds != null ? timeMilliseconds / 1000 : null),
    memoryBytes: memoryBytesDirect ?? (memoryMb != null ? memoryMb * 1024 * 1024 : null),
    speedPerSecond,
  };
}

function formatSeconds(value: number | null) {
  return value == null ? "—" : `${value.toFixed(3)} s`;
}

function formatBytes(value: number | null) {
  if (value == null) return "—";
  if (value >= 1024 * 1024 * 1024) return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GiB`;
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(2)} MiB`;
  if (value >= 1024) return `${(value / 1024).toFixed(2)} KiB`;
  return `${value.toFixed(0)} B`;
}

function formatSpeed(value: number | null) {
  return value == null ? "—" : `${value.toFixed(3)} /s`;
}

function winnerList(value: unknown): string[] {
  if (Array.isArray(value)) return value.map((item) => prettyValue(item));
  if (isObject(value)) {
    if (Array.isArray(value.winners)) {
      return value.winners.map((item) => prettyValue(item));
    }
    if (Array.isArray(value.items)) {
      return value.items.map((item) => prettyValue(item));
    }
  }
  if (value != null) return [prettyValue(value)];
  return [];
}

function runDurationSeconds(item: ExperimentRunItem): number | null {
  const startedAt = (item as any)?.started_at;
  const finishedAt = (item as any)?.finished_at;

  if (typeof startedAt !== "string" || typeof finishedAt !== "string") return null;

  const startMs = Date.parse(startedAt);
  const finishMs = Date.parse(finishedAt);

  if (!Number.isFinite(startMs) || !Number.isFinite(finishMs)) return null;
  if (finishMs < startMs) return null;

  return (finishMs - startMs) / 1000;
}

function resultSummaryCsvRows(result: unknown) {
  const rec = isObject(result) ? result : null;
  const rows: Array<Record<string, unknown>> = [];

  const canonical = canonicalIndicators(result);

  rows.push({
    section: "summary",
    key: "time_seconds",
    value: canonical.timeSeconds != null ? canonical.timeSeconds : "",
  });
  rows.push({
    section: "summary",
    key: "memory_bytes",
    value: canonical.memoryBytes != null ? canonical.memoryBytes : "",
  });
  rows.push({
    section: "summary",
    key: "speed_per_second",
    value: canonical.speedPerSecond != null ? canonical.speedPerSecond : "",
  });

  const winners = winnerList(rec?.winners);
  winners.forEach((winner, index) => {
    rows.push({
      section: "winners",
      key: index + 1,
      value: winner,
    });
  });

  if (isObject(rec?.metrics)) {
    Object.entries(rec.metrics).forEach(([key, value]) => {
      rows.push({
        section: "metrics",
        key,
        value: prettyValue(value),
      });
    });
  }

  if (isObject(rec?.timings)) {
    Object.entries(rec.timings).forEach(([key, value]) => {
      rows.push({
        section: "timings",
        key,
        value: prettyValue(value),
      });
    });
  }

  if (isObject(rec?.artifacts)) {
    Object.entries(rec.artifacts).forEach(([key, value]) => {
      rows.push({
        section: "artifacts",
        key,
        value: prettyValue(value),
      });
    });
  }

  return rows;
}

function buildRunReportText(run: ExperimentRunItem | null, result: unknown) {
  const runRec = run && typeof run === "object" ? (run as Record<string, unknown>) : null;
  const resultRec = result && typeof result === "object" ? (result as Record<string, unknown>) : null;

  const winners = winnerList(resultRec?.winners);
  const metrics = isObject(resultRec?.metrics) ? Object.entries(resultRec.metrics) : [];
  const timings = isObject(resultRec?.timings) ? Object.entries(resultRec.timings) : [];
  const artifacts = isObject(resultRec?.artifacts) ? Object.entries(resultRec.artifacts) : [];
  const canonical = canonicalIndicators(result);

  const lines: string[] = [];

  lines.push("Experiment run report");
  lines.push("");

  lines.push("Run:");
  lines.push(`- id: ${prettyValue(runRec?.id)}`);
  lines.push(`- status: ${prettyValue(runRec?.status)}`);
  lines.push(`- experiment_id: ${prettyValue(runRec?.experiment_id)}`);
  lines.push(`- dataset_id: ${prettyValue(runRec?.dataset_id)}`);
  lines.push(`- started_at: ${prettyValue(runRec?.started_at)}`);
  lines.push(`- finished_at: ${prettyValue(runRec?.finished_at)}`);
  lines.push("");

  lines.push("Canonical indicators:");
  lines.push(`- time_seconds: ${canonical.timeSeconds != null ? canonical.timeSeconds.toFixed(3) : "—"}`);
  lines.push(`- memory_bytes: ${canonical.memoryBytes != null ? canonical.memoryBytes.toFixed(0) : "—"}`);
  lines.push(`- speed_per_second: ${canonical.speedPerSecond != null ? canonical.speedPerSecond.toFixed(3) : "—"}`);
  lines.push("");
  lines.push("Winners:");
  if (winners.length > 0) {
    winners.forEach((winner, index) => lines.push(`${index + 1}. ${winner}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Metrics:");
  if (metrics.length > 0) {
    metrics.forEach(([key, value]) => lines.push(`- ${key}: ${prettyValue(value)}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Timings:");
  if (timings.length > 0) {
    timings.forEach(([key, value]) => lines.push(`- ${key}: ${prettyValue(value)}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Artifacts:");
  if (artifacts.length > 0) {
    artifacts.forEach(([key, value]) => lines.push(`- ${key}: ${prettyValue(value)}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Full result JSON:");
  try {
    lines.push(JSON.stringify(result ?? {}, null, 2));
  } catch {
    lines.push(String(result));
  }
  lines.push("");

  return `${lines.join("\n")}`;
}

export function ExperimentRunsPage() {
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [items, setItems] = useState<ExperimentRunItem[]>([]);
  const [selected, setSelected] = useState<ExperimentRunItem | null>(null);
  const [selectedResult, setSelectedResult] = useState<unknown>(null);

  const [experimentIdFilter, setExperimentIdFilter] = useState("");
  const [batchPayload, setBatchPayload] = useState("{\n  \n}");

  const [pollingOn, setPollingOn] = useState(true);
  const [pollEverySec, setPollEverySec] = useState(5);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [resultLoading, setResultLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const listAbortRef = useRef<AbortController | null>(null);
  const detailAbortRef = useRef<AbortController | null>(null);
  const resultAbortRef = useRef<AbortController | null>(null);

  const timerRef = useRef<number | null>(null);
  const prevStatusRef = useRef<Map<string, string>>(new Map());

  const loadList = useCallback(
    async (silent?: boolean) => {
      if (!token) return;

      listAbortRef.current?.abort();
      const ac = new AbortController();
      listAbortRef.current = ac;

      if (!silent) setLoading(true);
      setErr(null);

      try {
        const list = await api.experimentRuns.list(
          token,
          {
            experiment_id: experimentIdFilter.trim() || undefined,
          },
          ac.signal
        );

        const prev = prevStatusRef.current;
        const next = new Map<string, string>();

        list.forEach((run, index) => {
          const id = runId(run, index);
          const s = runStatus(run);
          next.set(id, s);

          const prevS = prev.get(id);
          if (prevS && prevS !== s) {
            if (s === "done") {
              addNotification({
                kind: "success",
                title: "Запуск завершён",
                message: id,
              });
            } else if (s === "error") {
              addNotification({
                kind: "error",
                title: "Ошибка запуска",
                message: id,
              });
            }
          }
        });

        prevStatusRef.current = next;

        setItems(list);
        setLastUpdatedAt(nowTimeLabel());
      } catch (e: any) {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) setToken(null);
        setErr(e?.message || "Не удалось загрузить experiment runs");
        setItems([]);
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [token, experimentIdFilter, setToken, addNotification]
  );

  useEffect(() => {
    loadList(false);
    return () => {
      listAbortRef.current?.abort();
      detailAbortRef.current?.abort();
      resultAbortRef.current?.abort();
      if (timerRef.current) window.clearInterval(timerRef.current);
      timerRef.current = null;
    };
  }, [loadList]);

  useEffect(() => {
    if (!pollingOn) {
      if (timerRef.current) window.clearInterval(timerRef.current);
      timerRef.current = null;
      return;
    }

    const intervalMs = Math.max(2, Number.isFinite(pollEverySec) ? pollEverySec : 5) * 1000;

    if (timerRef.current) window.clearInterval(timerRef.current);
    timerRef.current = window.setInterval(() => {
      loadList(true);
    }, intervalMs);

    return () => {
      if (timerRef.current) window.clearInterval(timerRef.current);
      timerRef.current = null;
    };
  }, [pollingOn, pollEverySec, loadList]);

  const loadDetail = async (id: string) => {
    if (!token) return;

    detailAbortRef.current?.abort();
    const ac = new AbortController();
    detailAbortRef.current = ac;

    setDetailLoading(true);
    setErr(null);

    try {
      const item = await api.experimentRuns.get(token, id, ac.signal);
      setSelected(item);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить запуск");
      setSelected(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const loadResult = async (id: string) => {
    if (!token) return;

    resultAbortRef.current?.abort();
    const ac = new AbortController();
    resultAbortRef.current = ac;

    setResultLoading(true);
    setErr(null);

    try {
      const result = await api.experimentRuns.result(token, id, ac.signal);
      setSelectedResult(result);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить результат запуска");
      setSelectedResult(null);
    } finally {
      setResultLoading(false);
    }
  };

  const handleDownload = async (id: string) => {
    if (!token) return;

    setErr(null);
    setInfo(null);

    try {
      const { blob, filename } = await api.experimentRuns.download(token, id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);

      setInfo("Файл результата успешно скачан");
      addNotification({
        kind: "success",
        title: "Результат запуска скачан",
        message: `Файл ${filename} успешно подготовлен к загрузке`,
      });
    } catch (e: any) {
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось скачать результат");
    }
  };

  const handleBatchCreate = async () => {
    if (!token) return;

    setErr(null);
    setInfo(null);

    try {
      const parsed = JSON.parse(batchPayload);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error("Payload должен быть JSON-объектом");
      }

      const created = await api.experimentRuns.batch(token, parsed as Record<string, unknown>);
      setInfo(`Создано запусков: ${created.length}`);

      addNotification({
        kind: "success",
        title: "Batch запусков создан",
        message: `Создано запусков: ${created.length}`,
      });

      await loadList(false);
    } catch (e: any) {
      if (e?.status === 401) setToken(null);
      if (e instanceof SyntaxError) {
        setErr("Batch payload содержит невалидный JSON");
      } else {
        setErr(e?.message || "Не удалось создать batch запусков");
      }
    }
  };

  const selectedRecord =
    selected && typeof selected === "object" ? (selected as Record<string, unknown>) : null;
  const resultRecord =
    selectedResult && typeof selectedResult === "object" ? (selectedResult as Record<string, unknown>) : null;

  const metricsSummary = useMemo(() => summaryItems(resultRecord?.metrics), [resultRecord]);
  const timingsSummary = useMemo(() => summaryItems(resultRecord?.timings), [resultRecord]);
  const artifactsSummary = useMemo(() => summaryItems(resultRecord?.artifacts), [resultRecord]);
  const winners = useMemo(() => winnerList(resultRecord?.winners), [resultRecord]);
  const canonical = useMemo(() => canonicalIndicators(selectedResult), [selectedResult]);

  const metricChartItems = useMemo(() => numericItems(resultRecord?.metrics), [resultRecord]);
  const timingsChartItems = useMemo(() => numericItems(resultRecord?.timings), [resultRecord]);

  const counters = useMemo(() => {
    const m: Record<string, number> = {};
    for (const it of items) {
      const s = runStatus(it);
      m[s] = (m[s] || 0) + 1;
    }
    return m;
  }, [items]);

  const statusChartItems = useMemo(
    () =>
      Object.entries(counters).map(([label, value]) => ({
        label,
        value,
      })),
    [counters]
  );

  const durationChartItems = useMemo(
    () =>
      items
        .map((item, index) => {
          const duration = runDurationSeconds(item);
          if (duration == null) return null;
          return {
            label: runId(item, index).slice(0, 12),
            value: duration,
          };
        })
        .filter(Boolean) as Array<{ label: string; value: number }>,
    [items]
  );

  const exportRunsCsv = () => {
    downloadCsvFile(
      "experiment-runs.csv",
      items.map((item, index) => ({
        id: runId(item, index),
        status: runStatus(item),
        experiment_id: prettyValue((item as any)?.experiment_id),
        dataset_id: prettyValue((item as any)?.dataset_id),
        started_at: prettyValue((item as any)?.started_at),
        finished_at: prettyValue((item as any)?.finished_at),
      }))
    );
  };

    const exportRunsJson = () => {
    downloadJsonFile("experiment-runs.json", items);
  };

  const exportResultSummaryCsv = () => {
    if (!selectedResult) return;
    downloadCsvFile("experiment-run-result-summary.csv", resultSummaryCsvRows(selectedResult));
  };

  const exportResultReportTxt = () => {
    if (!selectedResult) return;
    downloadTextFile("experiment-run-report.txt", buildRunReportText(selected, selectedResult));
  };

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
          <h2 style={{ margin: 0 }}>Experiment runs</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => loadList(false)} disabled={loading}>
              Обновить
            </button>
            <button style={styles.btn} onClick={exportRunsCsv} disabled={items.length === 0}>
              Export CSV
            </button>
            <button style={styles.btn} onClick={exportRunsJson} disabled={items.length === 0}>
              Export runs JSON
            </button>
            {selectedResult ? (
              <>
                <button
                  style={styles.btn}
                  onClick={() => downloadJsonFile("experiment-run-result.json", selectedResult)}
                >
                  Export result JSON
                </button>
                <button style={styles.btn} onClick={exportResultSummaryCsv}>
                  Export summary CSV
                </button>
                <button style={styles.btn} onClick={exportResultReportTxt}>
                  Export report TXT
                </button>
              </>
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />
        {info ? (
          <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0", marginBottom: 12 }}>
            {info}
          </div>
        ) : null}

        <div style={{ marginTop: 12, ...styles.grid2 }}>
          <div>
            <label>Filter by experiment_id</label>
            <input
              style={styles.input}
              value={experimentIdFilter}
              onChange={(e) => setExperimentIdFilter(e.target.value)}
            />
          </div>
          <div style={{ display: "flex", alignItems: "end" }}>
            <button style={styles.btnPrimary} onClick={() => loadList(false)} disabled={loading}>
              Применить фильтр
            </button>
          </div>
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 10, flexWrap: "wrap", alignItems: "center" }}>
          <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <input type="checkbox" checked={pollingOn} onChange={(e) => setPollingOn(e.target.checked)} />
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
            <Badge key={k} text={`${k}: ${v}`} />
          ))}
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 12 }}>
          <SimpleBarChart
            title="Распределение запусков по статусам"
            items={statusChartItems}
            emptyText="Нет данных по статусам"
          />

          <SimpleBarChart
            title="Длительность завершённых запусков (сек)"
            items={durationChartItems}
            emptyText="Недостаточно данных по времени выполнения"
            valueFormatter={(value) => `${value.toFixed(2)} s`}
          />
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && items.length === 0 ? <div style={styles.muted}>Список пуст</div> : null}

          {items.map((item, index) => {
            const id = runId(item, index);
            const status = runStatus(item);
            const experimentId = prettyValue((item as any)?.experiment_id ?? "");
            const datasetId = prettyValue((item as any)?.dataset_id ?? "");

            return (
              <div key={id} style={{ ...styles.card, padding: 12 }}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
                  <div>
                    <div style={{ fontWeight: 700 }}>{id}</div>
                    <div style={styles.muted}>experiment_id: {experimentId || "—"}</div>
                    <div style={styles.muted}>dataset_id: {datasetId || "—"}</div>
                  </div>
                  <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <Badge text={status} />
                  </div>
                </div>

                <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                  started_at: {prettyValue((item as any)?.started_at)} · finished_at: {prettyValue((item as any)?.finished_at)}
                </div>

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btnPrimary} onClick={() => loadDetail(id)} disabled={detailLoading}>
                    Открыть
                  </button>
                  <button style={styles.btn} onClick={() => loadResult(id)} disabled={resultLoading}>
                    Result
                  </button>
                  <button style={styles.btn} onClick={() => handleDownload(id)}>
                    Download
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Batch create</h3>
          <label>Batch payload (JSON object)</label>
          <textarea
            style={{
              ...styles.input,
              minHeight: 220,
              fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
            }}
            value={batchPayload}
            onChange={(e) => setBatchPayload(e.target.value)}
          />

          <div style={{ height: 12 }} />

          <button style={styles.btnPrimary} onClick={handleBatchCreate}>
            Запустить batch
          </button>
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Карточка запуска</h3>
          {detailLoading ? <div style={styles.muted}>Загрузка…</div> : null}

          {selectedRecord ? (
            <div style={{ display: "grid", gap: 12 }}>
              <div>
                <div style={{ fontWeight: 700, fontSize: 18 }}>{prettyValue(selectedRecord.id)}</div>
                <div style={styles.muted}>Информация о выполнении запуска</div>
              </div>

              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                {selectedRecord.status != null ? <Badge text={prettyValue(selectedRecord.status)} /> : null}
              </div>

              <KeyValueList
                items={[
                  { label: "Experiment ID", value: prettyValue(selectedRecord.experiment_id) },
                  { label: "Dataset ID", value: prettyValue(selectedRecord.dataset_id) },
                  { label: "Started at", value: prettyValue(selectedRecord.started_at) },
                  { label: "Finished at", value: prettyValue(selectedRecord.finished_at) },
                ]}
              />

              <div>
                <h4 style={{ marginBottom: 8 }}>Поля запуска</h4>
                <JsonBlock value={selectedRecord} />
              </div>
            </div>
          ) : (
            <div style={styles.muted}>Ничего не выбрано</div>
          )}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Результат запуска</h3>
        {resultLoading ? <div style={styles.muted}>Загрузка…</div> : null}

        {resultRecord ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div>
              <div style={{ fontWeight: 700 }}>Результат вычисления</div>
              <div style={styles.muted}>Данные, возвращённые сервисом результата запуска</div>
            </div>
            <div>
              <h4 style={{ marginBottom: 8 }}>Ключевые показатели</h4>
              <SummaryGrid
                items={[
                  { label: "Time", value: formatSeconds(canonical.timeSeconds) },
                  { label: "Memory", value: formatBytes(canonical.memoryBytes) },
                  { label: "Speed", value: formatSpeed(canonical.speedPerSecond) },
                ]}
              />
            </div>

            <div>
              <h4 style={{ marginBottom: 8 }}>Победители</h4>
              {winners.length > 0 ? (
                <div style={{ ...styles.card, background: "#f9fafb" }}>
                  <ol style={{ margin: "0 0 0 18px" }}>
                    {winners.map((winner, index) => (
                      <li key={`${winner}-${index}`}>{winner}</li>
                    ))}
                  </ol>
                </div>
              ) : (
                <div style={styles.muted}>Победители не указаны</div>
              )}
            </div>

            <div>
              <h4 style={{ marginBottom: 8 }}>Метрики</h4>
              {metricsSummary.length > 0 ? (
                <SummaryGrid items={metricsSummary} />
              ) : resultRecord.metrics != null ? (
                <JsonBlock value={resultRecord.metrics} />
              ) : (
                <div style={styles.muted}>Метрики отсутствуют</div>
              )}
            </div>

            <SimpleBarChart
              title="Числовые метрики результата"
              items={metricChartItems}
              emptyText="Нет числовых метрик для графика"
            />

            <div>
              <h4 style={{ marginBottom: 8 }}>Временные показатели</h4>
              {timingsSummary.length > 0 ? (
                <SummaryGrid items={timingsSummary} />
              ) : resultRecord.timings != null ? (
                <JsonBlock value={resultRecord.timings} />
              ) : (
                <div style={styles.muted}>Timings отсутствуют</div>
              )}
            </div>

            <SimpleBarChart
              title="Числовые временные показатели"
              items={timingsChartItems}
              emptyText="Нет числовых timing-показателей для графика"
            />

            <div>
              <h4 style={{ marginBottom: 8 }}>Артефакты</h4>
              {artifactsSummary.length > 0 ? (
                <SummaryGrid items={artifactsSummary} />
              ) : resultRecord.artifacts != null ? (
                <JsonBlock value={resultRecord.artifacts} />
              ) : (
                <div style={styles.muted}>Artifacts отсутствуют</div>
              )}
            </div>

            <div>
              <h4 style={{ marginBottom: 8 }}>Протокол шагов</h4>
              {resultRecord.protocol != null ? (
                <ProtocolTimeline protocol={resultRecord.protocol} />
              ) : (
                <div style={styles.muted}>Протокол отсутствует</div>
              )}
            </div>

            <div>
              <h4 style={{ marginBottom: 8 }}>Полный JSON</h4>
              <JsonBlock value={selectedResult} />
            </div>
          </div>
        ) : (
          <div style={styles.muted}>Ничего не загружено</div>
        )}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug list</h3>
          <JsonBlock value={items} />
        </div>
      ) : null}
    </div>
  );
}