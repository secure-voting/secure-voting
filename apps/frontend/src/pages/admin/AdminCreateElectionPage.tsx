import React, { useMemo, useRef, useState } from "react";
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
  "Общие сведения",
  "Кандидаты",
  "Правило и бюллетень",
  "Доступ и публикация",
  "Проверка",
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

function toLocalInputValue(date: Date) {
  const adjusted = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return adjusted.toISOString().slice(0, 16);
}

function toRFC3339FromLocalInput(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toISOString();
}

function parseCandidateNames(text: string) {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
}

function candidateValidation(text: string) {
  const names = parseCandidateNames(text);

  if (names.length < 2) {
    return {
      ok: false,
      message: "Добавьте минимум двух кандидатов",
      names: [] as string[],
    };
  }

  const seen = new Map<string, number>();
  const duplicates: string[] = [];

  names.forEach((name) => {
    const key = name.toLowerCase();
    const count = seen.get(key) ?? 0;
    seen.set(key, count + 1);
    if (count === 1) duplicates.push(name);
  });

  if (duplicates.length > 0) {
    return {
      ok: false,
      message: `Имена кандидатов не должны повторяться: ${duplicates.join(", ")}`,
      names,
    };
  }

  return {
    ok: true,
    message: "",
    names,
  };
}

function ballotFormatHint(format: "approval" | "ranking" | "score") {
  if (format === "approval") {
    return "Для одобрительного бюллетеня задайте максимальное число допустимых отметок.";
  }
  if (format === "ranking") {
    return "Для ранжированного бюллетеня задайте top-k, если требуется ограничить только первые позиции.";
  }
  return "Для оценочного бюллетеня задайте диапазон значений и шаг оценки.";
}

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
        gridTemplateColumns: "repeat(5, 1fr)",
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

