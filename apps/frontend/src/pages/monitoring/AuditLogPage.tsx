import React, { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../../shared/api/client";
import type { AuditLogItem } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { Badge } from "../../shared/ui/Badge";
import { styles } from "../../shared/ui/styles";
import { downloadCsvFile, downloadJsonFile } from "../../shared/utils/export";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function compact(value: unknown) {
  if (value == null) return "";
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function auditCsvRows(items: AuditLogItem[]) {
  return items.map((item) => ({
    id: compact(item.id),
    occurred_at: compact(item.occurred_at),
    actor_user_id: compact((item as Record<string, unknown>).actor_user_id),
    event_type: compact(item.event_type),
    details: compact((item as Record<string, unknown>).details),
  }));
}

export function AuditLogPage() {
  const { token, setToken } = useAuth();

  const [items, setItems] = useState<AuditLogItem[]>([]);
  const [selected, setSelected] = useState<AuditLogItem | null>(null);

  const [eventType, setEventType] = useState("");
  const [actorUserId, setActorUserId] = useState("");
  const [since, setSince] = useState("");
  const [until, setUntil] = useState("");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const loadList = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const list = await api.audit.list(
        token,
        {
          event_type: eventType.trim() || undefined,
          actor_user_id: actorUserId.trim() || undefined,
          since: since.trim() || undefined,
          until: until.trim() || undefined,
        },
        ac.signal
      );
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить журнал событий");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, eventType, actorUserId, since, until, setToken]);

    const exportCsv = useCallback(() => {
      downloadCsvFile("audit-log.csv", auditCsvRows(items));
    }, [items]);

    const exportJson = useCallback(() => {
      downloadJsonFile("audit-log.json", items);
    }, [items]);

    const exportSelectedJson = useCallback(() => {
      if (!selected) return;
      const id = String(selected.id ?? "selected");
      downloadJsonFile(`audit-log-${id}.json`, selected);
    }, [selected]);

  useEffect(() => {
    loadList();
    return () => abortRef.current?.abort();
  }, [loadList]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline", flexWrap: "wrap" }}>
          <h2 style={{ margin: 0 }}>Audit log</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={loadList} disabled={loading}>
              Обновить
            </button>
            <button style={styles.btn} onClick={exportCsv} disabled={items.length === 0}>
              Export CSV
            </button>
            <button style={styles.btn} onClick={exportJson} disabled={items.length === 0}>
              Export JSON
            </button>
            <button style={styles.btn} onClick={exportSelectedJson} disabled={!selected}>
              Export selected JSON
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 12, ...styles.grid2 }}>
          <div>
            <label>event_type</label>
            <input style={styles.input} value={eventType} onChange={(e) => setEventType(e.target.value)} />
          </div>
          <div>
            <label>actor_user_id</label>
            <input style={styles.input} value={actorUserId} onChange={(e) => setActorUserId(e.target.value)} />
          </div>
          <div>
            <label>since (RFC3339)</label>
            <input style={styles.input} value={since} onChange={(e) => setSince(e.target.value)} />
          </div>
          <div>
            <label>until (RFC3339)</label>
            <input style={styles.input} value={until} onChange={(e) => setUntil(e.target.value)} />
          </div>
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && items.length === 0 ? <div style={styles.muted}>Список пуст</div> : null}

          {items.map((item, index) => {
            const id = String(item.id ?? `audit-${index}`);
            const event = String(item.event_type ?? "unknown");
            const occurredAt = String(item.occurred_at ?? "—");

            return (
              <button
                key={id}
                style={{
                  ...styles.btn,
                  ...styles.card,
                  textAlign: "left",
                  padding: 12,
                }}
                onClick={() => setSelected(item)}
              >
                <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
                  <div>
                    <div style={{ fontWeight: 700 }}>{id}</div>
                    <div style={styles.muted}>occurred_at: {occurredAt}</div>
                  </div>
                  <Badge text={event} />
                </div>
              </button>
            );
          })}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Событие</h3>
        {selected ? <JsonBlock value={selected} /> : <div style={styles.muted}>Ничего не выбрано</div>}
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