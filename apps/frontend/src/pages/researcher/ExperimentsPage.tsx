import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { Experiment } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function formatParams(params: unknown) {
  if (!params || typeof params !== "object") {
    return <span style={styles.muted}>Параметры не указаны</span>;
  }

  const entries = Object.entries(params as Record<string, unknown>);
  if (entries.length === 0) {
    return <span style={styles.muted}>Параметры не указаны</span>;
  }

  return (
    <div style={{ display: "grid", gap: 6 }}>
      {entries.map(([key, value]) => (
        <div key={key}>
          <b>{key}:</b> <span>{typeof value === "string" ? value : JSON.stringify(value)}</span>
        </div>
      ))}
    </div>
  );
}

export function ExperimentsPage() {
  const { token, setToken, me } = useAuth();

  const [items, setItems] = useState<Experiment[]>([]);
  const [selected, setSelected] = useState<Experiment | null>(null);

  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);
  const detailAbortRef = useRef<AbortController | null>(null);

  const loadList = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const list = await api.experiments.list(token, ac.signal);
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить список экспериментов");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    loadList();
    return () => {
      abortRef.current?.abort();
      detailAbortRef.current?.abort();
    };
  }, [loadList]);

  const loadDetail = async (id: string) => {
    if (!token) return;

    detailAbortRef.current?.abort();
    const ac = new AbortController();
    detailAbortRef.current = ac;

    setDetailLoading(true);
    setErr(null);

    try {
      const item = await api.experiments.get(token, id, ac.signal);
      setSelected(item);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить эксперимент");
      setSelected(null);
    } finally {
      setDetailLoading(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Эксперименты</h2>
          <div style={{ display: "flex", gap: 8 }}>
            <button style={styles.btn} onClick={loadList} disabled={loading}>
              Обновить
            </button>
            {me?.role === "researcher" ? (
              <Link to="/research/experiments/create" style={{ textDecoration: "none" }}>
                <button style={styles.btnPrimary}>Создать</button>
              </Link>
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && items.length === 0 ? <div style={styles.muted}>Список пуст</div> : null}

          {items.map((item) => (
            <div key={item.id} style={{ ...styles.card, padding: 12 }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
                <div>
                  <div style={{ fontWeight: 700 }}>{item.id}</div>
                  <div style={styles.muted}>created_by: {item.created_by}</div>
                </div>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <Badge text={item.type} />
                  <Badge text={item.status} />
                </div>
              </div>

              <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                created_at: {item.created_at}
                {item.seed != null ? ` · seed: ${item.seed}` : ""}
              </div>

              <div style={{ marginTop: 10 }}>
                <button style={styles.btnPrimary} onClick={() => loadDetail(item.id)} disabled={detailLoading}>
                  Открыть
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Карточка эксперимента</h3>
        {detailLoading ? <div style={styles.muted}>Загрузка…</div> : null}

        {selected ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div>
              <div style={{ fontWeight: 700, fontSize: 18 }}>{selected.id}</div>
              <div style={styles.muted}>Эксперимент типа {selected.type}</div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={selected.type} />
              <Badge text={selected.status} />
              {selected.seed != null ? <Badge text={`seed: ${selected.seed}`} /> : null}
            </div>

            <KeyValueList
              items={[
                { label: "Created by", value: selected.created_by },
                { label: "Created at", value: selected.created_at },
              ]}
            />

            <div>
              <h4 style={{ marginBottom: 8 }}>Параметры</h4>
              {formatParams(selected.params)}
            </div>
          </div>
        ) : (
          <div style={styles.muted}>Ничего не выбрано</div>
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