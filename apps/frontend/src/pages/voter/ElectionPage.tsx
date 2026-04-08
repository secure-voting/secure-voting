import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionDetail } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { styles } from "../../shared/ui/styles";
import { downloadJsonFile } from "../../shared/utils/export";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

export function ElectionPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken, me } = useAuth();

  const [item, setItem] = useState<ElectionDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const isVoter = me?.role === "voter";

  const load = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const detail = await api.elections.get(token, electionId, ac.signal);
      setItem(detail);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось загрузить карточку голосования");
      }
      setItem(null);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

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
          <h2 style={{ margin: 0 }}>Карточка голосования</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Назад</button>
            </Link>
            {isVoter ? (
              <Link to={`/elections/${electionId}/vote`} style={{ textDecoration: "none" }}>
                <button style={styles.btnPrimary}>Голосовать</button>
              </Link>
            ) : null}
            <Link to={`/elections/${electionId}/results`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Результаты</button>
            </Link>
            <button style={styles.btn} onClick={load} disabled={loading}>
              Обновить
            </button>
            {item ? (
              <button
                style={styles.btn}
                onClick={() => downloadJsonFile(`election-${electionId}.json`, item)}
              >
                Export JSON
              </button>
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {item ? (
          <>
            <div style={{ marginTop: 10 }}>
              <div style={{ fontWeight: 800, fontSize: 18 }}>{item.title}</div>
              <div style={styles.muted}>{item.description || "Описание отсутствует"}</div>
            </div>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`status: ${item.status}`} />
              <Badge text={`access: ${item.access_mode}`} />
              <Badge text={`format: ${item.ballot_format}`} />
              <Badge text={`rule: ${item.tally_rule}`} />
            </div>

            <div style={{ marginTop: 12 }}>
              <SummaryGrid
                items={[
                  { label: "Organizer", value: item.organizer_email ?? item.created_by ?? "—" },
                  { label: "Created at", value: item.created_at ?? "—" },
                  { label: "Start at", value: item.start_at },
                  { label: "End at", value: item.end_at },
                  { label: "Publish at", value: item.publish_at ?? "—" },
                  { label: "Published at", value: item.published_at ?? "—" },
                  { label: "Committee size", value: String(item.committee_size ?? "—") },
                  { label: "Quota type", value: item.quota_type ?? "—" },
                  { label: "Show aggregates", value: item.show_aggregates ? "yes" : "no" },
                  { label: "Candidates", value: String(item.candidates.length) },
                  {
                    label: "Approval max choices",
                    value: item.ballot_format === "approval"
                      ? String(item.approval_max_choices ?? "—")
                      : "—",
                  },
                  {
                    label: "Ranking top-k",
                    value: item.ballot_format === "ranking"
                      ? String(item.ranking_top_k ?? "—")
                      : "—",
                  },
                  {
                    label: "Score range",
                    value: item.ballot_format === "score"
                      ? `${item.score_min ?? "—"}..${item.score_max ?? "—"}`
                      : "—",
                  },
                  {
                    label: "Score step",
                    value: item.ballot_format === "score"
                      ? String(item.score_step ?? "—")
                      : "—",
                  },
                  {
                    label: "Allow skip",
                    value: item.ballot_format === "score"
                      ? (item.score_allow_skip ? "yes" : "no")
                      : "—",
                  },
                ]}
              />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Кандидаты</h3>
            <div style={{ display: "grid", gap: 8 }}>
              {item.candidates.map((candidate) => {
                const description =
                  candidate.meta &&
                  typeof candidate.meta === "object" &&
                  typeof (candidate.meta as any).description === "string"
                    ? String((candidate.meta as any).description)
                    : "";

                return (
                  <div
                    key={candidate.id}
                    style={{
                      ...styles.card,
                      padding: 10,
                      display: "flex",
                      justifyContent: "space-between",
                      gap: 10,
                      alignItems: "baseline",
                    }}
                  >
                    <div>
                      <div style={{ fontWeight: 700 }}>{candidate.name}</div>
                      {description ? <div style={{ ...styles.muted, marginTop: 4 }}>{description}</div> : null}
                      <div style={{ ...styles.muted, marginTop: 4 }}>{candidate.id}</div>
                    </div>
                    {candidate.meta ? <Badge text="meta" /> : null}
                  </div>
                );
              })}
            </div>
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug JSON</h3>
          {item ? <JsonBlock value={item} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}