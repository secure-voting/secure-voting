import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ResultResp } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { ProtocolTimeline } from "../../shared/ui/ProtocolTimeline";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { SimpleBarChart } from "../../shared/ui/SimpleBarChart";
import { styles } from "../../shared/ui/styles";
import { downloadJsonFile } from "../../shared/utils/export";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

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

function extractWinnerList(winners: unknown): string[] {
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

function summaryItemsFromObject(value: unknown): Array<{ label: string; value: React.ReactNode }> {
  if (!isObject(value)) return [];

  return Object.entries(value)
    .slice(0, 12)
    .map(([key, val]) => ({
      label: key,
      value: compactValue(val),
    }));
}

function numericItemsFromObject(value: unknown): Array<{ label: string; value: number }> {
  if (!isObject(value)) return [];

  return Object.entries(value)
    .filter(([, val]) => typeof val === "number" && Number.isFinite(val))
    .slice(0, 12)
    .map(([key, val]) => ({
      label: key,
      value: Number(val),
    }));
}

export function ResultsPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken } = useAuth();

  const [res, setRes] = useState<ResultResp | null>(null);
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
      const r = await api.results.get(token, electionId, ac.signal);
      setRes(r);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else if (e?.status === 403) {
        setInfo("Результаты ещё не опубликованы");
      } else if (e?.status === 404) {
        setInfo("Результаты пока недоступны");
      } else {
        setErr(e?.message || "Не удалось загрузить результаты");
      }
      setRes(null);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  const winners = useMemo(() => extractWinnerList(res?.winners), [res?.winners]);
  const metricsSummary = useMemo(() => summaryItemsFromObject(res?.metrics), [res?.metrics]);
  const paramsSummary = useMemo(() => summaryItemsFromObject(res?.params), [res?.params]);
  const metricChartItems = useMemo(() => numericItemsFromObject(res?.metrics), [res?.metrics]);

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
              <button
                style={styles.btn}
                onClick={() => downloadJsonFile(`election-result-${electionId}.json`, res)}
              >
                Export JSON
              </button>
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
              <Badge text={`method: ${res.method}`} />
              <Badge text={`version: ${String(res.version)}`} />
              <Badge text={`published_at: ${res.published_at ?? "null"}`} />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Победители</h3>
            {winners.length > 0 ? (
              <div style={{ ...styles.card, background: "#f9fafb" }}>
                <ol style={{ margin: "0 0 0 18px" }}>
                  {winners.map((winner, index) => (
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
                ) : res.metrics != null ? (
                  <JsonBlock value={res.metrics} />
                ) : (
                  <div style={styles.muted}>Сводные показатели не предоставлены</div>
                )}
              </div>

              <SimpleBarChart
                title="Числовые метрики"
                items={metricChartItems}
                emptyText="Числовые метрики для графика отсутствуют"
              />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Протокол шагов</h3>
            {res.protocol != null ? (
              <ProtocolTimeline protocol={res.protocol} />
            ) : (
              <div style={styles.muted}>Протокол шагов отсутствует</div>
            )}

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Параметры расчёта</h3>
            {paramsSummary.length > 0 ? (
              <SummaryGrid items={paramsSummary} />
            ) : res.params != null ? (
              <JsonBlock value={res.params} />
            ) : (
              <div style={styles.muted}>Параметры расчёта не указаны</div>
            )}
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Results JSON</h3>
          {res ? <JsonBlock value={res} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}