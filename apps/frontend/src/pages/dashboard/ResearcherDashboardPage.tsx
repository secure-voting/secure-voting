import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { DatasetListItem, Experiment, ExperimentRunItem } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

function experimentRunStatus(item: ExperimentRunItem) {
  return typeof item.status === "string" ? item.status : "unknown";
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

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>Рабочий стол исследователя</h2>
        <div style={styles.muted}>
          Добро пожаловать{me?.email ? `, ${me.email}` : ""}. Здесь собраны наборы данных, эксперименты и запуски.
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Badge text={`datasets: ${datasets.length}`} />
          <Badge text={`experiments: ${experiments.length}`} />
          <Badge text={`runs: ${runs.length}`} />
          <button style={styles.btn} onClick={load} disabled={loading}>
            Обновить
          </button>
        </div>

        <ErrorBanner error={err} />
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
            <h3 style={{ marginTop: 0 }}>Последние наборы данных</h3>
            <Link to="/research/datasets" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Все datasets</button>
            </Link>
          </div>

          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {datasets.length === 0 ? (
            <div style={styles.muted}>Наборы данных не найдены</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {datasets.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ fontWeight: 700 }}>{item.name}</div>
                  <div style={styles.muted}>{item.id}</div>

                  <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <Badge text={item.source} />
                    <Badge text={item.format} />
                  </div>

                  <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                    created_at: {item.created_at}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <div style={styles.card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
            <h3 style={{ marginTop: 0 }}>Последние эксперименты</h3>
            <div style={{ display: "flex", gap: 8 }}>
              <Link to="/research/experiments" style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Все experiments</button>
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
                  <div style={{ fontWeight: 700 }}>{item.id}</div>
                  <div style={styles.muted}>type: {item.type}</div>

                  <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <Badge text={item.type} />
                    <Badge text={item.status} />
                    {item.seed != null ? <Badge text={`seed: ${item.seed}`} /> : null}
                  </div>

                  <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                    created_at: {item.created_at}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h3 style={{ marginTop: 0 }}>Последние запуски</h3>
          <Link to="/research/runs" style={{ textDecoration: "none" }}>
            <button style={styles.btn}>Все runs</button>
          </Link>
        </div>

        {runs.length === 0 ? (
          <div style={styles.muted}>Запуски не найдены</div>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {runs.slice(0, 8).map((item, index) => (
              <div key={String(item.id ?? index)} style={{ ...styles.card, padding: 10 }}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
                  <div>
                    <div style={{ fontWeight: 700 }}>{String(item.id ?? `run-${index}`)}</div>
                    <div style={styles.muted}>experiment_id: {String(item.experiment_id ?? "—")}</div>
                    <div style={styles.muted}>dataset_id: {String(item.dataset_id ?? "—")}</div>
                  </div>
                  <Badge text={experimentRunStatus(item)} />
                </div>

                <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                  started_at: {String(item.started_at ?? "—")} · finished_at: {String(item.finished_at ?? "—")}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}