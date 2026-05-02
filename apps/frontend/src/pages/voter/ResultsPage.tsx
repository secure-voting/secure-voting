import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionDetail, ResultResp } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { ProtocolTimeline } from "../../shared/ui/ProtocolTimeline";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { SimpleBarChart } from "../../shared/ui/SimpleBarChart";
import { styles } from "../../shared/ui/styles";
import { ActionMenu } from "../../shared/ui/ActionMenu";
import {
  downloadJsonFile,
  downloadPdfTextFile,
  downloadXlsxFile,
} from "../../shared/utils/export";

function compactValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function isEmptyObject(value: unknown): boolean {
  return isObject(value) && Object.keys(value).length === 0;
}

function extractWinnerIdList(winners: unknown): string[] {
  if (Array.isArray(winners)) {
    return winners.map((item) => compactValue(item));
  }

  if (winners && typeof winners === "object") {
    const obj = winners as Record<string, unknown>;

    if (Array.isArray(obj.winners)) {
      return obj.winners.map((item) => compactValue(item));
    }

    if (Array.isArray(obj.items)) {
      return obj.items.map((item) => compactValue(item));
    }
  }

  if (winners != null) {
    return [compactValue(winners)];
  }

  return [];
}

function metricLabel(key: string): string {
  switch (key) {
    case "total_ballots":
      return "Всего бюллетеней";
    case "valid_ballots":
      return "Корректных бюллетеней";
    case "invalid_ballots":
      return "Некорректных бюллетеней";
    case "candidates_count":
      return "Число кандидатов";
    case "winner_count":
      return "Число победителей";
    case "committee_size":
      return "Размер комитета";
    case "rounds_count":
      return "Число раундов";
    case "winner_score":
      return "Баллы победителя";
    case "runner_up_score":
      return "Баллы второго места";
    case "margin":
      return "Отрыв";
    case "average_score":
      return "Средний балл";
    default:
      return key;
  }
}

function paramLabel(key: string): string {
  switch (key) {
    case "tally_rule":
      return "Правило подсчета";
    case "ballot_format":
      return "Формат бюллетеня";
    case "committee_size":
      return "Размер комитета";
    case "quota_type":
      return "Тип квоты";
    case "approval_max_choices":
      return "Лимит выбора";
    case "ranking_top_k":
      return "Ограничение top-k";
    case "score_min":
      return "Минимальная оценка";
    case "score_max":
      return "Максимальная оценка";
    case "score_step":
      return "Шаг оценки";
    case "score_allow_skip":
      return "Разрешить пропуск";
    case "show_aggregates":
      return "Показывать агрегаты";
    default:
      return key;
  }
}

function formatParamValue(key: string, value: unknown): string {
  if (key === "show_aggregates" || key === "score_allow_skip") {
    return value ? "Да" : "Нет";
  }
  return compactValue(value);
}

function summaryItemsFromMetrics(value: unknown): Array<{ label: string; value: React.ReactNode }> {
  if (!isObject(value)) return [];

  const summary = isObject(value.summary) ? value.summary : value;

  return Object.entries(summary)
    .filter(([, val]) => val != null && !(isObject(val) && Object.keys(val).length === 0))
    .slice(0, 12)
    .map(([key, val]) => ({
      label: metricLabel(key),
      value: compactValue(val),
    }));
}

function paramsItems(value: unknown): Array<{ label: string; value: React.ReactNode }> {
  if (!isObject(value)) return [];

  return Object.entries(value)
    .filter(([, val]) => val != null)
    .map(([key, val]) => ({
      label: paramLabel(key),
      value: formatParamValue(key, val),
    }));
}

function numericItemsFromMetrics(value: unknown): Array<{ label: string; value: number }> {
  if (!isObject(value)) return [];

  const series = isObject(value.series) ? value.series : null;
  const numeric = isObject(value.numeric) ? value.numeric : null;

  if (series && Array.isArray(series.candidate_scores_final)) {
    return series.candidate_scores_final
      .filter((item) => isObject(item) && typeof item.value === "number" && Number.isFinite(item.value))
      .map((item) => ({
        label:
          typeof item.candidate_name === "string" && item.candidate_name.trim()
            ? item.candidate_name.trim()
            : typeof item.candidate_id === "string" && item.candidate_id.trim()
              ? item.candidate_id.trim()
              : "Кандидат",
        value: Number(item.value),
      }));
  }

  if (numeric) {
    return Object.entries(numeric)
      .filter(([, val]) => typeof val === "number" && Number.isFinite(val))
      .slice(0, 12)
      .map(([key, val]) => ({
        label: metricLabel(key),
        value: Number(val),
      }));
  }

  return [];
}

