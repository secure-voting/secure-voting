import React, { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { ElectionDetail, UpdateElectionRulesInput, TallyRuleInfo } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { styles } from "../../shared/ui/styles";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function accessModeLabel(value: "open" | "invite") {
  return value === "open" ? "Открытый доступ" : "Только по приглашению";
}

function Hint({ text }: { text: string }) {
  return (
    <span
      title={text}
      style={{
        display: "inline-flex",
        marginLeft: 6,
        width: 18,
        height: 18,
        borderRadius: "50%",
        border: "1px solid #98a2b3",
        alignItems: "center",
        justifyContent: "center",
        fontSize: 12,
        cursor: "help",
        userSelect: "none",
      }}
    >
      ?
    </span>
  );
}

function toLocalInputValue(date: Date) {
  const adjusted = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return adjusted.toISOString().slice(0, 16);
}

function toRFC3339FromLocalInput(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toISOString();
}

function supportsBallotFormat(
  rule: TallyRuleInfo | undefined,
  format: "approval" | "ranking" | "score"
) {
  return Boolean(rule?.ballot_formats?.includes(format));
}

function selectedRuleInfo(
  rules: TallyRuleInfo[],
  ruleId: string
) {
  return rules.find((rule) => rule.id === ruleId);
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
  const [delayPublish, setDelayPublish] = useState(false);
  const [publishAtLocal, setPublishAtLocal] = useState("");
  const [showAggregates, setShowAggregates] = useState(true);

  const [approvalMaxChoices, setApprovalMaxChoices] = useState<number>(1);
  const [limitRankingTopK, setLimitRankingTopK] = useState(true);
  const [rankingTopKInput, setRankingTopKInput] = useState("1");

  const [scoreMin, setScoreMin] = useState<number>(0);
  const [scoreMax, setScoreMax] = useState<number>(10);
  const [scoreStep, setScoreStep] = useState<number>(1);
  const [scoreAllowSkip, setScoreAllowSkip] = useState(false);

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [availableRules, setAvailableRules] = useState<TallyRuleInfo[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);

  const abortRef = useRef<AbortController | null>(null);
  const currentRule = selectedRuleInfo(availableRules, tallyRule);

  const normalizedTopK = () => {
    const candidatesCount = item?.candidates.length ?? 1;
    const raw = Number(rankingTopKInput);
    if (!Number.isFinite(raw) || raw < 1) return 1;
    if (raw > candidatesCount) return candidatesCount;
    return Math.floor(raw);
  };

  const hydrate = (e: ElectionDetail) => {
    setTallyRule(e.tally_rule);
    setBallotFormat((e.ballot_format as "approval" | "ranking" | "score") || "ranking");
    setCommitteeSize(e.committee_size ?? 1);
    setQuotaType((e.quota_type as "hare" | "droop") ?? "hare");
    setAccessMode((e.access_mode as "open" | "invite") ?? "open");
    setDelayPublish(Boolean(e.publish_at));
    setPublishAtLocal(e.publish_at ? toLocalInputValue(new Date(e.publish_at)) : "");
    setShowAggregates(Boolean(e.show_aggregates));
    setApprovalMaxChoices(e.approval_max_choices ?? 1);
    setLimitRankingTopK(e.ranking_top_k != null);
    setRankingTopKInput(String(e.ranking_top_k ?? 1));
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

  const loadRules = useCallback(async () => {
    if (!token) return;

    setRulesLoading(true);
    try {
      const items = await api.capabilities.tallyRules(token);
      const electionRules = items.filter((item) => item.supports_election_tally);
      setAvailableRules(electionRules);

      if (electionRules.length > 0 && !electionRules.some((item) => item.id === tallyRule)) {
        setTallyRule(electionRules[0].id);
      }
    } catch (e: any) {
      if (e?.status === 401) setToken(null);
      else setErr((prev) => prev || "Не удалось загрузить список правил");
    } finally {
      setRulesLoading(false);
    }
  }, [token, setToken, tallyRule]);

  useEffect(() => {
    load();
    return () => abortRef.current?.abort();
  }, [load]);


  useEffect(() => {
    if (!currentRule) return;

    if (!supportsBallotFormat(currentRule, ballotFormat)) {
      const nextFormat = currentRule.ballot_formats.find(
        (f): f is "approval" | "ranking" | "score" =>
          f === "approval" || f === "ranking" || f === "score"
      );
      if (nextFormat) {
        setBallotFormat(nextFormat);
      }
    }

    if (!currentRule.supports_ranking_top_k && ballotFormat === "ranking") {
      setLimitRankingTopK(false);
      setRankingTopKInput("1");
    }
  }, [currentRule, ballotFormat]);

  useEffect(() => {
    loadRules();
  }, [loadRules]);


  const validate = (): string | null => {
    if (!item) return "Нет данных голосования";

    const candidatesCount = item.candidates.length;

    if (!tallyRule.trim()) return "Выберите правило подсчёта";
    if (!availableRules.some((item) => item.id === tallyRule)) return "Недопустимое правило подсчёта";
    if (committeeSize < 1) return "Размер комитета должен быть не меньше 1";

    if (ballotFormat === "approval") {
      if (approvalMaxChoices < 1) return "Максимум отметок должен быть не меньше 1";
      if (approvalMaxChoices > candidatesCount) return "Максимум отметок не может превышать число кандидатов";
    }

    if (ballotFormat === "ranking" && limitRankingTopK) {
      const topK = normalizedTopK();
      if (topK < 1) return "top-k должен быть не меньше 1";
      if (topK > candidatesCount) return "top-k не может превышать число кандидатов";
    }

    if (ballotFormat === "score") {
      if (scoreStep <= 0) return "Шаг оценки должен быть больше 0";
      if (scoreMin > scoreMax) return "Нижняя граница оценки не может быть больше верхней";
      if ((scoreMax - scoreMin) % scoreStep !== 0) {
        return "Диапазон оценок должен делиться на шаг без остатка";
      }
    }

    if (delayPublish) {
      const publishAtRFC3339 = toRFC3339FromLocalInput(publishAtLocal);
      if (!publishAtRFC3339) return "Укажите корректную дату и время публикации";

      const publishTs = Date.parse(publishAtRFC3339);
      const endTs = Date.parse(item.end_at);
      if (!Number.isNaN(endTs) && publishTs <= endTs) {
        return "Дата публикации результатов должна быть позже окончания голосования";
      }
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
        tally_rule: tallyRule,
        ballot_format: ballotFormat,
        committee_size: committeeSize,
        quota_type: committeeSize > 1 ? quotaType : null,
        access_mode: accessMode,
        publish_at: delayPublish ? toRFC3339FromLocalInput(publishAtLocal) : null,
        show_aggregates: showAggregates,
      };

      if (!currentRule?.requires_committee_size) {
        body.committee_size = undefined;
      }

      if (!currentRule?.supports_quota_type) {
        body.quota_type = null;
      }

      if (!currentRule?.requires_approval_max_choices) {
        delete body.approval_max_choices;
      }

      if (!currentRule?.supports_ranking_top_k) {
        body.ranking_top_k = null;
      }

      if (!currentRule?.requires_score_range) {
        delete body.score_min;
        delete body.score_max;
        delete body.score_step;
        delete body.score_allow_skip;
      }

      if (ballotFormat === "approval") {
        body.approval_max_choices = approvalMaxChoices;
      }

      if (ballotFormat === "ranking") {
        body.ranking_top_k = limitRankingTopK ? normalizedTopK() : null;
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
                <label>Правило подсчёта</label>
                <select style={styles.input} value={tallyRule} onChange={(e) => setTallyRule(e.target.value)}>
                  {availableRules.map((rule) => (
                    <option key={rule.id} value={rule.id}>
                      {rule.label}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label>
                  Формат бюллетеня
                  <Hint text="Тип данных, которые вводит избиратель: approval, ranking или score." />
                </label>
                <select
                  style={styles.input}
                  value={ballotFormat}
                  onChange={(e) => setBallotFormat(e.target.value as "approval" | "ranking" | "score")}
                >
                  {(["approval", "ranking", "score"] as const).map((format) => (
                    <option
                      key={format}
                      value={format}
                      disabled={!supportsBallotFormat(currentRule, format)}
                    >
                      {format}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label>Размер комитета</label>
                <input
                  style={styles.input}
                  type="number"
                  min={1}
                  value={committeeSize}
                  disabled={!currentRule?.requires_committee_size}
                  onChange={(e) => setCommitteeSize(Number(e.target.value))}
                />
              </div>

              <div>
                <label>
                  Тип квоты
                  <Hint text="Квота определяет порог голосов, необходимый для распределения мандатов в некоторых многомандатных правилах." />
                </label>
                <select
                  style={styles.input}
                  value={quotaType}
                  disabled={committeeSize <= 1 || !currentRule?.supports_quota_type}
                  onChange={(e) => setQuotaType(e.target.value as "hare" | "droop")}
                >
                  <option value="hare">hare</option>
                  <option value="droop">droop</option>
                </select>

                <div style={{ marginTop: 6, fontSize: 13, color: "#667085" }}>
                  {committeeSize <= 1
                    ? "Для одного победителя квота не используется."
                    : quotaType === "hare"
                    ? "Квота Хэра: число голосов на один мандат."
                    : "Квота Друпа: более строгий порог избрания."}
                </div>
              </div>

              <div>
                <label>
                  Access mode
                  <Hint text="Определяет, смогут ли пользователи участвовать свободно или только по приглашению." />
                </label>
                <select
                  style={styles.input}
                  value={accessMode}
                  onChange={(e) => setAccessMode(e.target.value as "open" | "invite")}
                >
                  <option value="open">{accessModeLabel("open")}</option>
                  <option value="invite">{accessModeLabel("invite")}</option>
                </select>
              </div>

              <div style={{ ...styles.card, background: "#f9fafb" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={delayPublish}
                    onChange={(e) => setDelayPublish(e.target.checked)}
                  />
                  <span>
                    Отложить публикацию результатов
                    <Hint text="Если включено, результаты будут опубликованы в заданный момент времени." />
                  </span>
                </label>

                {delayPublish ? (
                  <div style={{ marginTop: 12, maxWidth: 420 }}>
                    <label>Дата и время публикации</label>
                    <input
                      style={styles.input}
                      type="datetime-local"
                      value={publishAtLocal}
                      onChange={(e) => setPublishAtLocal(e.target.value)}
                    />
                  </div>
                ) : (
                  <div style={{ marginTop: 12, ...styles.muted }}>
                    Результаты можно будет публиковать вручную после завершения голосования.
                  </div>
                )}
              </div>

              <div style={{ display: "flex", alignItems: "center" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={showAggregates}
                    onChange={(e) => setShowAggregates(e.target.checked)}
                  />
                  Показывать агрегированные данные
                  <Hint text="Определяет, будут ли в опубликованных результатах показаны агрегированные метрики и сводные значения." />
                </label>
              </div>
            </div>

            <hr style={styles.hr} />

            {ballotFormat === "approval" && currentRule?.requires_approval_max_choices ? (
              <div style={styles.grid2}>
                <div>
                  <label>Максимум отметок</label>
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


            {ballotFormat === "ranking" && currentRule?.supports_ranking_top_k ? (
              <div style={{ display: "grid", gap: 12 }}>
                <label style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <input
                    type="checkbox"
                    checked={limitRankingTopK}
                    onChange={(e) => setLimitRankingTopK(e.target.checked)}
                  />
                  <span>
                    Ограничить ранжирование top-k
                    <Hint text="Если включено, избиратель сможет выбрать только первые k позиций." />
                  </span>
                </label>

                <div>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    disabled={!limitRankingTopK}
                    value={rankingTopKInput}
                    onChange={(e) => setRankingTopKInput(e.target.value)}
                    onBlur={() => setRankingTopKInput(String(normalizedTopK()))}
                  />
                  <div style={{ marginTop: 6, fontSize: 13, color: "#667085" }}>
                    Максимально допустимое значение: {item.candidates.length}
                  </div>
                </div>
              </div>
            ) : null}

            {ballotFormat === "score" ? (
              <div style={styles.grid2}>
                <div>
                  <label>Нижняя граница оценки</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={scoreMin}
                    onChange={(e) => setScoreMin(Number(e.target.value))}
                  />
                </div>

                <div>
                  <label>Верхняя граница оценки</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={scoreMax}
                    onChange={(e) => setScoreMax(Number(e.target.value))}
                  />
                </div>

                <div>
                  <label>Шаг оценки</label>
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
                    Разрешить пропуск оценки
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