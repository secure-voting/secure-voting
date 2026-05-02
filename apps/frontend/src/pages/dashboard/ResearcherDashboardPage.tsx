import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { DatasetListItem, Experiment, ExperimentRunItem } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

function displayName(me: ReturnType<typeof useAuth>["me"]) {
  const fullName = typeof me?.full_name === "string" ? me.full_name.trim() : "";
  if (fullName) return fullName;
  return me?.email || "исследователь";
}

function formatLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    approval: "Одобрение",
    ranking: "Ранжирование",
    score: "Оценивание",
  };

  return labels[raw] || raw || "Формат не указан";
}

function sourceLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    generate: "Сгенерирован",
    import: "Импортирован",
    external: "Внешний",
  };

  return labels[raw] || raw || "Источник не указан";
}

function experimentTypeLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    algo: "Алгоритмический эксперимент",
    behavior: "Поведенческий эксперимент",
  };

  return labels[raw] || raw || "Эксперимент";
}

function experimentStatusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    draft: "Черновик",
    queued: "В очереди",
    running: "Выполняется",
    done: "Завершен",
    error: "Ошибка",
  };

  return labels[raw] || raw || "Статус неизвестен";
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

function shortId(value: unknown) {
  const raw = typeof value === "string" || typeof value === "number" ? String(value).trim() : "";
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

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
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

  return isObject(value) ? value : {};
}

function experimentRuleId(exp?: Experiment | null) {
  const params = experimentParamsObject(exp?.params);
  const rule = params.tally_rule;
  return typeof rule === "string" ? rule : "";
}

function experimentRuleLabel(exp?: Experiment | null) {
  const rule = experimentRuleId(exp);
  return rule ? tallyRuleLabel(rule) : "—";
}

function experimentFormatLabel(exp?: Experiment | null) {
  const params = experimentParamsObject(exp?.params);
  return formatLabel(params.ballot_format);
}

function experimentTitle(exp: Experiment) {
  const rule = experimentRuleLabel(exp);
  const format = experimentFormatLabel(exp);

  if (rule !== "—" && format !== "Формат не указан") {
    return `${rule} · ${format}`;
  }

  if (rule !== "—") return rule;
  if (format !== "Формат не указан") return `${experimentTypeLabel(exp.type)} · ${format}`;

  return experimentTypeLabel(exp.type);
}

function experimentSubtitle(exp: Experiment) {
  const params = experimentParamsObject(exp.params);
  const parts: string[] = [];

  if (typeof params.candidates === "number") {
    parts.push(`${params.candidates} кандидатов`);
  }

  if (typeof params.voters === "number") {
    parts.push(`${params.voters} профилей`);
  }

  if (typeof params.committee_size === "number") {
    parts.push(`комитет ${params.committee_size}`);
  }

  if (typeof exp.seed === "number") {
    parts.push(`seed ${exp.seed}`);
  }

  return parts.length > 0 ? parts.join(" · ") : "Параметры эксперимента";
}

function experimentRunStatus(item: ExperimentRunItem) {
  return typeof item.status === "string" ? item.status : "unknown";
}

