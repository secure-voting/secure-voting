import React, { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { styles } from "../../shared/ui/styles";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

const STEPS = [
  "Основные параметры",
  "Параметры бюллетеня",
  "Дополнительные параметры",
  "Проверка",
] as const;

const EXPERIMENT_TYPES = [
  { value: "algo", label: "algo" },
  { value: "behavior", label: "behavior" },
] as const;

const BALLOT_FORMATS = [
  { value: "approval", label: "approval" },
  { value: "ranking", label: "ranking" },
  { value: "score", label: "score" },
] as const;

const TALLY_RULES = [
  { value: "plurality", label: "Plurality" },
  { value: "approval", label: "Approval voting" },
  { value: "inverse_plurality", label: "Inverse plurality" },
  { value: "borda", label: "Borda" },
  { value: "black", label: "Black" },
  { value: "copeland_i", label: "Copeland I" },
  { value: "copeland_ii", label: "Copeland II" },
  { value: "copeland_iii", label: "Copeland III" },
  { value: "simpson", label: "Simpson (Maxmin)" },
  { value: "minmax", label: "Minmax" },
  { value: "threshold", label: "Threshold" },
  { value: "hare", label: "Hare" },
  { value: "inverse_borda", label: "Inverse Borda" },
  { value: "nanson", label: "Nanson" },
  { value: "coombs", label: "Coombs" },
  { value: "practical_condorcet", label: "Condorcet practical" },
] as const;

function StepHeader({
  current,
  onGo,
}: {
  current: number;
  onGo: (step: number) => void;
}) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(4, 1fr)",
        gap: 8,
        marginBottom: 16,
      }}
    >
      {STEPS.map((title, index) => {
        const isCurrent = current === index;
        const isDone = index < current;

        return (
          <button
            key={title}
            type="button"
            onClick={() => onGo(index)}
            style={{
              ...styles.btn,
              padding: 10,
              borderColor: isCurrent ? "#111827" : isDone ? "#bbf7d0" : "#e5e7eb",
              background: isCurrent ? "#111827" : isDone ? "#f0fdf4" : "white",
              color: isCurrent ? "white" : "inherit",
              fontWeight: isCurrent ? 700 : 500,
            }}
          >
            {index + 1}. {title}
          </button>
        );
      })}
    </div>
  );
}

function formatHint(format: "approval" | "ranking" | "score") {
  if (format === "approval") {
    return "Для approval-эксперимента можно указать максимум допустимых отметок.";
  }
  if (format === "ranking") {
    return "Для ranking-эксперимента можно ограничить только первые позиции через top-k.";
  }
  return "Для score-эксперимента укажите диапазон значений и шаг оценки.";
}

