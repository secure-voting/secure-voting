import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { DatasetListItem, Experiment, ExperimentRunItem, ExperimentRunResultResp } from "../../shared/api/types";
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
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";
import {
  downloadCsvFile,
  downloadJsonFile,
  downloadPdfTextFile,
  downloadTextFile,
  downloadXlsxFile,
} from "../../shared/utils/export";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

type RunsLocationState = {
  createdRuns?: Array<{
    rule: string;
    experimentId: string;
    runId: string;
    jobId: string;
  }>;
  autoOpenRunId?: string;
  experimentIdFilter?: string;
};

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

function shortId(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
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

function datasetCandidatesCount(value: unknown, datasetMap?: Record<string, DatasetListItem>) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";

  const dataset = datasetMap?.[raw];
  const candidates = (dataset as any)?.candidates;

  if (Array.isArray(candidates)) return String(candidates.length);

  return "—";
}

function formatBallotFormat(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    approval: "Одобрение",
    ranking: "Ранжирование",
    score: "Оценивание",
  };

  return labels[raw] || raw || "—";
}

function runId(item: ExperimentRunItem, index: number) {
  const id = (item as any)?.id;
  return typeof id === "string" && id.trim() ? id.trim() : `run-${index}`;
}

function runStatus(item: ExperimentRunItem) {
  const s = (item as any)?.status;
  return typeof s === "string" ? s : "unknown";
}

function runStatusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    queued: "В очереди",
    running: "Выполняется",
    done: "Завершен",
    error: "Ошибка",
    unknown: "Статус неизвестен",
  };

  return labels[raw] || raw || "Статус неизвестен";
}

function datasetLabel(value: unknown, datasetMap?: Record<string, DatasetListItem>) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "Набор данных не указан";

  const dataset = datasetMap?.[raw];
  if (dataset?.name && dataset.name.trim()) {
    return dataset.name.trim();
  }

  return `Набор данных ${shortId(raw)}`;
}

function experimentParamsObject(value: unknown): Record<string, unknown> {
  if (!value) return {};

  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      return isObject(parsed) ? parsed : {};
    } catch {
      return {};
    }
  }

  if (isObject(value)) {
    return value;
  }

  return {};
}

function experimentRuleId(exp?: Experiment | null) {
  const params = experimentParamsObject(exp?.params);
  const rule = params.tally_rule;
  return typeof rule === "string" ? rule : "";
}

function experimentRuleLabel(exp?: Experiment | null) {
  const id = experimentRuleId(exp);
  return id ? tallyRuleLabel(id) : "—";
}

function experimentBallotFormat(exp?: Experiment | null) {
  const params = experimentParamsObject(exp?.params);
  const value = params.ballot_format;
  return typeof value === "string" ? value : "";
}

function experimentRunExperiment(item: ExperimentRunItem, experimentMap: Record<string, Experiment>) {
  const experimentID = typeof item.experiment_id === "string" ? item.experiment_id : "";
  return experimentID ? experimentMap[experimentID] ?? null : null;
}

