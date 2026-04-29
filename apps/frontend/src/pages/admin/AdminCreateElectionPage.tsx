import React, { useMemo, useState, useEffect } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { DateTimeField } from "../../shared/ui/DateTimeField";
import { styles } from "../../shared/ui/styles";
import type { CandidateDraft, CandidatePayload, TallyRuleInfo } from "../../shared/api/types";
import { mergeRuleItems } from "../../shared/utils/mergeRuleItems";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

const STEPS = [
  "Общие сведения",
  "Кандидаты",
  "Правило и бюллетень",
  "Доступ и публикация",
  "Проверка",
] as const;

const CREATE_ELECTION_DRAFT_KEY = "secure-voting:create-election-draft:v1";

function toLocalInputValue(date: Date) {
  const adjusted = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return adjusted.toISOString().slice(0, 16);
}

function toRFC3339FromLocalInput(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toISOString();
}

function normalizedCandidateName(value: string) {
  return value.trim().replace(/\s+/g, " ");
}

function safeNumber(value: unknown, fallback: number) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function candidateError(candidate: CandidateDraft, all: CandidateDraft[]) {
  const name = normalizedCandidateName(candidate.name);

  if (!name) return "Укажите ФИО кандидата";
  if (name.length < 2) return "Имя кандидата слишком короткое";
  if (name.length > 200) return "Имя кандидата слишком длинное";
  if (candidate.description.trim().length > 1000) return "Описание слишком длинное";

  const key = name.toLowerCase();
  const duplicates = all.filter(
    (x) => normalizedCandidateName(x.name).toLowerCase() === key && key !== ""
  );
  if (duplicates.length > 1) return "Дубликат имени кандидата";

  return "";
}

