import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionSummary } from "../../shared/api/types";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { Badge } from "../../shared/ui/Badge";
import { styles } from "../../shared/ui/styles";
import { useAuth } from "../../app/auth";

export function ElectionsPage() {
  const { token, me, setToken } = useAuth();
  const [items, setItems] = useState<ElectionSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const list = await api.elections.list(token, ac.signal);
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить список выборов");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  const isAdmin = me?.role === "admin";
  const isVoter = me?.role === "voter";

  return (
    <div style={styles.card}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <h2 style={{ margin: 0 }}>Список голосований</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button style={styles.btn} onClick={reload} disabled={loading}>
            Обновить
          </button>
          {isAdmin ? (
            <Link to="/admin/elections/create" style={{ textDecoration: "none" }}>
              <button style={styles.btnPrimary}>Создать</button>
            </Link>
          ) : null}
        </div>
      </div>

      <ErrorBanner error={err} />

      {loading ? <div style={{ marginTop: 10, ...styles.muted }}>Загрузка…</div> : null}
      {!loading && items.length === 0 ? <div style={{ marginTop: 10, ...styles.muted }}>Нет голосований</div> : null}

      <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
        {items.map((e) => (
          <div key={e.id} style={{ ...styles.card, padding: 12 }}>
            <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
              <div>
                <div style={{ fontWeight: 700 }}>{e.title}</div>
                <div style={styles.muted}>{e.description || ""}</div>
              </div>
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Badge text={e.status} />
                <Badge text={e.access_mode} />
              </div>
            </div>

            <div style={{ marginTop: 8, display: "flex", gap: 10, flexWrap: "wrap", ...styles.muted, fontSize: 12 }}>
              <span>start: {e.start_at}</span>
              <span>end: {e.end_at}</span>
              {e.published_at ? <span>published: {e.published_at}</span> : null}
            </div>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Link to={`/elections/${e.id}`} style={{ textDecoration: "none" }}>
                <button style={styles.btnPrimary}>Открыть</button>
              </Link>

              {isVoter ? (
                <Link to={`/elections/${e.id}/vote`} style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Голосовать</button>
                </Link>
              ) : null}

              <Link to={`/elections/${e.id}/results`} style={{ textDecoration: "none" }}>
                <button style={styles.btn}>Результаты</button>
              </Link>

              {isAdmin ? (
                <Link to={`/admin/elections/${e.id}/rules`} style={{ textDecoration: "none" }}>
                  <button style={styles.btn}>Настройки</button>
                </Link>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}