import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionDetail, UpdateElectionRulesInput } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { styles } from "../../shared/ui/styles";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

const TALLY_RULES = [
  "plurality",
  "approval",
  "inverse_plurality",
  "borda",
  "black",
  "copeland_i",
  "copeland_ii",
  "copeland_iii",
  "simpson",
  "minmax",
  "hare",
  "inverse_borda",
  "nanson",
  "coombs",
  "practical_condorcet",
  "threshold",
];

const TALLY_RULE_ALIASES: Record<string, string> = {
  minimax: "minmax",
  condorcet_practical: "practical_condorcet",
};

function normalizeTallyRule(value: string) {
  const trimmed = value.trim();
  return TALLY_RULE_ALIASES[trimmed] ?? trimmed;
}

export function ElectionRulesPage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [item, setItem] = useState<ElectionDetail | null>(null);

  const [tallyRule, setTallyRule] = useState("plurality");
  const [ballotFormat, setBallotFormat] = useState<"approval" | "ranking" | "score">("ranking");

  const [committeeSize, setCommitteeSize] = useState<number>(1);
  const [quotaType, setQuotaType] = useState<"hare" | "droop">("hare");

  const [accessMode, setAccessMode] = useState<"open" | "invite">("open");
  const [publishAt, setPublishAt] = useState("");
  const [showAggregates, setShowAggregates] = useState(true);

  const [approvalMaxChoices, setApprovalMaxChoices] = useState<number>(1);
  const [rankingTopK, setRankingTopK] = useState<number>(1);

  const [scoreMin, setScoreMin] = useState<number>(0);
  const [scoreMax, setScoreMax] = useState<number>(10);
  const [scoreStep, setScoreStep] = useState<number>(1);
  const [scoreAllowSkip, setScoreAllowSkip] = useState(false);

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const hydrate = (e: ElectionDetail) => {
    setTallyRule(normalizeTallyRule(e.tally_rule));
    setBallotFormat((e.ballot_format as "approval" | "ranking" | "score") || "ranking");
    setCommitteeSize(e.committee_size ?? 1);
    setQuotaType((e.quota_type as "hare" | "droop") ?? "hare");
    setAccessMode((e.access_mode as "open" | "invite") ?? "open");
    setPublishAt(e.publish_at ?? "");
    setShowAggregates(Boolean(e.show_aggregates));
    setApprovalMaxChoices(e.approval_max_choices ?? 1);
    setRankingTopK(e.ranking_top_k ?? 1);
    setScoreMin(e.score_min ?? 0);
    setScoreMax(e.score_max ?? 10);
    setScoreStep(e.score_step ?? 1);
    setScoreAllowSkip(Boolean(e.score_allow_skip));
  };

  const load = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);
    setInfo(null);

    try {
      const e = await api.elections.get(token, electionId, ac.signal);
      setItem(e);
      hydrate(e);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить настройки голосования");
      setItem(null);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);

  useEffect(() => {
    if (ballotFormat === "approval" && tallyRule !== "approval") {
      setTallyRule("approval");
    }
  }, [ballotFormat, tallyRule]);

  const validate = (): string | null => {
    if (!item) return "Нет данных голосования";

    const candidatesCount = item.candidates.length;

    const normalizedRule = normalizeTallyRule(tallyRule);
    if (!normalizedRule) return "Выберите правило подсчёта";
    if (!TALLY_RULES.includes(normalizedRule)) return "Недопустимое правило подсчёта";
    if (committeeSize < 1) return "committee_size должен быть не меньше 1";

    if (ballotFormat === "approval") {
      if (approvalMaxChoices < 1) return "approval_max_choices должен быть не меньше 1";
      if (approvalMaxChoices > candidatesCount) return "approval_max_choices не может превышать число кандидатов";
    }

    if (ballotFormat === "ranking") {
      if (rankingTopK < 1) return "ranking_top_k должен быть не меньше 1";
      if (rankingTopK > candidatesCount) return "ranking_top_k не может превышать число кандидатов";
    }

    if (ballotFormat === "score") {
      if (scoreStep <= 0) return "score_step должен быть больше 0";
      if (scoreMin > scoreMax) return "score_min не может быть больше score_max";
      if ((scoreMax - scoreMin) % scoreStep !== 0) {
        return "Диапазон score должен делиться на score_step без остатка";
      }
    }

    if (publishAt.trim()) {
      const parsed = Date.parse(publishAt);
      if (Number.isNaN(parsed)) return "publish_at должен быть в формате RFC3339";
    }

    return null;
  };

  const submit = async () => {
    if (!token) return;

    const validationError = validate();
    if (validationError) {
      setErr(validationError);
      return;
    }

    setSaving(true);
    setErr(null);
    setInfo(null);

    try {
      const body: UpdateElectionRulesInput = {
        tally_rule: normalizeTallyRule(tallyRule),
        ballot_format: ballotFormat,
        committee_size: committeeSize,
        quota_type: committeeSize > 1 ? quotaType : null,
        access_mode: accessMode,
        publish_at: publishAt.trim() ? publishAt.trim() : null,
        show_aggregates: showAggregates,
      };

      if (ballotFormat === "approval") {
        body.approval_max_choices = approvalMaxChoices;
      }

      if (ballotFormat === "ranking") {
        body.ranking_top_k = rankingTopK;
      }

      if (ballotFormat === "score") {
        body.score_min = scoreMin;
        body.score_max = scoreMax;
        body.score_step = scoreStep;
        body.score_allow_skip = scoreAllowSkip;
      }

      await api.elections.updateRules(token, electionId, body);
      setInfo("Настройки успешно сохранены");

      addNotification({
        kind: "success",
        title: "Настройки голосования обновлены",
        message: `Изменены параметры голосования ${electionId}`,
      });

      await load();
    } catch (e: any) {
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось сохранить настройки");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Настройка правил голосования</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to={`/elections/${electionId}`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К карточке</button>
            </Link>
            <button style={styles.btn} onClick={load} disabled={loading || saving}>
              Обновить
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />

        {info ? (
          <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0", marginBottom: 12 }}>
            {info}
          </div>
        ) : null}

        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {item ? (
          <>
            <div style={{ marginBottom: 12 }}>
              <div style={{ fontWeight: 700 }}>{item.title}</div>
              <div style={styles.muted}>{item.description || ""}</div>
            </div>

            <div style={styles.grid2}>
              <div>
                <label>Tally rule</label>
                <select style={styles.input} value={tallyRule} onChange={(e) => setTallyRule(e.target.value)}>
                  {TALLY_RULES.map((rule) => (
                    <option key={rule} value={rule}>
                      {rule}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label>Ballot format</label>
                <select
                  style={styles.input}
                  value={ballotFormat}
                  onChange={(e) => setBallotFormat(e.target.value as "approval" | "ranking" | "score")}
                >
                  <option value="approval">approval</option>
                  <option value="ranking">ranking</option>
                  <option value="score">score</option>
                </select>
              </div>

              <div>
                <label>Committee size</label>
                <input
                  style={styles.input}
                  type="number"
                  min={1}
                  value={committeeSize}
                  onChange={(e) => setCommitteeSize(Number(e.target.value))}
                />
              </div>

              <div>
                <label>Quota type</label>
                <select
                  style={styles.input}
                  value={quotaType}
                  onChange={(e) => setQuotaType(e.target.value as "hare" | "droop")}
                >
                  <option value="hare">hare</option>
                  <option value="droop">droop</option>
                </select>
              </div>

              <div>
                <label>Access mode</label>
                <select
                  style={styles.input}
                  value={accessMode}
                  onChange={(e) => setAccessMode(e.target.value as "open" | "invite")}
                >
                  <option value="open">open</option>
                  <option value="invite">invite</option>
                </select>
              </div>

              <div>
                <label>Publish at (RFC3339, optional)</label>
                <input
                  style={styles.input}
                  value={publishAt}
                  onChange={(e) => setPublishAt(e.target.value)}
                />
              </div>

              <div style={{ display: "flex", alignItems: "center" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={showAggregates}
                    onChange={(e) => setShowAggregates(e.target.checked)}
                  />
                  show_aggregates
                </label>
              </div>
            </div>

            <hr style={styles.hr} />

            {ballotFormat === "approval" ? (
              <div style={styles.grid2}>
                <div>
                  <label>approval_max_choices</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={item.candidates.length}
                    value={approvalMaxChoices}
                    onChange={(e) => setApprovalMaxChoices(Number(e.target.value))}
                  />
                </div>
              </div>
            ) : null}

            {ballotFormat === "ranking" ? (
              <div style={styles.grid2}>
                <div>
                  <label>ranking_top_k</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={item.candidates.length}
                    value={rankingTopK}
                    onChange={(e) => setRankingTopK(Number(e.target.value))}
                  />
                </div>
              </div>
            ) : null}

            {ballotFormat === "score" ? (
              <div style={styles.grid2}>
                <div>
                  <label>score_min</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={scoreMin}
                    onChange={(e) => setScoreMin(Number(e.target.value))}
                  />
                </div>

                <div>
                  <label>score_max</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={scoreMax}
                    onChange={(e) => setScoreMax(Number(e.target.value))}
                  />
                </div>

                <div>
                  <label>score_step</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    value={scoreStep}
                    onChange={(e) => setScoreStep(Number(e.target.value))}
                  />
                </div>

                <div style={{ display: "flex", alignItems: "center" }}>
                  <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                    <input
                      type="checkbox"
                      checked={scoreAllowSkip}
                      onChange={(e) => setScoreAllowSkip(e.target.checked)}
                    />
                    score_allow_skip
                  </label>
                </div>
              </div>
            ) : null}

            <div style={{ marginTop: 16 }}>
              <button style={styles.btnPrimary} onClick={submit} disabled={saving || loading}>
                {saving ? "Сохранение…" : "Сохранить настройки"}
              </button>
            </div>
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Election JSON</h3>
          {item ? <JsonBlock value={item} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}