function runComputeDurationSeconds(
  item: ExperimentRunItem,
  index: number,
  resultMap: Record<string, ExperimentRunResultResp>
): number | null {
  const result = resultMap[runId(item, index)];
  if (!result) return null;

  const canonical = canonicalIndicators(result);
  return canonical.timeSeconds;
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

function experimentRunTitle(
  item: ExperimentRunItem,
  index: number,
  experimentMap: Record<string, Experiment>
) {
  const exp = experimentRunExperiment(item, experimentMap);
  const ruleLabel = experimentRuleLabel(exp);

  if (ruleLabel && ruleLabel !== "—") {
    return `Запуск ${ruleLabel}`;
  }

  return `Запуск эксперимента ${index + 1}`;
}

function experimentRunSubtitle(
  item: ExperimentRunItem,
  experimentMap: Record<string, Experiment>,
  datasetMap: Record<string, DatasetListItem>
) {
  const parts: string[] = [];

  const exp = experimentRunExperiment(item, experimentMap);
  const ruleID = experimentRuleId(exp);
  const ballotFormat = experimentBallotFormat(exp);

  if (ruleID) {
    parts.push(`правило ${tallyRuleLabel(ruleID)}`);
  }

  if (ballotFormat) {
    parts.push(formatBallotFormat(ballotFormat));
  }

  parts.push(datasetLabel(item.dataset_id, datasetMap));

  const duration = runDurationSeconds(item);
  if (duration != null) {
    parts.push(`время выполнения ${duration.toFixed(3)} s`);
  }

  return parts.join(" · ");
}

function nowTimeLabel() {
  return new Date().toLocaleTimeString();
}

function humanizeMetricKey(key: string) {
  return key
    .replace(/_/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function metricLabel(key: string): string {
  const labels: Record<string, string> = {
    total_ballots: "Всего бюллетеней",
    valid_ballots: "Корректных бюллетеней",
    invalid_ballots: "Некорректных бюллетеней",
    candidates_count: "Число кандидатов",
    voter_count: "Число избирателей",
    voters_count: "Число избирателей",
    ballots_count: "Число бюллетеней",
    winner_count: "Число победителей",
    winners_count: "Число победителей",
    committee_size: "Размер комитета",
    rounds_count: "Число раундов",
    iterations_count: "Число итераций",
    winner_score: "Баллы победителя",
    runner_up_score: "Баллы второго места",
    margin: "Отрыв",
    average_score: "Средний балл",
    median_score: "Медианный балл",
    min_score: "Минимальный балл",
    max_score: "Максимальный балл",
    score_sum: "Сумма баллов",
    score_mean: "Среднее значение баллов",
    score_variance: "Дисперсия баллов",
    score_stddev: "Стандартное отклонение баллов",
    total_seconds: "Общее время, с",
    duration_seconds: "Длительность, с",
    elapsed_seconds: "Прошло времени, с",
    time_seconds: "Время, с",
    total_ms: "Общее время, мс",
    duration_ms: "Длительность, мс",
    elapsed_ms: "Прошло времени, мс",
    time_ms: "Время, мс",
    peak_memory_bytes: "Пиковая память, байт",
    max_memory_bytes: "Максимальная память, байт",
    memory_bytes: "Память, байт",
    ram_bytes: "ОЗУ, байт",
    peak_memory_mb: "Пиковая память, МиБ",
    max_memory_mb: "Максимальная память, МиБ",
    memory_mb: "Память, МиБ",
    ram_mb: "ОЗУ, МиБ",
    throughput_ops_per_sec: "Пропускная способность, операций/с",
    ops_per_sec: "Операций/с",
    ballots_per_sec: "Бюллетеней/с",
    speed_per_sec: "Скорость, 1/с",
    throughput: "Пропускная способность",
    comparisons_count: "Число сравнений",
    pairwise_comparisons_count: "Число попарных сравнений",
    ties_count: "Число ничьих",
    eliminated_count: "Исключено кандидатов",
    selected_count: "Выбрано кандидатов",
    memory_rss_bytes: "RSS-память, байт",
    cpu_usage_percent: "CPU, %",
    throughput_ballots_per_sec: "Бюллетеней/с",
    tie_detected: "Обнаружена ничья",
    normalized_margin: "Нормированный отрыв",
    round_sizes: "Размеры раундов",
    candidate_scores_final: "Финальные оценки кандидатов",
  };

  return labels[key] || humanizeMetricKey(key) || key;
}

function summaryItems(value: unknown): Array<{ label: string; value: React.ReactNode }> {
  if (!isObject(value)) return [];

  const summary = isObject(value.summary) ? value.summary : value;

  return Object.entries(summary)
    .filter(([, val]) => val != null && !isObject(val) && !Array.isArray(val))
    .slice(0, 12)
    .map(([key, val]) => ({
      label: metricLabel(key),
      value: prettyValue(val),
    }));
}

function numericItems(value: unknown): Array<{ label: string; value: number }> {
  if (!isObject(value)) return [];

  const series = isObject(value.series) ? value.series : null;
  const numeric = isObject(value.numeric) ? value.numeric : null;
  const summary = isObject(value.summary) ? value.summary : null;

  if (series && Array.isArray(series.candidate_scores_final)) {
    const scalarItems = series.candidate_scores_final
      .filter(
        (item) =>
          isObject(item) &&
          typeof item.value === "number" &&
          Number.isFinite(item.value)
      )
      .map((item) => ({
        label:
          typeof item.candidate_name === "string" && item.candidate_name.trim()
            ? item.candidate_name.trim()
            : typeof item.candidate_id === "string" && item.candidate_id.trim()
              ? item.candidate_id.trim()
              : "Кандидат",
        value: Number(item.value),
      }));

    if (scalarItems.length > 0) {
      return scalarItems;
    }
  }

  const source = numeric ?? summary ?? value;

  return Object.entries(source)
    .filter(([, val]) => typeof val === "number" && Number.isFinite(val))
    .slice(0, 12)
    .map(([key, val]) => ({
      label: metricLabel(key),
      value: Number(val),
    }));
}

function vectorChartItems(value: unknown): Array<{ label: string; value: number }> {
  if (!isObject(value)) return [];

  const series = isObject(value.series) ? value.series : null;
  if (!series || !Array.isArray(series.candidate_scores_final)) return [];

  const items: Array<{ label: string; value: number }> = [];

  for (const raw of series.candidate_scores_final) {
    if (!isObject(raw)) continue;
    if (!Array.isArray(raw.values)) continue;
    if (!raw.values.every((v) => typeof v === "number" && Number.isFinite(v))) continue;

    const baseLabel =
      typeof raw.candidate_name === "string" && raw.candidate_name.trim()
        ? raw.candidate_name.trim()
        : typeof raw.candidate_id === "string" && raw.candidate_id.trim()
          ? raw.candidate_id.trim()
          : "Кандидат";

    raw.values.forEach((v, index) => {
      items.push({
        label: `${baseLabel} [${index + 1}]`,
        value: Number(v),
      });
    });
  }

  return items;
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
    numberFromRecordByKeys(timings, [
      "total_seconds",
      "duration_seconds",
      "elapsed_seconds",
      "time_seconds",
    ]) ??
    numberFromRecordByKeys(metrics, [
      "total_seconds",
      "duration_seconds",
      "elapsed_seconds",
      "time_seconds",
    ]);

  const timeMilliseconds =
    numberFromRecordByKeys(timings, [
      "total_ms",
      "duration_ms",
      "elapsed_ms",
      "time_ms",
    ]) ??
    numberFromRecordByKeys(metrics, [
      "total_ms",
      "duration_ms",
      "elapsed_ms",
      "time_ms",
    ]);

  const memoryBytesDirect =
    numberFromRecordByKeys(timings, [
      "memory_rss_bytes",
      "peak_memory_bytes",
      "max_memory_bytes",
      "memory_bytes",
      "ram_bytes",
    ]) ??
    numberFromRecordByKeys(metrics, [
      "memory_rss_bytes",
      "peak_memory_bytes",
      "max_memory_bytes",
      "memory_bytes",
      "ram_bytes",
    ]);

  const memoryMb =
    numberFromRecordByKeys(timings, [
      "peak_memory_mb",
      "max_memory_mb",
      "memory_mb",
      "ram_mb",
    ]) ??
    numberFromRecordByKeys(metrics, [
      "peak_memory_mb",
      "max_memory_mb",
      "memory_mb",
      "ram_mb",
    ]);

  const speedPerSecond =
    numberFromRecordByKeys(timings, [
      "throughput_ballots_per_sec",
      "throughput_ops_per_sec",
      "ops_per_sec",
      "ballots_per_sec",
      "speed_per_sec",
      "throughput",
    ]) ??
    numberFromRecordByKeys(metrics, [
      "throughput_ballots_per_sec",
      "throughput_ops_per_sec",
      "ops_per_sec",
      "ballots_per_sec",
      "speed_per_sec",
      "throughput",
    ]);

  const cpuUsagePercent =
    numberFromRecordByKeys(timings, ["cpu_usage_percent"]) ??
    numberFromRecordByKeys(metrics, ["cpu_usage_percent"]);

  const ballotsCount =
    numberFromRecordByKeys(timings, ["ballots_count", "total_ballots"]) ??
    numberFromRecordByKeys(metrics, ["ballots_count", "total_ballots"]);

  return {
    timeSeconds: timeSecondsDirect ?? (timeMilliseconds != null ? timeMilliseconds / 1000 : null),
    memoryBytes: memoryBytesDirect ?? (memoryMb != null ? memoryMb * 1024 * 1024 : null),
    speedPerSecond,
    cpuUsagePercent,
    ballotsCount,
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

function formatPercent(value: number | null) {
  return value == null ? "—" : `${value.toFixed(2)} %`;
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

function resultErrorText(value: unknown): string | null {
  if (!isObject(value)) return null;
  const text = value.error_text;
  return typeof text === "string" && text.trim() ? text.trim() : null;
}

function resultMethod(value: unknown) {
  if (!isObject(value)) return "";
  const method = value.method;
  return typeof method === "string" && method.trim() ? method.trim() : "";
}

function resultParams(value: unknown) {
  if (!isObject(value)) return null;
  return value.params ?? null;
}

function resultStatus(value: unknown) {
  if (!isObject(value)) return "";
  const status = value.status;
  return typeof status === "string" ? status : "";
}

function resultSummaryCsvRows(result: unknown) {
  const rec = isObject(result) ? result : null;
  const rows: Array<Record<string, unknown>> = [];

  const canonical = canonicalIndicators(result);

  rows.push({
    section: "summary",
    key: "status",
    value: resultStatus(result),
  });
  rows.push({
    section: "summary",
    key: "method",
    value: resultMethod(result),
  });
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
  rows.push({
    section: "summary",
    key: "cpu_usage_percent",
    value: canonical.cpuUsagePercent != null ? canonical.cpuUsagePercent : "",
  });

  rows.push({
    section: "summary",
    key: "ballots_count",
    value: canonical.ballotsCount != null ? canonical.ballotsCount : "",
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

function buildRunReportText(
  run: ExperimentRunItem | null,
  result: unknown,
  experiment: Experiment | null,
  datasetMap: Record<string, DatasetListItem>
) {
  const runRec = run && typeof run === "object" ? (run as Record<string, unknown>) : null;
  const resultRec = isObject(result) ? result : null;
  const experimentParams = experimentParamsObject(experiment?.params);

  const winners = winnerList(resultRec?.winners);
  const metrics = isObject(resultRec?.metrics) ? Object.entries(resultRec.metrics) : [];
  const timings = isObject(resultRec?.timings) ? Object.entries(resultRec.timings) : [];
  const artifacts = isObject(resultRec?.artifacts) ? Object.entries(resultRec.artifacts) : [];
  const canonical = canonicalIndicators(result);

  const lines: string[] = [];

  lines.push("Отчет по запуску эксперимента");
  lines.push("");

  lines.push("Запуск:");
  lines.push(`- status: ${runStatusLabel(runRec?.status)}`);
  lines.push(`- rule: ${experimentRuleLabel(experiment)}`);
  lines.push(`- ballot_format: ${formatBallotFormat(experimentBallotFormat(experiment))}`);
  lines.push(`- dataset: ${datasetLabel(runRec?.dataset_id, datasetMap)}`);
  lines.push(`- started_at: ${prettyValue(runRec?.started_at)}`);
  lines.push(`- finished_at: ${prettyValue(runRec?.finished_at)}`);
  lines.push(`- technical_id: ${prettyValue(runRec?.id)}`);
  lines.push(`- experiment_id: ${prettyValue(runRec?.experiment_id)}`);
  lines.push(`- dataset_id: ${prettyValue(runRec?.dataset_id)}`);
  lines.push("");

  lines.push("Параметры эксперимента:");
  if (Object.keys(experimentParams).length > 0) {
    Object.entries(experimentParams).forEach(([key, value]) => {
      lines.push(`- ${key}: ${prettyValue(value)}`);
    });
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Ключевые показатели:");
  lines.push(`- method: ${resultMethod(result) || "—"}`);
  lines.push(`- result_status: ${runStatusLabel(resultStatus(result))}`);
  lines.push(`- time_seconds: ${canonical.timeSeconds != null ? canonical.timeSeconds.toFixed(3) : "—"}`);
  lines.push(`- memory_bytes: ${canonical.memoryBytes != null ? canonical.memoryBytes.toFixed(0) : "—"}`);
  lines.push(`- speed_per_second: ${canonical.speedPerSecond != null ? canonical.speedPerSecond.toFixed(3) : "—"}`);
  lines.push(`- cpu_usage_percent: ${canonical.cpuUsagePercent != null ? canonical.cpuUsagePercent.toFixed(2) : "—"}`);
  lines.push(`- ballots_count: ${canonical.ballotsCount != null ? canonical.ballotsCount.toFixed(0) : "—"}`);
  lines.push("");

  lines.push("Победители:");
  if (winners.length > 0) {
    winners.forEach((winner, index) => lines.push(`${index + 1}. ${winner}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Метрики:");
  if (metrics.length > 0) {
    metrics.forEach(([key, value]) => lines.push(`- ${metricLabel(key)}: ${prettyValue(value)}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Временные показатели:");
  if (timings.length > 0) {
    timings.forEach(([key, value]) => lines.push(`- ${metricLabel(key)}: ${prettyValue(value)}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  lines.push("Артефакты:");
  if (artifacts.length > 0) {
    artifacts.forEach(([key, value]) => lines.push(`- ${metricLabel(key)}: ${prettyValue(value)}`));
  } else {
    lines.push("—");
  }
  lines.push("");

  const protocol = resultRec?.protocol ?? null;
  const errorText =
    resultRec && typeof resultRec.error_text === "string" && resultRec.error_text.trim()
      ? resultRec.error_text.trim()
      : "";

  if (errorText) {
    lines.push("Ошибка:");
    lines.push(errorText);
    lines.push("");
  }

  lines.push("Протокол:");
  if (protocol != null) {
    lines.push(prettyValue(protocol));
  } else {
    lines.push("—");
  }
  lines.push("");

  return lines.join("\n");
}

export function ExperimentRunsPage() {
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [items, setItems] = useState<ExperimentRunItem[]>([]);
  const [selected, setSelected] = useState<ExperimentRunItem | null>(null);
  const [selectedResult, setSelectedResult] = useState<ExperimentRunResultResp | null>(null);
  const [resultMap, setResultMap] = useState<Record<string, ExperimentRunResultResp>>({});

  const location = useLocation();
  const locationState = (location.state ?? null) as RunsLocationState | null;
  const [experimentIdFilter, setExperimentIdFilter] = useState(
    locationState?.experimentIdFilter || ""
  );
  const [batchPayload, setBatchPayload] = useState("{\n  \n}");

  const [pollingOn, setPollingOn] = useState(true);
  const [pollEverySec, setPollEverySec] = useState(5);
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [resultLoading, setResultLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const [experimentMap, setExperimentMap] = useState<Record<string, Experiment>>({});
  const [datasetMap, setDatasetMap] = useState<Record<string, DatasetListItem>>({});
  const autoOpenRunRef = useRef<string>(locationState?.autoOpenRunId || "");

  const listAbortRef = useRef<AbortController | null>(null);
  const detailAbortRef = useRef<AbortController | null>(null);
  const resultAbortRef = useRef<AbortController | null>(null);

  const detailSectionRef = useRef<HTMLDivElement | null>(null);
  const resultSectionRef = useRef<HTMLDivElement | null>(null);

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
        const [list, experiments, datasets] = await Promise.all([
          api.experimentRuns.list(
            token,
            {
              experiment_id: experimentIdFilter.trim() || undefined,
            },
            ac.signal
          ),
          api.experiments.list(token, ac.signal),
          api.datasets.list(token, ac.signal).catch(() => []),
        ]);

        const nextExperimentMap: Record<string, Experiment> = {};
        for (const exp of experiments) {
          if (exp?.id) nextExperimentMap[exp.id] = exp;
        }
        setExperimentMap(nextExperimentMap);
        const nextDatasetMap: Record<string, DatasetListItem> = {};
        for (const dataset of datasets) {
          if (dataset?.id) nextDatasetMap[dataset.id] = dataset;
        }
        setDatasetMap(nextDatasetMap);

        const prev = prevStatusRef.current;
        const next = new Map<string, string>();

        list.forEach((run, index) => {
          const id = runId(run, index);
          const s = runStatus(run);
          next.set(id, s);

          const prevS = prev.get(id);
          if (prevS && prevS !== s) {
            const message = `${experimentRunTitle(run, index, nextExperimentMap)} · ${datasetLabel(
              run.dataset_id,
              nextDatasetMap
            )}`;

            if (s === "done") {
              addNotification({
                kind: "success",
                title: "Запуск завершен",
                message,
              });
            } else if (s === "error") {
              addNotification({
                kind: "error",
                title: "Ошибка запуска",
                message,
              });
            }
          }
        });

        prevStatusRef.current = next;

        setItems(list);

        const doneRuns = list
          .map((run, index) => ({
            id: runId(run, index),
            status: runStatus(run),
          }))
          .filter((run) => run.status === "done")
          .slice(0, 50);

        const resultEntries = await Promise.all(
          doneRuns.map(async (run) => {
            try {
              const result = await api.experimentRuns.result(token, run.id, ac.signal);
              return [run.id, result] as const;
            } catch {
              return null;
            }
          })
        );

        const nextResultMap: Record<string, ExperimentRunResultResp> = {};
        for (const entry of resultEntries) {
          if (entry) {
            nextResultMap[entry[0]] = entry[1];
          }
        }
        setResultMap(nextResultMap);
        setLastUpdatedAt(nowTimeLabel());
      } catch (e: any) {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) setToken(null);
        setErr(e?.message || "Не удалось загрузить список запусков");
        setItems([]);
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [token, experimentIdFilter, setToken, addNotification]
  );

  const loadDetail = useCallback(
    async (id: string) => {
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
    },
    [token, setToken]
  );

  const loadResult = useCallback(
    async (id: string) => {
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
    },
    [token, setToken]
  );

  const scrollToRunDetail = useCallback(() => {
    window.requestAnimationFrame(() => {
      detailSectionRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
  }, []);

  const scrollToRunResult = useCallback(() => {
    window.requestAnimationFrame(() => {
      resultSectionRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
  }, []);

  const openRunCard = useCallback(
    async (id: string) => {
      await Promise.all([loadDetail(id), loadResult(id)]);
      scrollToRunDetail();
    },
    [loadDetail, loadResult, scrollToRunDetail]
  );

  const openRunResult = useCallback(
    async (id: string) => {
      await loadResult(id);
      scrollToRunResult();
    },
    [loadResult, scrollToRunResult]
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

  useEffect(() => {
    const targetRunId = autoOpenRunRef.current;
    if (!targetRunId) return;
    if (items.length === 0) return;

    const exists = items.some((item, index) => runId(item, index) === targetRunId);
    if (!exists) return;

    autoOpenRunRef.current = "";

    if (locationState?.createdRuns?.length) {
      setInfo(`Создано запусков: ${locationState.createdRuns.length}`);
    }

    loadDetail(targetRunId);
    loadResult(targetRunId);
    scrollToRunDetail();
  }, [items, locationState, loadDetail, loadResult, scrollToRunDetail]);

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
  const selectedExperimentID =
    selected && typeof selected.experiment_id === "string" ? selected.experiment_id : "";
  const selectedExperiment = selectedExperimentID ? experimentMap[selectedExperimentID] ?? null : null;
  const selectedExperimentParams = experimentParamsObject(selectedExperiment?.params);

  const metricsSummary = useMemo(() => summaryItems(resultRecord?.metrics), [resultRecord]);
  const timingsSummary = useMemo(() => summaryItems(resultRecord?.timings), [resultRecord]);
  const artifactsSummary = useMemo(() => summaryItems(resultRecord?.artifacts), [resultRecord]);
  const winners = useMemo(() => winnerList(resultRecord?.winners), [resultRecord]);
  const canonical = useMemo(() => canonicalIndicators(selectedResult), [selectedResult]);
  const errorText = useMemo(() => resultErrorText(selectedResult), [selectedResult]);

  const methodText = useMemo(() => resultMethod(selectedResult), [selectedResult]);
  const paramsValue = useMemo(() => resultParams(selectedResult), [selectedResult]);
  const resultStatusText = useMemo(() => resultStatus(selectedResult), [selectedResult]);

  const metricChartItems = useMemo(() => numericItems(resultRecord?.metrics), [resultRecord]);
  const vectorMetricChartItems = useMemo(() => vectorChartItems(resultRecord?.metrics), [resultRecord]);
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
        label: runStatusLabel(label),
        value,
      })),
    [counters]
  );

  const durationChartItems = useMemo(
    () =>
      items
        .map((item, index) => {
          const duration = runComputeDurationSeconds(item, index, resultMap);
          if (duration == null) return null;
          return {
            label: experimentRunTitle(item, index, experimentMap),
            value: duration,
          };
        })
        .filter(Boolean) as Array<{ label: string; value: number }>,
    [items, experimentMap, resultMap]
  );

  const exportRunsCsv = () => {
    downloadCsvFile(
      "experiment-runs.csv",
      items.map((item, index) => {
        const experiment = experimentRunExperiment(item, experimentMap);

        return {
          title: experimentRunTitle(item, index, experimentMap),
          status: runStatusLabel(runStatus(item)),
          rule: experimentRuleLabel(experiment),
          ballot_format: formatBallotFormat(experimentBallotFormat(experiment)),
          dataset: datasetLabel(item.dataset_id, datasetMap),
          duration_seconds: runDurationSeconds(item) ?? "",
          started_at: prettyValue((item as any)?.started_at),
          finished_at: prettyValue((item as any)?.finished_at),
          id: runId(item, index),
          experiment_id: prettyValue((item as any)?.experiment_id),
          dataset_id: prettyValue((item as any)?.dataset_id),
        };
      })
    );
  };

  const exportRunsXlsx = () => {
    downloadXlsxFile(
      "experiment-runs.xlsx",
      items.map((item, index) => {
        const experiment = experimentRunExperiment(item, experimentMap);

        return {
          title: experimentRunTitle(item, index, experimentMap),
          status: runStatusLabel(runStatus(item)),
          rule: experimentRuleLabel(experiment),
          ballot_format: formatBallotFormat(experimentBallotFormat(experiment)),
          dataset: datasetLabel(item.dataset_id, datasetMap),
          duration_seconds: runDurationSeconds(item) ?? "",
          started_at: prettyValue((item as any)?.started_at),
          finished_at: prettyValue((item as any)?.finished_at),
          id: runId(item, index),
          experiment_id: prettyValue((item as any)?.experiment_id),
          dataset_id: prettyValue((item as any)?.dataset_id),
        };
      }),
      "Runs"
    );
  };

  const exportRunsJson = () => {
    downloadJsonFile("experiment-runs.json", items);
  };

  const exportResultSummaryCsv = () => {
    if (!selectedResult) return;
    downloadCsvFile("experiment-run-result-summary.csv", resultSummaryCsvRows(selectedResult));
  };

  const exportResultSummaryXlsx = () => {
    if (!selectedResult) return;
    downloadXlsxFile(
      "experiment-run-result-summary.xlsx",
      resultSummaryCsvRows(selectedResult),
      "ResultSummary"
    );
  };

  const exportResultReportPdf = () => {
    if (!selectedResult) return;
    downloadPdfTextFile(
      "experiment-run-report.pdf",
      "Отчет по запуску эксперимента",
      buildRunReportText(selected, selectedResult, selectedExperiment, datasetMap)
    );
  };

  const exportResultReportTxt = () => {
    if (!selectedResult) return;
    downloadTextFile(
      "experiment-run-report.txt",
      buildRunReportText(selected, selectedResult, selectedExperiment, datasetMap)
    );
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
          <h2 style={{ margin: 0 }}>Запуски экспериментов</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => loadList(false)} disabled={loading}>
              Обновить
            </button>
            <button style={styles.btn} onClick={exportRunsCsv} disabled={items.length === 0}>
              Экспорт CSV
            </button>
            <button style={styles.btn} onClick={exportRunsXlsx} disabled={items.length === 0}>
              Экспорт XLSX
            </button>
            <button style={styles.btn} onClick={exportRunsJson} disabled={items.length === 0}>
              Экспорт списка в JSON
            </button>
            {selectedResult ? (
              <>
                <button
                  style={styles.btn}
                  onClick={() => downloadJsonFile("experiment-run-result.json", selectedResult)}
                >
                  Экспорт результата в JSON
                </button>
                <button style={styles.btn} onClick={exportResultSummaryCsv}>
                  Экспорт сводки CSV
                </button>
                <button style={styles.btn} onClick={exportResultSummaryXlsx}>
                  Экспорт сводки XLSX
                </button>
                <button style={styles.btn} onClick={exportResultReportTxt}>
                  Экспорт отчета TXT
                </button>
                <button style={styles.btn} onClick={exportResultReportPdf}>
                  Экспорт отчета PDF
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
            <label>Фильтр по эксперименту</label>
            <input
              style={styles.input}
              value={experimentIdFilter}
              onChange={(e) => setExperimentIdFilter(e.target.value)}
              placeholder="Введите ID эксперимента"
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
            <Badge key={k} text={`${runStatusLabel(k)}: ${v}`} />
          ))}
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 12 }}>
          <SimpleBarChart
            title="Распределение запусков по статусам"
            items={statusChartItems}
            emptyText="Нет данных по статусам"
          />

          <SimpleBarChart
            title="Время вычисления завершенных запусков (сек)"
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
            const experiment = experimentRunExperiment(item, experimentMap);
            const ruleText = experimentRuleLabel(experiment);
            const ballotFormat = experimentBallotFormat(experiment);

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
                    <div style={{ fontWeight: 800 }}>{experimentRunTitle(item, index, experimentMap)}</div>
                    <div style={{ ...styles.muted, marginTop: 4 }}>
                      {experimentRunSubtitle(item, experimentMap, datasetMap)}
                    </div>
                  </div>

                  <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <Badge text={runStatusLabel(status)} />
                    {ruleText !== "—" ? <Badge text={ruleText} /> : null}
                    {ballotFormat ? <Badge text={formatBallotFormat(ballotFormat)} /> : null}
                  </div>
                </div>

                <div style={{ marginTop: 10 }}>
                  <SummaryGrid
                    items={[
                      { label: "Статус", value: runStatusLabel(status) },
                      { label: "Кандидаты", value: datasetCandidatesCount((item as any)?.dataset_id, datasetMap) },
                      { label: "Набор данных", value: datasetLabel((item as any)?.dataset_id, datasetMap) },
                      { label: "Формат", value: formatBallotFormat(ballotFormat) },
                      { label: "Начало", value: formatDateTime((item as any)?.started_at) },
                      { label: "Завершение", value: formatDateTime((item as any)?.finished_at) },
                      {
                        label: "Длительность",
                        value:
                          runDurationSeconds(item) != null
                            ? `${runDurationSeconds(item)?.toFixed(3)} s`
                            : "—",
                      },
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
                        { label: "ID запуска", value: id },
                        { label: "ID эксперимента", value: prettyValue((item as any)?.experiment_id) },
                        { label: "ID набора данных", value: prettyValue((item as any)?.dataset_id) },
                        { label: "Правило", value: experimentRuleId(experiment) || "—" },
                        { label: "Формат бюллетеня", value: ballotFormat || "—" },
                      ]}
                    />
                  </div>
                </details>

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btnPrimary} onClick={() => openRunCard(id)} disabled={detailLoading}>
                    Открыть карточку
                  </button>
                  <button style={styles.btn} onClick={() => openRunResult(id)} disabled={resultLoading}>
                    Загрузить результат
                  </button>
                  <button style={styles.btn} onClick={() => handleDownload(id)}>
                    Скачать результат
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Пакетный запуск</h3>
          <label>JSON-запрос</label>
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

        <div ref={detailSectionRef} style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Карточка запуска</h3>
          {detailLoading ? <div style={styles.muted}>Загрузка…</div> : null}

          {selectedRecord ? (
            <div style={{ display: "grid", gap: 12 }}>
              <div>
                <div style={{ fontWeight: 800, fontSize: 18 }}>
                  {selected ? experimentRunTitle(selected, 0, experimentMap) : "Запуск эксперимента"}
                </div>
                {selected ? (
                  <div style={{ ...styles.muted, marginTop: 4 }}>
                    {experimentRunSubtitle(selected, experimentMap, datasetMap)}
                  </div>
                ) : (
                  <div style={styles.muted}>Информация о выполнении запуска</div>
                )}
              </div>

              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                {selectedRecord.status != null ? (
                  <Badge text={runStatusLabel(selectedRecord.status)} />
                ) : null}
                {selectedExperiment ? <Badge text={experimentRuleLabel(selectedExperiment)} /> : null}
                {experimentBallotFormat(selectedExperiment) ? (
                  <Badge text={formatBallotFormat(experimentBallotFormat(selectedExperiment))} />
                ) : null}
              </div>

              <SummaryGrid
                items={[
                  { label: "Статус", value: runStatusLabel(selectedRecord.status) },
                  {
                    label: "Эксперимент",
                    value: selectedExperiment ? experimentRuleLabel(selectedExperiment) : "—",
                  },
                  {
                    label: "Формат бюллетеня",
                    value: formatBallotFormat(experimentBallotFormat(selectedExperiment)),
                  },
                  { label: "Набор данных", value: datasetLabel(selectedRecord.dataset_id, datasetMap) },
                  { label: "Начало", value: formatDateTime(selectedRecord.started_at) },
                  { label: "Завершение", value: formatDateTime(selectedRecord.finished_at) },
                  {
                    label: "Длительность",
                    value:
                      selected && runDurationSeconds(selected) != null
                        ? `${runDurationSeconds(selected)?.toFixed(3)} s`
                        : "—",
                  },
                ]}
              />

              {Object.keys(selectedExperimentParams).length > 0 ? (
                <div>
                  <h4 style={{ marginBottom: 8 }}>Параметры эксперимента</h4>
                  <SummaryGrid
                    items={Object.entries(selectedExperimentParams)
                      .filter(([, value]) => value == null || !isObject(value))
                      .slice(0, 12)
                      .map(([key, value]) => ({
                        label: metricLabel(key),
                        value: prettyValue(value),
                      }))}
                  />
                </div>
              ) : null}

              <details>
                <summary style={{ cursor: "pointer", ...styles.muted }}>
                  Технические сведения
                </summary>
                <div style={{ marginTop: 10, display: "grid", gap: 12 }}>
                  <KeyValueList
                    items={[
                      { label: "ID запуска", value: prettyValue(selectedRecord.id) },
                      { label: "ID эксперимента", value: prettyValue(selectedRecord.experiment_id) },
                      { label: "ID набора данных", value: prettyValue(selectedRecord.dataset_id) },
                      { label: "Правило", value: experimentRuleId(selectedExperiment) || "—" },
                      {
                        label: "Формат бюллетеня",
                        value: experimentBallotFormat(selectedExperiment) || "—",
                      },
                    ]}
                  />

                  <div>
                    <h4 style={{ marginBottom: 8 }}>Все поля запуска</h4>
                    <JsonBlock value={selectedRecord} />
                  </div>
                </div>
              </details>
            </div>
          ) : (
            <div style={styles.muted}>Выберите запуск из списка</div>
          )}
        </div>
      </div>

      <div ref={resultSectionRef} style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Результат запуска</h3>
        {resultLoading ? <div style={styles.muted}>Загрузка…</div> : null}

        {errorText ? (
          <div
            style={{
              padding: 12,
              borderRadius: 12,
              border: "1px solid #fecaca",
              background: "#fef2f2",
              color: "#991b1b",
            }}
          >
            <div style={{ fontWeight: 700, marginBottom: 6 }}>Ошибка выполнения</div>
            <div>{errorText}</div>
          </div>
        ) : null}

        {resultRecord ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div>
              <div style={{ fontWeight: 700 }}>Результат вычисления</div>
              <div style={styles.muted}>Данные, возвращенные сервисом результата запуска</div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`Правило: ${experimentRuleLabel(selectedExperiment)}`} />
              <Badge text={`Формат: ${formatBallotFormat(experimentBallotFormat(selectedExperiment))}`} />
              {methodText ? <Badge text={`Метод: ${methodText}`} /> : null}
              {resultStatusText ? <Badge text={`Результат: ${runStatusLabel(resultStatusText)}`} /> : null}
            </div>

            <div>
              <h4 style={{ marginBottom: 8 }}>Ключевые показатели</h4>
              <SummaryGrid
                items={[
                  { label: "Время", value: formatSeconds(canonical.timeSeconds) },
                  { label: "Память", value: formatBytes(canonical.memoryBytes) },
                  { label: "Скорость", value: formatSpeed(canonical.speedPerSecond) },
                  { label: "CPU", value: formatPercent(canonical.cpuUsagePercent) },
                  {
                    label: "Бюллетени",
                    value: canonical.ballotsCount == null ? "—" : canonical.ballotsCount.toFixed(0),
                  },
                ]}
              />
            </div>

            {paramsValue != null ? (
              <details>
                <summary style={{ cursor: "pointer", ...styles.muted }}>
                  Параметры результата
                </summary>
                <div style={{ marginTop: 10 }}>
                  <JsonBlock value={paramsValue} />
                </div>
              </details>
            ) : null}

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

            <SimpleBarChart
              title="Векторные оценки кандидатов"
              items={vectorMetricChartItems}
              emptyText="Нет векторных оценок для графика"
            />

            <div>
              <h4 style={{ marginBottom: 8 }}>Временные показатели</h4>
              {timingsSummary.length > 0 ? (
                <SummaryGrid items={timingsSummary} />
              ) : resultRecord.timings != null ? (
                <JsonBlock value={resultRecord.timings} />
              ) : (
                <div style={styles.muted}>Временные показатели отсутствуют</div>
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
                <div style={styles.muted}>Артефакты отсутствуют</div>
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