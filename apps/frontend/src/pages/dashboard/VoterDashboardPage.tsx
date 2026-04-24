import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionSummary } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

function isActiveLike(status: string) {
  return status === "active" || status === "scheduled" || status === "paused";
}

export function VoterDashboardPage() {
  const { token, setToken, me } = useAuth();

  const [items, setItems] = useState<ElectionSummary[]>([]);
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
      const elections = await api.elections.list(token, ac.signal);
      setItems(elections);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить рабочий стол голосующего");
      }
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  const activeItems = useMemo(() => items.filter((item) => isActiveLike(item.status)), [items]);
  const publishedItems = useMemo(() => items.filter((item) => Boolean(item.published_at)), [items]);
  const recentItems = useMemo(() => items.slice(0, 5), [items]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>Рабочий стол голосующего</h2>
        <div style={styles.muted}>
          Добро пожаловать{me?.email ? `, ${me.email}` : ""}. Здесь собраны доступные голосования и опубликованные результаты.
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Badge text={`all: ${items.length}`} />
          <Badge text={`active_like: ${activeItems.length}`} />
          <Badge text={`published: ${publishedItems.length}`} />
          <button style={styles.btn} onClick={load} disabled={loading}>
            Обновить
          </button>
        </div>

        <ErrorBanner error={err} />
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Доступные голосования</h3>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && activeItems.length === 0 ? (
            <div style={styles.muted}>Сейчас нет доступных голосований</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {activeItems.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ fontWeight: 700 }}>{item.title}</div>
                  <div style={styles.muted}>{item.description || "Описание отсутствует"}</div>

                  <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                    <Badge text={item.status} />
                    <Badge text={item.access_mode} />
                  </div>

                  <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                    start: {item.start_at}
                    <br />
                    end: {item.end_at}
                  </div>

                  <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <Link to={`/elections/${item.id}`} style={{ textDecoration: "none" }}>
                      <button style={styles.btn}>Карточка</button>
                    </Link>
                    <Link to={`/elections/${item.id}/vote`} style={{ textDecoration: "none" }}>
                      <button style={styles.btnPrimary}>Голосовать</button>
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Опубликованные результаты</h3>
          {publishedItems.length === 0 ? (
            <div style={styles.muted}>Пока нет опубликованных результатов</div>
          ) : (
            <div style={{ display: "grid", gap: 8 }}>
              {publishedItems.slice(0, 6).map((item) => (
                <div key={item.id} style={{ ...styles.card, padding: 10 }}>
                  <div style={{ fontWeight: 700 }}>{item.title}</div>
                  <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                    published_at: {item.published_at}
                  </div>

                  <div style={{ marginTop: 10 }}>
                    <Link to={`/elections/${item.id}/results`} style={{ textDecoration: "none" }}>
                      <button style={styles.btnPrimary}>Открыть результаты</button>
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Последние выборы</h3>
        {recentItems.length === 0 ? (
          <div style={styles.muted}>Список пока пуст</div>
        ) : (
          <div style={{ display: "grid", gap: 8 }}>
            {recentItems.map((item) => (
              <div
                key={item.id}
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr auto",
                  gap: 10,
                  alignItems: "center",
                  padding: "8px 0",
                  borderBottom: "1px solid #f3f4f6",
                }}
              >
                <div>
                  <div style={{ fontWeight: 700 }}>{item.title}</div>
                  <div style={styles.muted}>{item.status}</div>
                </div>
                <Link to={`/elections/${item.id}`} style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Открыть</button>
                </Link>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}