function ballotFormatHint(format: "approval" | "ranking" | "score") {
  if (format === "approval") {
    return "Для approval задайте максимальное число допустимых отметок.";
  }
  if (format === "ranking") {
    return "Для ranking можно включить top-k и ограничить только первые позиции.";
  }
  return "Для score задайте диапазон значений и шаг оценки.";
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

function ballotFormatLabel(value: "approval" | "ranking" | "score") {
  if (value === "approval") return "Одобрение";
  if (value === "ranking") return "Ранжирование";
  return "Оценивание";
}

function accessModeLabel(value: "open" | "invite") {
  return value === "open" ? "Открытый доступ" : "Только по приглашениям";
}

function quotaTypeLabel(value: "hare" | "droop") {
  return value === "hare" ? "Хэра" : "Друпа";
}

function submitModeLabel(value: "draft" | "open" | "schedule") {
  if (value === "draft") return "Сохранить как черновик";
  if (value === "open") return "Открыть сразу";
  return "Запланировать открытие";
}

function reviewBallotFormatLabel(value: "approval" | "ranking" | "score") {
  if (value === "approval") return "Одобрение";
  if (value === "ranking") return "Ранжирование";
  return "Оценивание";
}

function reviewQuotaUsageLabel(enabled: boolean, quotaType: "hare" | "droop") {
  if (!enabled) return "Не используется";
  return `Квота ${quotaTypeLabel(quotaType)}`;
}

function reviewRankingModeLabel(limitEnabled: boolean, topK: number) {
  if (!limitEnabled) return "Без ограничения числа позиций";
  if (topK === 1) return "Только первое место";
  return `Не более ${topK} позиций`;
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

function ruleDisplayLabel(rule: TallyRuleInfo | undefined, fallbackId: string) {
  if (rule?.label?.trim()) return rule.label.trim();
  return tallyRuleLabel(rule?.id || fallbackId);
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

  const [availableRules, setAvailableRules] = useState<TallyRuleInfo[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);

  useEffect(() => {
    if (!token) return;

    const ac = new AbortController();
    setRulesLoading(true);

    api.capabilities
      .tallyRules(token, ac.signal)
      .then((items) => {
        const mergedItems = mergeRuleItems(items);
        const electionRules = mergedItems.filter((item) => item.supports_election_tally);
        setAvailableRules(electionRules);

        setTallyRule((prev) => {
          if (electionRules.length === 0) return "";
          if (electionRules.some((item) => item.id === prev)) return prev;
          return electionRules[0].id;
        });
      })
      .catch((e: any) => {
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setErr((prev) => prev || "Не удалось загрузить список правил подсчёта");
      })
      .finally(() => {
        setRulesLoading(false);
      });

    return () => ac.abort();
  }, [token, setToken]);

  const { addNotification } = useNotifications();

  const [step, setStep] = useState(0);

  const [title, setTitle] = useState("Новое голосование");
  const [description, setDescription] = useState("");

  const [startAtLocal, setStartAtLocal] = useState(
    toLocalInputValue(new Date(Date.now() + 10 * 60_000))
  );
  const [endAtLocal, setEndAtLocal] = useState(
    toLocalInputValue(new Date(Date.now() + 70 * 60_000))
  );

  const [candidates, setCandidates] = useState<CandidateDraft[]>([
    { name: "", description: "" },
    { name: "", description: "" },
  ]);

  const [importingCandidates, setImportingCandidates] = useState(false);
  const [importedCandidatesFileName, setImportedCandidatesFileName] = useState("");

  const [ballotFormat, setBallotFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [tallyRule, setTallyRule] = useState("");

  const [committeeSize, setCommitteeSize] = useState<number>(1);
  const [quotaType, setQuotaType] = useState<"hare" | "droop">("hare");

  const [approvalMax, setApprovalMax] = useState<number>(2);

  const [limitRankingTopK, setLimitRankingTopK] = useState(true);
  const [rankingTopKInput, setRankingTopKInput] = useState("3");

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

  const [submitMode, setSubmitMode] = useState<"draft" | "open" | "schedule">("draft");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [createdID, setCreatedID] = useState<string | null>(null);

  const candidateCount = candidates.length;

  useEffect(() => {
    try {
      const raw = localStorage.getItem(CREATE_ELECTION_DRAFT_KEY);
      if (!raw) return;

      const draft = JSON.parse(raw) as Record<string, unknown>;

      if (typeof draft.step === "number" && draft.step >= 0 && draft.step < STEPS.length) {
        setStep(draft.step);
      }

      if (typeof draft.title === "string") setTitle(draft.title);
      if (typeof draft.description === "string") setDescription(draft.description);
      if (typeof draft.startAtLocal === "string") setStartAtLocal(draft.startAtLocal);
      if (typeof draft.endAtLocal === "string") setEndAtLocal(draft.endAtLocal);

      if (Array.isArray(draft.candidates)) {
        const restoredCandidates = draft.candidates
          .filter(
            (item): item is CandidateDraft =>
              Boolean(item) &&
              typeof item === "object" &&
              typeof (item as CandidateDraft).name === "string" &&
              typeof (item as CandidateDraft).description === "string"
          )
          .map((item) => ({
            name: item.name,
            description: item.description,
          }));

        if (restoredCandidates.length >= 2) {
          setCandidates(restoredCandidates);
        }
      }

      if (typeof draft.importedCandidatesFileName === "string") {
        setImportedCandidatesFileName(draft.importedCandidatesFileName);
      }

      if (draft.ballotFormat === "approval" || draft.ballotFormat === "ranking" || draft.ballotFormat === "score") {
        setBallotFormat(draft.ballotFormat);
      }

      if (typeof draft.tallyRule === "string") setTallyRule(draft.tallyRule);
      setCommitteeSize(safeNumber(draft.committeeSize, 1));

      if (draft.quotaType === "hare" || draft.quotaType === "droop") {
        setQuotaType(draft.quotaType);
      }

      setApprovalMax(safeNumber(draft.approvalMax, 2));
      setLimitRankingTopK(Boolean(draft.limitRankingTopK));

      if (typeof draft.rankingTopKInput === "string") {
        setRankingTopKInput(draft.rankingTopKInput);
      }

      setScoreMin(safeNumber(draft.scoreMin, 0));
      setScoreMax(safeNumber(draft.scoreMax, 10));
      setScoreStep(safeNumber(draft.scoreStep, 1));
      setScoreAllowSkip(Boolean(draft.scoreAllowSkip));

      if (draft.accessMode === "open" || draft.accessMode === "invite") {
        setAccessMode(draft.accessMode);
      }

      setShowAggregates(Boolean(draft.showAggregates));
      setDelayPublish(Boolean(draft.delayPublish));

      if (typeof draft.publishAtLocal === "string") {
        setPublishAtLocal(draft.publishAtLocal);
      }

      if (draft.submitMode === "draft" || draft.submitMode === "open" || draft.submitMode === "schedule") {
        setSubmitMode(draft.submitMode);
      }
    } catch {
      localStorage.removeItem(CREATE_ELECTION_DRAFT_KEY);
    }
  }, []);

  useEffect(() => {
    const draft = {
      step,
      title,
      description,
      startAtLocal,
      endAtLocal,
      candidates,
      importedCandidatesFileName,
      ballotFormat,
      tallyRule,
      committeeSize,
      quotaType,
      approvalMax,
      limitRankingTopK,
      rankingTopKInput,
      scoreMin,
      scoreMax,
      scoreStep,
      scoreAllowSkip,
      accessMode,
      showAggregates,
      delayPublish,
      publishAtLocal,
      submitMode,
    };

    localStorage.setItem(CREATE_ELECTION_DRAFT_KEY, JSON.stringify(draft));
  }, [
    step,
    title,
    description,
    startAtLocal,
    endAtLocal,
    candidates,
    importedCandidatesFileName,
    ballotFormat,
    tallyRule,
    committeeSize,
    quotaType,
    approvalMax,
    limitRankingTopK,
    rankingTopKInput,
    scoreMin,
    scoreMax,
    scoreStep,
    scoreAllowSkip,
    accessMode,
    showAggregates,
    delayPublish,
    publishAtLocal,
    submitMode,
  ]);

  const currentRule = useMemo(
    () => selectedRuleInfo(availableRules, tallyRule),
    [availableRules, tallyRule]
  );

  const allowedBallotFormats = useMemo(() => {
    const formats = new Set<"approval" | "ranking" | "score">();

    for (const rule of availableRules) {
      for (const format of rule.ballot_formats ?? []) {
        if (format === "approval" || format === "ranking" || format === "score") {
          formats.add(format);
        }
      }
    }

    return Array.from(formats);
  }, [availableRules]);

  const rulesForSelectedBallotFormat = useMemo(
    () => availableRules.filter((rule) => supportsBallotFormat(rule, ballotFormat)),
    [availableRules, ballotFormat]
  );

  useEffect(() => {
    if (allowedBallotFormats.length === 0) return;
    if (!allowedBallotFormats.includes(ballotFormat)) {
      setBallotFormat(allowedBallotFormats[0]);
    }
  }, [allowedBallotFormats, ballotFormat]);

  useEffect(() => {
    if (rulesForSelectedBallotFormat.length === 0) return;
    if (!rulesForSelectedBallotFormat.some((rule) => rule.id === tallyRule)) {
      setTallyRule(rulesForSelectedBallotFormat[0].id);
    }
  }, [rulesForSelectedBallotFormat, tallyRule]);

  useEffect(() => {
    if (!currentRule) return;

    if (!currentRule.requires_committee_size && committeeSize !== 1) {
      setCommitteeSize(1);
    }

    if (!currentRule.supports_quota_type && committeeSize > 1) {
      setQuotaType("hare");
    }

    if (!currentRule.requires_approval_max_choices && ballotFormat === "approval") {
      setApprovalMax(1);
    }

    if (!currentRule.supports_ranking_top_k && ballotFormat === "ranking") {
      setLimitRankingTopK(false);
      setRankingTopKInput("1");
    }

    if (!currentRule.requires_score_range && ballotFormat === "score") {
      setScoreMin(0);
      setScoreMax(10);
      setScoreStep(1);
      setScoreAllowSkip(false);
    }
  }, [currentRule, ballotFormat, committeeSize]);

  const candidateErrors = useMemo(
    () => candidates.map((candidate) => candidateError(candidate, candidates)),
    [candidates]
  );

  const candidatesValid =
    candidates.length >= 2 && candidateErrors.every((x) => x === "");

  const candidatePayload = useMemo<CandidatePayload[]>(
    () =>
      candidates.map((candidate) => {
        const name = normalizedCandidateName(candidate.name);
        const desc = candidate.description.trim();

        return {
          name,
          meta: desc ? { description: desc } : undefined,
        };
      }),
    [candidates]
  );

  const startAtRFC3339 = useMemo(() => toRFC3339FromLocalInput(startAtLocal), [startAtLocal]);
  const endAtRFC3339 = useMemo(() => toRFC3339FromLocalInput(endAtLocal), [endAtLocal]);
  const publishAtRFC3339 = useMemo(
    () => (delayPublish ? toRFC3339FromLocalInput(publishAtLocal) : ""),
    [delayPublish, publishAtLocal]
  );

  const normalizedTopK = () => {
    const raw = Number(rankingTopKInput);
    if (!Number.isFinite(raw) || raw < 1) return 1;
    if (raw > candidateCount) return candidateCount;
    return Math.floor(raw);
  };

  const updateCandidate = (index: number, patch: Partial<CandidateDraft>) => {
    setCandidates((prev) =>
      prev.map((item, i) => (i === index ? { ...item, ...patch } : item))
    );
  };

  const addCandidate = () => {
    setCandidates((prev) => [...prev, { name: "", description: "" }]);
  };

  const removeCandidate = (index: number) => {
    setCandidates((prev) => {
      const next = prev.filter((_, i) => i !== index);
      return next.length >= 2 ? next : prev;
    });
  };

  const handleCandidatesFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] ?? null;
    e.target.value = "";

    if (!file || !token) return;

    setImportingCandidates(true);
    setErr(null);

    try {
      const imported = await api.elections.importCandidates(token, file);

      if (imported.length < 2) {
        setErr("Файл должен содержать не менее двух кандидатов");
        return;
      }

      setCandidates(imported);
      setImportedCandidatesFileName(file.name);

      addNotification({
        kind: "success",
        title: "Кандидаты импортированы",
        message: `Загружено кандидатов: ${imported.length}`,
      });
    } catch (err: any) {
      if (err?.status === 401) {
        setToken(null);
        return;
      }
      setErr(err?.message || "Не удалось импортировать кандидатов");
    } finally {
      setImportingCandidates(false);
    }
  };

  const validateStep = (targetStep: number) => {
    if (targetStep >= 0) {
      if (!title.trim()) return "Введите название голосования";
      if (!startAtRFC3339) return "Укажите корректную дату и время начала";
      if (!endAtRFC3339) return "Укажите корректную дату и время окончания";

      const nowTs = Date.now();
      const startTs = new Date(startAtRFC3339).getTime();
      const endTs = new Date(endAtRFC3339).getTime();

      if (startTs >= endTs) {
        return "Дата начала должна быть раньше даты окончания";
      }
      if (endTs <= nowTs) {
        return "Дата окончания должна быть позже текущего времени";
      }
    }

    if (targetStep >= 3) {
      if (submitMode !== "open") {
        const startTs = new Date(startAtRFC3339).getTime();
        if (startTs <= Date.now()) {
          return "Для черновика или запланированного старта дата начала должна быть позже текущего времени";
        }
      }

      if (delayPublish) {
        if (!publishAtRFC3339) {
          return "Укажите корректную дату и время публикации";
        }
        const publishTs = new Date(publishAtRFC3339).getTime();
        const endTs = new Date(endAtRFC3339).getTime();
        if (publishTs <= endTs) {
          return "Дата публикации результатов должна быть позже окончания голосования";
        }
      }
    }

    if (targetStep >= 1) {
      if (!candidatesValid) {
        return candidateErrors.find(Boolean) || "Проверьте список кандидатов";
      }
    }

    if (targetStep >= 2) {
      if (rulesLoading && availableRules.length === 0) {
        return "Список правил подсчёта еще загружается";
      }
      if (availableRules.length === 0) {
        return "Нет доступных правил подсчёта";
      }
      if (rulesForSelectedBallotFormat.length === 0) {
        return "Для выбранного формата бюллетеня нет доступных правил подсчёта";
      }
      if (!currentRule) {
        return "Выберите допустимое правило подсчёта";
      }
      if (!currentRule.supports_election_tally) {
        return "Выбранное правило недоступно для подсчёта голосования";
      }
      if (!supportsBallotFormat(currentRule, ballotFormat)) {
        return "Выбранное правило не поддерживает этот формат бюллетеня";
      }

      if (currentRule.requires_committee_size && committeeSize < 1) {
        return "Размер комитета должен быть не меньше 1";
      }

      if (ballotFormat === "approval" && currentRule.requires_approval_max_choices) {
        if (approvalMax < 1) return "approval_max_choices должен быть не меньше 1";
        if (approvalMax > candidateCount) {
          return "approval_max_choices не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "ranking" && currentRule.supports_ranking_top_k && limitRankingTopK) {
        const topK = normalizedTopK();
        if (topK < 1) return "ranking_top_k должен быть не меньше 1";
        if (topK > candidateCount) {
          return "ranking_top_k не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "score" && currentRule.requires_score_range) {
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

  const resetDraft = () => {
    localStorage.removeItem(CREATE_ELECTION_DRAFT_KEY);

    setStep(0);
    setTitle("Новое голосование");
    setDescription("");
    setStartAtLocal(toLocalInputValue(new Date(Date.now() + 10 * 60_000)));
    setEndAtLocal(toLocalInputValue(new Date(Date.now() + 70 * 60_000)));
    setCandidates([
      { name: "", description: "" },
      { name: "", description: "" },
    ]);
    setImportedCandidatesFileName("");
    setBallotFormat("ranking");
    const defaultRankingRule =
      availableRules.find((rule) => supportsBallotFormat(rule, "ranking"))?.id ||
      availableRules[0]?.id ||
      "";

    setTallyRule(defaultRankingRule);
    setCommitteeSize(1);
    setQuotaType("hare");
    setApprovalMax(2);
    setLimitRankingTopK(true);
    setRankingTopKInput("3");
    setScoreMin(0);
    setScoreMax(10);
    setScoreStep(1);
    setScoreAllowSkip(false);
    setAccessMode("open");
    setShowAggregates(true);
    setDelayPublish(false);
    setPublishAtLocal(toLocalInputValue(new Date(Date.now() + 24 * 60 * 60_000)));
    setSubmitMode("draft");
    setCreatedID(null);
    setErr(null);
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

    try {
      const effectiveStartAtRFC3339 =
        submitMode === "open" ? new Date().toISOString() : startAtRFC3339;
      const body: Record<string, unknown> = {
        title: title.trim(),
        description: description.trim() ? description.trim() : null,
        start_at: effectiveStartAtRFC3339,
        end_at: endAtRFC3339,
        tally_rule: tallyRule,
        ballot_format: ballotFormat,
        committee_size: committeeSize,
        quota_type: committeeSize > 1 ? quotaType : null,
        access_mode: accessMode,
        publish_at: delayPublish ? publishAtRFC3339 : null,
        show_aggregates: showAggregates,
        candidates: candidatePayload,
      };
      
      if (!currentRule?.requires_committee_size) {
        body.committee_size = null;
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
        body.approval_max_choices = approvalMax;
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

      const id = await api.elections.create(token, body);

      if (submitMode === "open") {
        await api.elections.action(token, id, "open");
      }
      if (submitMode === "schedule") {
        await api.elections.action(token, id, "schedule");
      }

      setCreatedID(id);
      localStorage.removeItem(CREATE_ELECTION_DRAFT_KEY);

      addNotification({
        kind: "success",
        title: "Голосование создано",
        message:
          submitMode === "open"
            ? `Создано и открыто голосование ${id}`
            : submitMode === "schedule"
            ? `Создано и запланировано голосование ${id}`
            : `Создано новое голосование ${id}`,
      });

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
              <button style={styles.btn}>К дашборду</button>
            </Link>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>К списку</button>
            </Link>
            <button type="button" style={styles.btn} onClick={resetDraft} disabled={loading}>
              Очистить черновик
            </button>
          </div>
        </div>

        <div style={{ marginTop: 8, ...styles.muted }}>
          Пошаговый мастер создания голосования с валидацией ключевых параметров.
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

              <DateTimeField
                label={
                  <>
                    Дата и время начала
                    <Hint text="Для режима «Открыть сразу» время начала будет заменено текущим моментом." />
                  </>
                }
                value={startAtLocal}
                onChange={setStartAtLocal}
                minuteStep={5}
              />

              <DateTimeField
                label={
                  <>
                    Дата и время окончания
                    <Hint text="После этой даты новые бюллетени приниматься не должны." />
                  </>
                }
                value={endAtLocal}
                onChange={setEndAtLocal}
                minuteStep={5}
              />
            </div>
          </div>
        ) : null}

        {step === 1 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.muted}>
              Добавьте не менее двух кандидатов. Для каждого можно указать краткое описание.
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <div style={{ fontWeight: 600, marginBottom: 8 }}>
                Импорт кандидатов из файла
              </div>

              <div style={{ ...styles.muted, marginBottom: 12 }}>
                Поддерживаются CSV и JSON. CSV: колонки name и description.
                JSON: массив строк или массив объектов с полями name и description.
              </div>

              <input
                type="file"
                accept=".csv,.json,application/json,text/csv"
                onChange={handleCandidatesFileChange}
                disabled={importingCandidates || loading}
              />

              {importingCandidates ? (
                <div style={{ marginTop: 8, fontSize: 13, color: "#667085" }}>
                  Импорт кандидатов...
                </div>
              ) : null}

              {importedCandidatesFileName ? (
                <div style={{ marginTop: 8, fontSize: 13, color: "#667085" }}>
                  Последний импорт: {importedCandidatesFileName}
                </div>
              ) : null}
            </div>

            <div style={{ display: "grid", gap: 16 }}>
              {candidates.map((candidate, index) => (
                <div
                  key={index}
                  style={{
                    border: "1px solid #d0d7de",
                    borderRadius: 12,
                    padding: 16,
                    background: "#fff",
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 12 }}>
                    <strong>Кандидат #{index + 1}</strong>
                    <button
                      type="button"
                      onClick={() => removeCandidate(index)}
                      disabled={candidates.length <= 2}
                    >
                      Удалить
                    </button>
                  </div>

                  <label style={{ display: "block", marginBottom: 12 }}>
                    <div style={{ marginBottom: 6 }}>ФИО</div>
                    <input
                      style={styles.input}
                      maxLength={200}
                      value={candidate.name}
                      onChange={(e) => updateCandidate(index, { name: e.target.value })}
                      onBlur={(e) =>
                        updateCandidate(index, { name: normalizedCandidateName(e.target.value) })
                      }
                      placeholder="Иванов Иван Иванович"
                    />
                  </label>

                  <label style={{ display: "block", marginBottom: 8 }}>
                    <div style={{ marginBottom: 6 }}>Описание</div>
                    <textarea
                      style={{ ...styles.input, minHeight: 90 }}
                      maxLength={1000}
                      value={candidate.description}
                      onChange={(e) => updateCandidate(index, { description: e.target.value })}
                      placeholder="Краткая информация о кандидате"
                    />
                    <div style={{ marginTop: 6, fontSize: 12, color: "#667085" }}>
                      {candidate.description.trim().length}/1000
                    </div>
                  </label>

                  {candidateErrors[index] ? (
                    <div style={{ color: "#b42318", fontSize: 14 }}>{candidateErrors[index]}</div>
                  ) : null}
                </div>
              ))}
            </div>

            <div>
              <button type="button" style={styles.btn} onClick={addCandidate}>
                + Добавить кандидата
              </button>
            </div>
          </div>
        ) : null}

        {step === 2 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={styles.grid2}>
              <div>
                <label>
                  Формат бюллетеня
                  <Hint text="Тип данных, которые будет вводить избиратель: approval, ranking или score." />
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
                      disabled={!allowedBallotFormats.includes(format)}
                    >
                      {format}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label>
                  Правило подсчёта
                  <Hint text="Алгоритм определения победителей на основе бюллетеней." />
                </label>
                <select
                  style={styles.input}
                  value={tallyRule}
                  onChange={(e) => setTallyRule(e.target.value)}
                >
                  {rulesForSelectedBallotFormat.map((rule) => (
                    <option key={rule.id} value={rule.id}>
                      {ruleDisplayLabel(rule, rule.id)}
                    </option>
                  ))}
                </select>
                {rulesLoading ? (
                  <div style={{ marginTop: 6, fontSize: 13, color: "#667085" }}>
                    Загрузка списка правил…
                  </div>
                ) : null}
              </div>

              <div>
                <label>
                  Размер комитета
                  <Hint text="Количество победителей. Для части правил параметр обязателен." />
                </label>
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
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              {ballotFormatHint(ballotFormat)}
            </div>

            {ballotFormat === "approval" && currentRule?.requires_approval_max_choices ? (
              <div style={styles.grid2}>
                <div>
                  <label>Максимум отметок</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={Math.max(candidateCount, 1)}
                    value={approvalMax}
                    onChange={(e) => setApprovalMax(Number(e.target.value))}
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
                    Максимально допустимое значение: {candidateCount}
                  </div>
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
                <label>
                  Режим доступа
                  <Hint text="open — доступно всем, invite — только приглашённым пользователям." />
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

              <div style={{ display: "flex", alignItems: "center" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={showAggregates}
                    onChange={(e) => setShowAggregates(e.target.checked)}
                  />
                  Показывать агрегированные данные в результатах
                  <Hint text="Определяет, будут ли в опубликованных результатах показаны агрегированные метрики и сводные значения." />
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
                  <Hint text="Если включено, результаты не будут опубликованы сразу после завершения голосования." />
                </label>
              </div>

              {delayPublish ? (
                <div style={{ marginTop: 12, maxWidth: 420 }}>
                  <DateTimeField
                    label="Дата и время публикации"
                    value={publishAtLocal}
                    onChange={setPublishAtLocal}
                    minuteStep={5}
                  />
                </div>
              ) : (
                <div style={{ marginTop: 12, ...styles.muted }}>
                  Результаты можно будет публиковать вручную после завершения голосования.
                </div>
              )}
            </div>

            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <h3>Что сделать после сохранения</h3>

              <div style={{ display: "grid", gap: 10, marginTop: 12 }}>
                <label>
                  <input
                    type="radio"
                    checked={submitMode === "draft"}
                    onChange={() => setSubmitMode("draft")}
                  />
                  <span style={{ marginLeft: 8 }}>Сохранить как черновик</span>
                </label>

                <label>
                  <input
                    type="radio"
                    checked={submitMode === "open"}
                    onChange={() => setSubmitMode("open")}
                  />
                  <span style={{ marginLeft: 8 }}>Открыть сразу</span>
                </label>

                <label>
                  <input
                    type="radio"
                    checked={submitMode === "schedule"}
                    onChange={() => setSubmitMode("schedule")}
                  />
                  <span style={{ marginLeft: 8 }}>Запланировать открытие</span>
                </label>
              </div>

              {submitMode === "open" ? (
                <div
                  style={{
                    marginTop: 12,
                    padding: 12,
                    borderRadius: 10,
                    background: "#fff7e6",
                    border: "1px solid #f7b955",
                    color: "#8a4b00",
                    fontWeight: 500,
                  }}
                >
                  Голосование будет открыто сразу после создания.
                  Время начала будет зафиксировано текущим моментом.
                  Участники смогут начать голосование немедленно.
                </div>
              ) : null}

              {submitMode === "schedule" ? (
                <div
                  style={{
                    marginTop: 12,
                    padding: 12,
                    borderRadius: 10,
                    background: "#eff8ff",
                    border: "1px solid #84caff",
                    color: "#175cd3",
                    fontWeight: 500,
                  }}
                >
                   Голосование будет создано в статусе scheduled.
                   Оно откроется по запланированному жизненному циклу, а не сразу.
                </div>
              ) : null}

              {submitMode === "draft" ? (
                <div
                  style={{
                    marginTop: 12,
                    padding: 12,
                    borderRadius: 10,
                    background: "#f8fafc",
                    border: "1px solid #cbd5e1",
                    color: "#334155",
                    fontWeight: 500,
                  }}
                >
                  Голосование сохранится как черновик.
                  Участники не увидят его и не смогут голосовать, пока вы не выполните schedule или open.
                </div>
              ) : null}

            </div>
          </div>
        ) : null}

        {step === 4 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <SummaryGrid
              items={[
                { label: "Название", value: title.trim() || "—" },
                { label: "Описание", value: description.trim() || "—" },
                { label: "Формат бюллетеня", value: reviewBallotFormatLabel(ballotFormat) },
                { label: "Правило подсчёта", value: ruleDisplayLabel(currentRule, tallyRule) },
                {
                  label: "Размер комитета",
                  value: currentRule?.requires_committee_size ? String(committeeSize) : "Один победитель",
                },
                {
                  label: "Квота",
                  value: reviewQuotaUsageLabel(
                    Boolean(committeeSize > 1 && currentRule?.supports_quota_type),
                    quotaType
                  ),
                },
                { label: "Режим доступа", value: accessModeLabel(accessMode) },
                { label: "Показывать агрегаты", value: showAggregates ? "Да" : "Нет" },
                {
                  label: "Начало голосования",
                  value: submitMode === "open" ? "Сразу после создания" : startAtRFC3339 || "—",
                },
                { label: "Окончание голосования", value: endAtRFC3339 || "—" },
                {
                  label: "Публикация результатов",
                  value: delayPublish ? publishAtRFC3339 || "—" : "Сразу после готовности и ручной публикации",
                },
                { label: "Число кандидатов", value: String(candidateCount) },
                { label: "После сохранения", value: submitModeLabel(submitMode) },
              ]}
            />

            <div style={styles.card}>
              <h3 style={{ marginTop: 0 }}>Кандидаты</h3>
              <div style={{ display: "grid", gap: 10 }}>
                {candidatePayload.map((candidate, index) => (
                  <div key={`${candidate.name}-${index}`}>
                    <div>
                      {index + 1}. {candidate.name}
                    </div>
                    {candidate.meta?.description ? (
                      <div style={{ marginTop: 4, ...styles.muted }}>{candidate.meta.description}</div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>

            <div style={styles.card}>
              <h3 style={{ marginTop: 0 }}>Параметры бюллетеня</h3>

              {ballotFormat === "approval" ? (
                <div style={{ display: "grid", gap: 6 }}>
                  <div>Максимальное число отметок: {approvalMax}</div>
                </div>
              ) : null}

              {ballotFormat === "ranking" ? (
                <div style={{ display: "grid", gap: 6 }}>
                  <div>
                    Режим ранжирования:{" "}
                    {reviewRankingModeLabel(
                      Boolean(currentRule?.supports_ranking_top_k && limitRankingTopK),
                      normalizedTopK()
                    )}
                  </div>
                </div>
              ) : null}

              {ballotFormat === "score" ? (
                <div style={{ display: "grid", gap: 6 }}>
                  <div>
                    Диапазон оценок: {scoreMin}..{scoreMax}
                  </div>
                  <div>Шаг изменения оценки: {scoreStep}</div>
                  <div>Разрешить пропуск оценки: {scoreAllowSkip ? "Да" : "Нет"}</div>
                </div>
              ) : null}
            </div>

            {createdID ? (
              <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                <div style={{ fontWeight: 700 }}>Голосование создано</div>
                <div style={{ marginTop: 6 }}>ID: {createdID}</div>

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btnPrimary} onClick={() => nav(`/admin/elections/${createdID}`)}>
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
    </div>
  );
}