function vectorChartItemsFromMetrics(value: unknown): Array<{ label: string; value: number }> {
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

function hasDisplayableProtocol(protocol: unknown): boolean {
  if (protocol == null) return false;
  if (Array.isArray(protocol)) return protocol.length > 0;
  if (isObject(protocol)) {
    if (Array.isArray(protocol.steps)) return protocol.steps.length > 0;
    return Object.keys(protocol).length > 0;
  }
  return true;
}

function resultRows(result: ResultResp, winnerLabels: string[]) {
  const rows: Array<Record<string, unknown>> = [];

  rows.push(
    { section: "summary", key: "election_id", value: result.election_id },
    { section: "summary", key: "version", value: result.version },
    { section: "summary", key: "method", value: result.method },
    { section: "summary", key: "published_at", value: result.published_at ?? "" }
  );

  winnerLabels.forEach((winner, index) => {
    rows.push({
      section: "winners",
      key: index + 1,
      value: winner,
    });
  });

  return rows;
}

function buildResultReportText(result: ResultResp, winnerLabels: string[]) {
  const lines: string[] = [];

  lines.push("Отчет по результатам голосования");
  lines.push("");
  lines.push(`ID голосования: ${compactValue(result.election_id)}`);
  lines.push(`Версия результата: ${compactValue(result.version)}`);
  lines.push(`Метод: ${compactValue(result.method)}`);
  lines.push(`Опубликовано: ${compactValue(result.published_at ?? "—")}`);
  lines.push("");
  lines.push("Победители:");
  if (winnerLabels.length > 0) {
    winnerLabels.forEach((winner, index) => {
      lines.push(`${index + 1}. ${winner}`);
    });
  } else {
    lines.push("—");
  }

  return `${lines.join("\n")}`;
}

export function ResultsPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken } = useAuth();

  const [res, setRes] = useState<ResultResp | null>(null);
  const [detail, setDetail] = useState<ElectionDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);
    setInfo(null);

    try {
      const [resultData, electionDetail] = await Promise.all([
        api.results.get(token, electionId, ac.signal),
        api.elections.get(token, electionId, ac.signal).catch(() => null),
      ]);
      setRes(resultData);
      setDetail(electionDetail);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else if (e?.status === 403) {
        setInfo("Результаты еще не опубликованы");
      } else if (e?.status === 404) {
        setInfo("Результаты пока недоступны");
      } else {
        setErr(e?.message || "Не удалось загрузить результаты");
      }
      setRes(null);
      setDetail(null);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  const candidateNameById = useMemo(() => {
    const map = new Map<string, string>();
    if (!detail?.candidates) return map;
    for (const candidate of detail.candidates) {
      if (candidate?.id && candidate?.name) {
        map.set(String(candidate.id), String(candidate.name));
      }
    }
    return map;
  }, [detail]);

  const winnerIds = useMemo(() => extractWinnerIdList(res?.winners), [res?.winners]);

  const winnerLabels = useMemo(
    () =>
      winnerIds.map((winnerId) => {
        const name = candidateNameById.get(winnerId);
        return name ? name : winnerId;
      }),
    [winnerIds, candidateNameById]
  );

  const metricsSummary = useMemo(() => summaryItemsFromMetrics(res?.metrics), [res?.metrics]);
  const paramsSummary = useMemo(() => paramsItems(res?.params), [res?.params]);
  const metricChartItems = useMemo(() => numericItemsFromMetrics(res?.metrics), [res?.metrics]);
  const vectorMetricChartItems = useMemo(() => vectorChartItemsFromMetrics(res?.metrics), [res?.metrics]);
  const showProtocolBlock = hasDisplayableProtocol(res?.protocol);

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
          <h2 style={{ margin: 0 }}>Результаты голосования</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to={`/elections/${electionId}`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Карточка</button>
            </Link>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Назад</button>
            </Link>
            <button style={styles.btn} onClick={reload} disabled={loading}>
              Обновить
            </button>
            {res ? (
              <ActionMenu
                label="Экспорт"
                items={[
                  {
                    label: "JSON",
                    onClick: () => downloadJsonFile(`election-result-${electionId}.json`, res),
                  },
                  {
                    label: "XLSX",
                    onClick: () =>
                      downloadXlsxFile(
                        `election-result-${electionId}.xlsx`,
                        resultRows(res, winnerLabels),
                        "Результаты"
                      ),
                  },
                  {
                    label: "PDF",
                    onClick: () =>
                      downloadPdfTextFile(
                        `election-result-${electionId}.pdf`,
                        "Отчет по результатам голосования",
                        buildResultReportText(res, winnerLabels)
                      ),
                  },
                ]}
              />
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />

        {info ? (
          <div style={{ ...styles.card, background: "#f9fafb", borderColor: "#e5e7eb", marginBottom: 12 }}>
            {info}
          </div>
        ) : null}

        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {res ? (
          <>
            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`Метод: ${res.method}`} />
              <Badge text={`Версия: ${String(res.version)}`} />
              <Badge text={`Опубликовано: ${res.published_at ?? "—"}`} />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Победители</h3>
            {winnerLabels.length > 0 ? (
              <div style={{ ...styles.card, background: "#f9fafb" }}>
                <ol style={{ margin: "0 0 0 18px" }}>
                  {winnerLabels.map((winner, index) => (
                    <li key={`${winner}-${index}`} style={{ marginBottom: 6 }}>
                      {winner}
                    </li>
                  ))}
                </ol>
              </div>
            ) : (
              <div style={styles.muted}>Данные о победителях отсутствуют</div>
            )}

            <hr style={styles.hr} />

            <div style={{ display: "grid", gap: 12 }}>
              <div>
                <h3 style={{ marginTop: 0 }}>Сводные показатели</h3>
                {metricsSummary.length > 0 ? (
                  <SummaryGrid items={metricsSummary} />
                ) : (
                  <div style={styles.muted}>Сводные показатели не предоставлены</div>
                )}
              </div>

              <SimpleBarChart
                title="Скалярные числовые метрики"
                items={metricChartItems}
                emptyText="Скалярные числовые метрики для графика отсутствуют"
              />

              <SimpleBarChart
                title="Векторные оценки кандидатов"
                items={vectorMetricChartItems}
                emptyText="Векторные оценки для графика отсутствуют"
              />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Протокол шагов</h3>
            {showProtocolBlock ? (
              <ProtocolTimeline protocol={res.protocol} />
            ) : (
              <div style={styles.muted}>Подробный протокол для данного метода отсутствует</div>
            )}

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Параметры расчета</h3>
            {paramsSummary.length > 0 ? (
              <SummaryGrid items={paramsSummary} />
            ) : (
              <div style={styles.muted}>Параметры расчета не указаны</div>
            )}
          </>
        ) : null}
      </div>
    </div>
  );
}