export function ExperimentCreatePage() {
  const nav = useNavigate();
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [step, setStep] = useState(0);

  const [type, setType] = useState<"algo" | "behavior">("algo");
  const [ballotFormat, setBallotFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [tallyRule, setTallyRule] = useState("borda");

  const [candidates, setCandidates] = useState(5);
  const [voters, setVoters] = useState(100);
  const [committeeSize, setCommitteeSize] = useState(1);
  const [quotaType, setQuotaType] = useState<"hare" | "droop">("hare");

  const [approvalMax, setApprovalMax] = useState(2);
  const [rankingTopK, setRankingTopK] = useState(3);

  const [scoreMin, setScoreMin] = useState(0);
  const [scoreMax, setScoreMax] = useState(10);
  const [scoreStep, setScoreStep] = useState(1);

  const [seed, setSeed] = useState("");

  const [includeAdvancedParams, setIncludeAdvancedParams] = useState(false);
  const [paramsText, setParamsText] = useState("{\n  \n}");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<unknown>(null);

  const parsedAdvancedParams = useMemo(() => {
    const trimmed = paramsText.trim();
    if (!includeAdvancedParams) {
      return { ok: true, value: {} as Record<string, unknown>, message: "" };
    }
    if (!trimmed) {
      return { ok: true, value: {} as Record<string, unknown>, message: "" };
    }

    try {
      const parsed = JSON.parse(trimmed);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        return {
          ok: false,
          value: {} as Record<string, unknown>,
          message: "Дополнительные параметры должны быть JSON-объектом",
        };
      }

      return {
        ok: true,
        value: parsed as Record<string, unknown>,
        message: "",
      };
    } catch {
      return {
        ok: false,
        value: {} as Record<string, unknown>,
        message: "Дополнительные параметры содержат невалидный JSON",
      };
    }
  }, [includeAdvancedParams, paramsText]);

  const structuredParams = useMemo(() => {
    const params: Record<string, unknown> = {
      ballot_format: ballotFormat,
      tally_rule: tallyRule,
      candidates,
      voters,
      committee_size: committeeSize,
    };

    if (committeeSize > 1) {
      params.quota_type = quotaType;
    }

    if (ballotFormat === "approval") {
      params.approval_max_choices = approvalMax;
    }

    if (ballotFormat === "ranking") {
      params.ranking_top_k = rankingTopK;
    }

    if (ballotFormat === "score") {
      params.score_min = scoreMin;
      params.score_max = scoreMax;
      params.score_step = scoreStep;
    }

    return params;
  }, [
    ballotFormat,
    tallyRule,
    candidates,
    voters,
    committeeSize,
    quotaType,
    approvalMax,
    rankingTopK,
    scoreMin,
    scoreMax,
    scoreStep,
  ]);

  const finalParams = useMemo(
    () => ({ ...structuredParams, ...parsedAdvancedParams.value }),
    [structuredParams, parsedAdvancedParams.value]
  );

  const validateStep = (targetStep: number) => {
    if (targetStep >= 0) {
      if (!type.trim()) return "Выберите тип эксперимента";
      if (!tallyRule.trim()) return "Выберите правило подсчёта";
      if (candidates < 2) return "Количество кандидатов должно быть не меньше 2";
      if (voters < 1) return "Количество избирателей должно быть не меньше 1";
      if (committeeSize < 1) return "Размер комитета должен быть не меньше 1";
    }

    if (targetStep >= 1) {
      if (ballotFormat === "approval") {
        if (approvalMax < 1) return "approval_max_choices должен быть не меньше 1";
        if (approvalMax > candidates) {
          return "approval_max_choices не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "ranking") {
        if (rankingTopK < 1) return "ranking_top_k должен быть не меньше 1";
        if (rankingTopK > candidates) {
          return "ranking_top_k не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "score") {
        if (scoreStep <= 0) return "Шаг оценки должен быть больше 0";
        if (scoreMin > scoreMax) {
          return "Нижняя граница оценки не может быть больше верхней";
        }
        if ((scoreMax - scoreMin) % scoreStep !== 0) {
          return "Диапазон оценок должен делиться на шаг без остатка";
        }
      }
    }

    if (targetStep >= 2) {
      if (seed.trim()) {
        const seedNum = Number(seed);
        if (!Number.isFinite(seedNum)) {
          return "Seed должен быть числом";
        }
      }

      if (!parsedAdvancedParams.ok) {
        return parsedAdvancedParams.message;
      }
    }

    return null;
  };

  const goNext = () => {
    const validationError = validateStep(step);
    if (validationError) {
      setErr(validationError);
      return;
    }

    setErr(null);
    setStep((prev) => Math.min(prev + 1, STEPS.length - 1));
  };

  const goBack = () => {
    setErr(null);
    setStep((prev) => Math.max(prev - 1, 0));
  };

  const goToStep = (nextStep: number) => {
    if (nextStep <= step) {
      setErr(null);
      setStep(nextStep);
      return;
    }

    const validationError = validateStep(nextStep - 1);
    if (validationError) {
      setErr(validationError);
      return;
    }

    setErr(null);
    setStep(nextStep);
  };

  const submit = async () => {
    if (!token) return;

    const validationError = validateStep(2);
    if (validationError) {
      setErr(validationError);
      return;
    }

    setLoading(true);
    setErr(null);
    setCreatedId(null);
    setRawResp(null);

    try {
      const body: {
        type: string;
        params: Record<string, unknown>;
        seed?: number;
      } = {
        type,
        params: finalParams,
      };

      if (seed.trim()) {
        body.seed = Number(seed);
      }

      const id = await api.experiments.create(token, body);
      setCreatedId(id);

      addNotification({
        kind: "success",
        title: "Эксперимент создан",
        message: `Создан эксперимент с id ${id}`,
      });

      if (IS_DEV) {
        setRawResp({ id, body });
      }

      setStep(STEPS.length - 1);
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось создать эксперимент");
      }
    } finally {
      setLoading(false);
    }
  };

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
          <h2 style={{ margin: 0 }}>Создание эксперимента</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/dashboard/researcher" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К dashboard</button>
            </Link>
            <Link to="/research/experiments" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К списку</button>
            </Link>
          </div>
        </div>

        <div style={{ marginTop: 8, ...styles.muted }}>
          Пошаговый мастер подготовки параметров алгоритмического эксперимента.
        </div>

        <div style={{ marginTop: 16 }}>
          <StepHeader current={step} onGo={goToStep} />
        </div>

        <ErrorBanner error={err} />

        {step === 0 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.grid2}>
              <div>
                <label>Тип эксперимента</label>
                <select
                  style={styles.input}
                  value={type}
                  onChange={(e) => setType(e.target.value as "algo" | "behavior")}
                >
                  {EXPERIMENT_TYPES.map((item) => (
                    <option key={item.value} value={item.value}>
                      {item.label}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label>Формат бюллетеня</label>
                <select
                  style={styles.input}
                  value={ballotFormat}
                  onChange={(e) => setBallotFormat(e.target.value as "approval" | "ranking" | "score")}
                >
                  {BALLOT_FORMATS.map((item) => (
                    <option key={item.value} value={item.value}>
                      {item.label}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label>Правило подсчёта</label>
                <select
                  style={styles.input}
                  value={tallyRule}
                  onChange={(e) => setTallyRule(e.target.value)}
                >
                  {TALLY_RULES.map((rule) => (
                    <option key={rule.value} value={rule.value}>
                      {rule.label}
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
                  onChange={(e) => setCommitteeSize(Number(e.target.value))}
                />
              </div>

              <div>
                <label>Количество кандидатов</label>
                <input
                  style={styles.input}
                  type="number"
                  min={2}
                  value={candidates}
                  onChange={(e) => setCandidates(Number(e.target.value))}
                />
              </div>

              <div>
                <label>Количество избирателей</label>
                <input
                  style={styles.input}
                  type="number"
                  min={1}
                  value={voters}
                  onChange={(e) => setVoters(Number(e.target.value))}
                />
              </div>

              <div>
                <label>Тип квоты</label>
                <select
                  style={styles.input}
                  value={quotaType}
                  onChange={(e) => setQuotaType(e.target.value as "hare" | "droop")}
                >
                  <option value="hare">hare</option>
                  <option value="droop">droop</option>
                </select>
              </div>
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <div><b>Проверки на этом шаге:</b></div>
              <div style={{ marginTop: 6, ...styles.muted }}>
                • задан тип эксперимента;
                <br />
                • выбраны формат бюллетеня и правило подсчёта;
                <br />
                • количество кандидатов не меньше 2;
                <br />
                • количество избирателей не меньше 1.
              </div>
            </div>
          </div>
        ) : null}

        {step === 1 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={{ ...styles.card, background: "#f9fafb" }}>
              {formatHint(ballotFormat)}
            </div>

            {ballotFormat === "approval" ? (
              <div style={styles.grid2}>
                <div>
                  <label>Максимум отметок</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={Math.max(candidates, 1)}
                    value={approvalMax}
                    onChange={(e) => setApprovalMax(Number(e.target.value))}
                  />
                </div>
              </div>
            ) : null}

            {ballotFormat === "ranking" ? (
              <div style={styles.grid2}>
                <div>
                  <label>top-k</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={Math.max(candidates, 1)}
                    value={rankingTopK}
                    onChange={(e) => setRankingTopK(Number(e.target.value))}
                  />
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
              </div>
            ) : null}

            <SummaryGrid
              items={[
                { label: "Формат", value: ballotFormat },
                { label: "Кандидатов", value: String(candidates) },
                { label: "Избирателей", value: String(voters) },
                { label: "Размер комитета", value: String(committeeSize) },
              ]}
            />
          </div>
        ) : null}

        {step === 2 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.grid2}>
              <div>
                <label>Seed</label>
                <input
                  style={styles.input}
                  value={seed}
                  onChange={(e) => setSeed(e.target.value)}
                  placeholder="Например: 42"
                />
              </div>
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <input
                  type="checkbox"
                  checked={includeAdvancedParams}
                  onChange={(e) => setIncludeAdvancedParams(e.target.checked)}
                />
                Добавить дополнительные JSON-параметры
              </label>

              <div style={{ marginTop: 8, ...styles.muted }}>
                Дополнительные параметры будут объединены со структурированными полями формы.
                При совпадении ключей значения из JSON переопределят значения из формы.
              </div>
            </div>

            {includeAdvancedParams ? (
              <div>
                <label>Advanced params (JSON object)</label>
                <textarea
                  style={{
                    ...styles.input,
                    minHeight: 220,
                    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
                  }}
                  value={paramsText}
                  onChange={(e) => setParamsText(e.target.value)}
                />
              </div>
            ) : null}

            <div style={{ ...styles.card, background: parsedAdvancedParams.ok ? "#f0fdf4" : "#fff1f2" }}>
              <div><b>Статус JSON-параметров:</b></div>
              <div style={{ marginTop: 6 }}>
                {parsedAdvancedParams.ok ? "Корректно" : parsedAdvancedParams.message}
              </div>
            </div>
          </div>
        ) : null}

        {step === 3 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <SummaryGrid
              items={[
                { label: "Тип эксперимента", value: type },
                { label: "Формат бюллетеня", value: ballotFormat },
                { label: "Правило подсчёта", value: tallyRule },
                { label: "Количество кандидатов", value: String(candidates) },
                { label: "Количество избирателей", value: String(voters) },
                { label: "Размер комитета", value: String(committeeSize) },
                { label: "Тип квоты", value: committeeSize > 1 ? quotaType : "не используется" },
                { label: "Seed", value: seed.trim() || "не задан" },
              ]}
            />

            <div style={styles.card}>
              <h3 style={{ marginTop: 0 }}>Итоговые параметры эксперимента</h3>
              <JsonBlock value={finalParams} />
            </div>

            {createdId ? (
              <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                <div style={{ fontWeight: 700 }}>Эксперимент создан</div>
                <div style={{ marginTop: 6 }}>ID: {createdId}</div>

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btnPrimary} onClick={() => nav("/research/experiments")}>
                    К списку экспериментов
                  </button>
                  <button style={styles.btn} onClick={() => nav("/research/runs")}>
                    К запускам
                  </button>
                </div>
              </div>
            ) : null}
          </div>
        ) : null}

        <hr style={styles.hr} />

        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, flexWrap: "wrap" }}>
          <button style={styles.btn} onClick={goBack} disabled={step === 0 || loading}>
            Назад
          </button>

          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            {step < STEPS.length - 1 ? (
              <button style={styles.btnPrimary} onClick={goNext} disabled={loading}>
                Далее
              </button>
            ) : (
              <button style={styles.btnPrimary} onClick={submit} disabled={loading || Boolean(createdId)}>
                {loading ? "Создание…" : createdId ? "Уже создано" : "Создать эксперимент"}
              </button>
            )}
          </div>
        </div>
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Create response</h3>
          {rawResp ? <JsonBlock value={rawResp} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}