function runDurationSeconds(item: ExperimentRunItem): number | null {
  const startedAt = item.started_at;
  const finishedAt = item.finished_at;

  if (typeof startedAt !== "string" || typeof finishedAt !== "string") return null;

  const startMs = Date.parse(startedAt);
  const finishMs = Date.parse(finishedAt);

  if (!Number.isFinite(startMs) || !Number.isFinite(finishMs)) return null;
  if (finishMs < startMs) return null;

  return (finishMs - startMs) / 1000;
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

function runExperiment(item: ExperimentRunItem, experimentMap: Record<string, Experiment>) {
  const experimentID = typeof item.experiment_id === "string" ? item.experiment_id : "";
  return experimentID ? experimentMap[experimentID] ?? null : null;
}

function runTitle(item: ExperimentRunItem, index: number, experimentMap: Record<string, Experiment>) {
  const exp = runExperiment(item, experimentMap);
  if (exp) return `Запуск ${experimentRuleLabel(exp)}`;
  return `Запуск эксперимента ${index + 1}`;
}

function runSubtitle(
  item: ExperimentRunItem,
  experimentMap: Record<string, Experiment>,
  datasetMap: Record<string, DatasetListItem>
) {
  const parts: string[] = [];

  const exp = runExperiment(item, experimentMap);
  if (exp) {
    const rule = experimentRuleLabel(exp);
    if (rule !== "—") parts.push(`правило ${rule}`);
  }

  parts.push(datasetLabel(item.dataset_id, datasetMap));

  const duration = runDurationSeconds(item);
  if (duration != null) {
    parts.push(`выполнен за ${duration.toFixed(3)} s`);
  }

  return parts.join(" · ");
}

export function ResearcherDashboardPage() {
  const { token, setToken, me } = useAuth();

  const [datasets, setDatasets] = useState<DatasetListItem[]>([]);
  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [runs, setRuns] = useState<ExperimentRunItem[]>([]);

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
      const [datasetList, experimentList, runList] = await Promise.all([
        api.datasets.list(token, ac.signal),
        api.experiments.list(token, ac.signal),
        api.experimentRuns.list(token, undefined, ac.signal),
      ]);

      setDatasets(datasetList);
      setExperiments(experimentList);
      setRuns(runList);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить рабочий стол исследователя");
      }
      setDatasets([]);
      setExperiments([]);
      setRuns([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  const experimentMap = useMemo(() => {
    const map: Record<string, Experiment> = {};
    for (const exp of experiments) {
      if (exp.id) map[exp.id] = exp;
    }
    return map;
  }, [experiments]);

  const datasetMap = useMemo(() => {
    const map: Record<string, DatasetListItem> = {};
    for (const dataset of datasets) {
      if (dataset.id) map[dataset.id] = dataset;
    }
    return map;
  }, [datasets]);

  const activeRuns = useMemo(
    () =>
      runs.filter((item) => {
        const status = experimentRunStatus(item);
        return status === "queued" || status === "running";
      }).length,
    [runs]
  );

  const failedRuns = useMemo(
    () => runs.filter((item) => experimentRunStatus(item) === "error").length,
    [runs]
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
            <h2 style={{ margin: 0 }}>Рабочий стол исследователя</h2>
            <div style={{ ...styles.muted, marginTop: 4 }}>
              Добро пожаловать, {displayName(me)}. Здесь собраны наборы данных, эксперименты и последние запуски.
            </div>
          </div>

          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
            <Link to="/research/datasets" style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Добавить набор данных</button>
            </Link>
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 14 }}>
          <KeyValueList
            items={[
              { label: "Наборов данных", value: String(datasets.length) },
              { label: "Экспериментов", value: String(experiments.length) },
              { label: "Запусков", value: String(runs.length) },
              { label: "Активных запусков", value: String(activeRuns) },
              { label: "Запусков с ошибкой", value: String(failedRuns) },
            ]}
          />
        </div>
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Быстрые действия</h3>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/research/datasets" style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Импортировать или сгенерировать данные</button>
            </Link>
            <Link to="/research/experiments/create" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Создать эксперимент</button>
            </Link>
            <Link to="/research/experiments" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Все эксперименты</button>
            </Link>
            <Link to="/research/runs" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Запуски</button>
            </Link>
          </div>
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Последние наборы данных</h3>
          <div style={{ marginBottom: 10 }}>
            <Link to="/research/datasets" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Все наборы данных</button>
            </Link>
          </div>

          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {datasets.length === 0 ? (
            <div style={styles.muted}>Наборы данных не найдены</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {datasets.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                    <div>
                      <div style={{ fontWeight: 800 }}>{item.name}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        Создан: {formatDateTime(item.created_at)}
                      </div>
                    </div>
                    <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                      <Badge text={sourceLabel(item.source)} />
                      <Badge text={formatLabel(item.format)} />
                    </div>
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID набора данных", value: item.id },
                          { label: "Источник", value: item.source },
                          { label: "Формат", value: item.format },
                          { label: "Создан", value: item.created_at },
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

      <div style={styles.grid2}>
        <div style={styles.card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
            <h3 style={{ marginTop: 0 }}>Последние эксперименты</h3>
            <div style={{ display: "flex", gap: 8 }}>
              <Link to="/research/experiments" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Все эксперименты</button>
              </Link>
              <Link to="/research/experiments/create" style={{ textDecoration: "none" }}>
                <button style={styles.btnPrimary}>Создать</button>
              </Link>
            </div>
          </div>

          {experiments.length === 0 ? (
            <div style={styles.muted}>Эксперименты не найдены</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {experiments.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
                    <div>
                      <div style={{ fontWeight: 800 }}>{experimentTitle(item)}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        {experimentSubtitle(item)}
                      </div>
                    </div>

                    <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                      <Badge text={experimentTypeLabel(item.type)} />
                      <Badge text={experimentStatusLabel(item.status)} />
                    </div>
                  </div>

                  <div style={{ marginTop: 8, ...styles.muted }}>
                    Создан: {formatDateTime(item.created_at)}
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID эксперимента", value: item.id },
                          { label: "Тип", value: item.type },
                          { label: "Статус", value: item.status },
                          { label: "Создан", value: item.created_at },
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
            <h3 style={{ marginTop: 0 }}>Последние запуски</h3>
            <Link to="/research/runs" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Все запуски</button>
            </Link>
          </div>

          {runs.length === 0 ? (
            <div style={styles.muted}>Запуски не найдены</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {runs.slice(0, 8).map((item, index) => (
                <div key={String(item.id ?? index)} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "flex-start" }}>
                    <div>
                      <div style={{ fontWeight: 800 }}>{runTitle(item, index, experimentMap)}</div>
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        {runSubtitle(item, experimentMap, datasetMap)}
                      </div>
                    </div>

                    <Badge text={runStatusLabel(experimentRunStatus(item))} />
                  </div>

                  <div style={{ marginTop: 8, ...styles.muted }}>
                    Начало: {formatDateTime(item.started_at)} · Завершение: {formatDateTime(item.finished_at)}
                  </div>

                  <details style={{ marginTop: 8 }}>
                    <summary style={{ cursor: "pointer", ...styles.muted }}>
                      Технические сведения
                    </summary>
                    <div style={{ marginTop: 8 }}>
                      <KeyValueList
                        items={[
                          { label: "ID запуска", value: String(item.id ?? "—") },
                          { label: "ID эксперимента", value: String(item.experiment_id ?? "—") },
                          { label: "ID набора данных", value: String(item.dataset_id ?? "—") },
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
      </div>
    </div>
  );
}