export function AdminCreateElectionPage() {
  const nav = useNavigate();
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const [step, setStep] = useState(0);

  const [title, setTitle] = useState("Новое голосование");
  const [description, setDescription] = useState("");

  const [startAtLocal, setStartAtLocal] = useState(
    toLocalInputValue(new Date(Date.now() + 10 * 60_000))
  );
  const [endAtLocal, setEndAtLocal] = useState(
    toLocalInputValue(new Date(Date.now() + 70 * 60_000))
  );

  const [candidatesText, setCandidatesText] = useState("Alice\nBob\nCarol");

  const [ballotFormat, setBallotFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [tallyRule, setTallyRule] = useState("plurality");

  const [committeeSize, setCommitteeSize] = useState<number>(1);
  const [quotaType, setQuotaType] = useState<"hare" | "droop">("hare");

  const [approvalMax, setApprovalMax] = useState<number>(2);
  const [rankingTopK, setRankingTopK] = useState<number>(3);

  const [scoreMin, setScoreMin] = useState<number>(0);
  const [scoreMax, setScoreMax] = useState<number>(10);
  const [scoreStep, setScoreStep] = useState<number>(1);
  const [scoreAllowSkip, setScoreAllowSkip] = useState<boolean>(false);

  const [accessMode, setAccessMode] = useState<"open" | "invite">("open");
  const [showAggregates, setShowAggregates] = useState(true);

  const [delayPublish, setDelayPublish] = useState(false);
  const [publishAtLocal, setPublishAtLocal] = useState(
    toLocalInputValue(new Date(Date.now() + 24 * 60 * 60_000))
  );

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [createdID, setCreatedID] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<unknown>(null);

  const parsedCandidates = useMemo(() => candidateValidation(candidatesText), [candidatesText]);
  const candidateNames = parsedCandidates.names;

  const startAtRFC3339 = useMemo(() => toRFC3339FromLocalInput(startAtLocal), [startAtLocal]);
  const endAtRFC3339 = useMemo(() => toRFC3339FromLocalInput(endAtLocal), [endAtLocal]);
  const publishAtRFC3339 = useMemo(
    () => (delayPublish ? toRFC3339FromLocalInput(publishAtLocal) : ""),
    [delayPublish, publishAtLocal]
  );

  const validateStep = (targetStep: number) => {
    if (targetStep >= 0) {
      if (!title.trim()) return "Введите название голосования";
      if (!startAtRFC3339) return "Укажите корректную дату и время начала";
      if (!endAtRFC3339) return "Укажите корректную дату и время окончания";

      const startTs = new Date(startAtRFC3339).getTime();
      const endTs = new Date(endAtRFC3339).getTime();

      if (startTs <= Date.now()) {
        return "Дата начала должна быть позже текущего времени";
      }
      if (startTs >= endTs) {
        return "Дата начала должна быть раньше даты окончания";
      }
    }

    if (targetStep >= 1) {
      if (!parsedCandidates.ok) return parsedCandidates.message;
    }

    if (targetStep >= 2) {
      if (!tallyRule.trim()) return "Выберите правило подсчёта";
      if (committeeSize < 1) return "Размер комитета должен быть не меньше 1";

      const candidatesCount = candidateNames.length;

      if (ballotFormat === "approval") {
        if (approvalMax < 1) return "approval_max_choices должен быть не меньше 1";
        if (approvalMax > candidatesCount) {
          return "approval_max_choices не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "ranking") {
        if (rankingTopK < 1) return "ranking_top_k должен быть не меньше 1";
        if (rankingTopK > candidatesCount) {
          return "ranking_top_k не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "score") {
        if (scoreStep <= 0) return "Шаг оценки должен быть больше 0";
        if (scoreMin > scoreMax) return "Нижняя граница оценки не может быть больше верхней";
        if ((scoreMax - scoreMin) % scoreStep !== 0) {
          return "Диапазон оценок должен делиться на шаг без остатка";
        }
      }
    }

    if (targetStep >= 3) {
      if (delayPublish && !publishAtRFC3339) {
        return "Укажите корректную дату и время публикации";
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

  const handleCandidatesFile = async (file: File | null) => {
    if (!file) return;

    try {
      const text = await file.text();
      setCandidatesText(text.replace(/\r\n/g, "\n"));
      setErr(null);
    } catch {
      setErr("Не удалось прочитать файл с кандидатами");
    } finally {
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  const submit = async () => {
    if (!token) return;

    const validationError = validateStep(3);
    if (validationError) {
      setErr(validationError);
      return;
    }

    setLoading(true);
    setErr(null);
    setCreatedID(null);
    setRawResp(null);

    try {
      const body: Record<string, unknown> = {
        title: title.trim(),
        description: description.trim() ? description.trim() : null,
        start_at: startAtRFC3339,
        end_at: endAtRFC3339,
        tally_rule: tallyRule,
        ballot_format: ballotFormat,
        committee_size: committeeSize,
        quota_type: committeeSize > 1 ? quotaType : null,
        access_mode: accessMode,
        publish_at: delayPublish ? publishAtRFC3339 : null,
        show_aggregates: showAggregates,
        candidates: candidateNames.map((name) => ({ name })),
      };

      if (ballotFormat === "approval") {
        body.approval_max_choices = approvalMax;
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

      const id = await api.elections.create(token, body);
      setCreatedID(id);

      addNotification({
        kind: "success",
        title: "Голосование создано",
        message: `Создано новое голосование с id ${id}`,
      });

      if (IS_DEV) {
        setRawResp({ id, body });
      }

      setStep(STEPS.length - 1);
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setErr(e?.message || "Не удалось создать голосование");
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
          <h2 style={{ margin: 0 }}>Создание голосования</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to="/dashboard/admin" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К dashboard</button>
            </Link>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К списку</button>
            </Link>
          </div>
        </div>

        <div style={{ marginTop: 8, ...styles.muted }}>
          Пошаговый мастер создания голосования с клиентской валидацией по ключевым требованиям ТЗ.
        </div>

        <div style={{ marginTop: 16 }}>
          <StepHeader current={step} onGo={goToStep} />
        </div>

        <ErrorBanner error={err} />

        {step === 0 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.grid2}>
              <div>
                <label>Название</label>
                <input style={styles.input} value={title} onChange={(e) => setTitle(e.target.value)} />
              </div>

              <div>
                <label>Описание</label>
                <input
                  style={styles.input}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>

              <div>
                <label>Дата и время начала</label>
                <input
                  style={styles.input}
                  type="datetime-local"
                  value={startAtLocal}
                  onChange={(e) => setStartAtLocal(e.target.value)}
                />
              </div>

              <div>
                <label>Дата и время окончания</label>
                <input
                  style={styles.input}
                  type="datetime-local"
                  value={endAtLocal}
                  onChange={(e) => setEndAtLocal(e.target.value)}
                />
              </div>
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <div><b>Проверки на этом шаге:</b></div>
              <div style={{ ...styles.muted, marginTop: 6 }}>
                • название не пустое;
                <br />
                • дата начала позже текущего времени;
                <br />
                • дата окончания позже даты начала.
              </div>
            </div>
          </div>
        ) : null}

        {step === 1 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                gap: 10,
                alignItems: "center",
                flexWrap: "wrap",
              }}
            >
              <div style={styles.muted}>
                Введите кандидатов по одному на строку или загрузите текстовый файл.
              </div>

              <input
                ref={fileInputRef}
                type="file"
                accept=".txt,.csv"
                onChange={(e) => handleCandidatesFile(e.target.files?.[0] ?? null)}
              />
            </div>

            <div>
              <label>Кандидаты (по одному на строку)</label>
              <textarea
                style={{
                  ...styles.input,
                  minHeight: 220,
                  fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
                }}
                value={candidatesText}
                onChange={(e) => setCandidatesText(e.target.value)}
              />
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <div><b>Предпросмотр:</b></div>
              <div style={{ marginTop: 8, display: "grid", gap: 6 }}>
                {candidateNames.length > 0 ? (
                  candidateNames.map((name, index) => (
                    <div key={`${name}-${index}`}>
                      {index + 1}. {name}
                    </div>
                  ))
                ) : (
                  <div style={styles.muted}>Список пока пуст</div>
                )}
              </div>
            </div>

            <div style={{ ...styles.card, background: parsedCandidates.ok ? "#f0fdf4" : "#fff1f2" }}>
              <div><b>Статус валидации:</b></div>
              <div style={{ marginTop: 6 }}>
                {parsedCandidates.ok
                  ? `Корректно. Кандидатов: ${candidateNames.length}`
                  : parsedCandidates.message}
              </div>
            </div>
          </div>
        ) : null}

        {step === 2 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.grid2}>
              <div>
                <label>Формат бюллетеня</label>
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
              {ballotFormatHint(ballotFormat)}
            </div>

            {ballotFormat === "approval" ? (
              <div style={styles.grid2}>
                <div>
                  <label>Максимум отметок</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={Math.max(candidateNames.length, 1)}
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
                    max={Math.max(candidateNames.length, 1)}
                    value={rankingTopK}
                    onChange={(e) => setRankingTopK(Number(e.target.value))}
                  />
                </div>
              </div>
            ) : null}

            {ballotFormat === "score" ? (
              <div style={styles.grid2}>
                <div>
                  <label>Нижняя граница</label>
                  <input
                    style={styles.input}
                    type="number"
                    value={scoreMin}
                    onChange={(e) => setScoreMin(Number(e.target.value))}
                  />
                </div>

                <div>
                  <label>Верхняя граница</label>
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
          </div>
        ) : null}

        {step === 3 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.grid2}>
              <div>
                <label>Режим доступа</label>
                <select
                  style={styles.input}
                  value={accessMode}
                  onChange={(e) => setAccessMode(e.target.value as "open" | "invite")}
                >
                  <option value="open">open</option>
                  <option value="invite">invite</option>
                </select>
              </div>

              <div style={{ display: "flex", alignItems: "center" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={showAggregates}
                    onChange={(e) => setShowAggregates(e.target.checked)}
                  />
                  Показывать агрегированные данные в результатах
                </label>
              </div>
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={delayPublish}
                    onChange={(e) => setDelayPublish(e.target.checked)}
                  />
                  Отложить публикацию результатов
                </label>
              </div>

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

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <div><b>Примечание:</b></div>
              <div style={{ marginTop: 6, ...styles.muted }}>
                Если выбран режим invite, сами приглашения можно будет создать на карточке голосования после его создания.
              </div>
            </div>
          </div>
        ) : null}

        {step === 4 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <SummaryGrid
              items={[
                { label: "Название", value: title.trim() || "—" },
                { label: "Формат бюллетеня", value: ballotFormat },
                { label: "Правило подсчёта", value: tallyRule },
                { label: "Размер комитета", value: String(committeeSize) },
                { label: "Тип квоты", value: committeeSize > 1 ? quotaType : "не используется" },
                { label: "Режим доступа", value: accessMode },
                { label: "Показывать агрегаты", value: showAggregates ? "да" : "нет" },
                { label: "Начало", value: startAtRFC3339 || "—" },
                { label: "Окончание", value: endAtRFC3339 || "—" },
                { label: "Отложенная публикация", value: delayPublish ? publishAtRFC3339 || "—" : "нет" },
                { label: "Кандидатов", value: String(candidateNames.length) },
              ]}
            />

            <div style={styles.card}>
              <h3 style={{ marginTop: 0 }}>Кандидаты</h3>
              <div style={{ display: "grid", gap: 6 }}>
                {candidateNames.map((name, index) => (
                  <div key={`${name}-${index}`}>
                    {index + 1}. {name}
                  </div>
                ))}
              </div>
            </div>

            <div style={styles.card}>
              <h3 style={{ marginTop: 0 }}>Параметры бюллетеня</h3>

              {ballotFormat === "approval" ? (
                <div>Максимум отметок: {approvalMax}</div>
              ) : null}

              {ballotFormat === "ranking" ? (
                <div>top-k: {rankingTopK}</div>
              ) : null}

              {ballotFormat === "score" ? (
                <div style={{ display: "grid", gap: 6 }}>
                  <div>Диапазон: {scoreMin}..{scoreMax}</div>
                  <div>Шаг: {scoreStep}</div>
                  <div>Разрешить пропуск: {scoreAllowSkip ? "да" : "нет"}</div>
                </div>
              ) : null}
            </div>

            {createdID ? (
              <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                <div style={{ fontWeight: 700 }}>Голосование создано</div>
                <div style={{ marginTop: 6 }}>ID: {createdID}</div>

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btnPrimary} onClick={() => nav(`/elections/${createdID}`)}>
                    Открыть карточку
                  </button>
                  <button style={styles.btn} onClick={() => nav("/elections")}>
                    К списку голосований
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
              <button style={styles.btnPrimary} onClick={submit} disabled={loading || Boolean(createdID)}>
                {loading ? "Создание…" : createdID ? "Уже создано" : "Создать голосование"}
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