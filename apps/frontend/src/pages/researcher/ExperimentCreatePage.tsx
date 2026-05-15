import React, { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { styles } from "../../shared/ui/styles";
import type {
  DatasetDetail,
  DatasetListItem,
  ElectionSummary,
  TallyRuleInfo,
} from "../../shared/api/types";
import { mergeRuleItems } from "../../shared/utils/mergeRuleItems";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

const STEPS = [
  "Основные параметры",
  "Параметры бюллетеня",
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

type ExperimentSourceKind = "dataset" | "published_election";

function shortId(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
}

function sourceKindLabel(value: ExperimentSourceKind) {
  if (value === "dataset") return "Существующий датасет";
  return "Опубликованное голосование";
}

function datasetOptionLabel(item: DatasetListItem) {
  return `${item.name} · ${item.format} · ${shortId(item.id)}`;
}

function electionOptionLabel(item: ElectionSummary) {
  const format = item.ballot_format || "формат не указан";
  const count =
    typeof item.candidate_count === "number"
      ? `${item.candidate_count} кандидатов`
      : "число кандидатов не указано";

  return `${item.title} · ${format} · ${count} · ${shortId(item.id)}`;
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
        gridTemplateColumns: "repeat(3, 1fr)",
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

function supportsBallotFormat(
  rule: TallyRuleInfo | undefined,
  format: "approval" | "ranking" | "score"
) {
  return Boolean(rule?.ballot_formats?.includes(format));
}

function selectedRuleInfo(rules: TallyRuleInfo[], ruleId: string) {
  return rules.find((rule) => rule.id === ruleId);
}

function ruleDisplayLabel(rule: TallyRuleInfo | undefined, fallbackId: string) {
  if (rule?.label?.trim()) return rule.label.trim();
  return tallyRuleLabel(rule?.id || fallbackId);
}

function quotaTypeDescription(value: "hare" | "droop") {
  if (value === "hare") {
    return "Квота Хэра: базовый порог, рассчитываемый как отношение числа голосов к размеру комитета.";
  }

  return "Квота Друпа: более строгий порог избрания.";
}

function quotaAvailabilityText(rule: TallyRuleInfo | undefined) {
  if (!rule) {
    return "Сначала выберите правило подсчёта.";
  }

  if (!rule.supports_quota_type) {
    return "Выбранное правило не поддерживает выбор квоты.";
  }

  const formats = rule.ballot_formats.join(", ");
  return `Квота доступна для выбранного правила. Поддерживаемые форматы бюллетеня: ${formats}.`;
}

export function ExperimentCreatePage() {
  const nav = useNavigate();
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [step, setStep] = useState(0);

  const [sourceKind, setSourceKind] = useState<ExperimentSourceKind>("dataset");

  const [datasets, setDatasets] = useState<DatasetListItem[]>([]);
  const [datasetsLoading, setDatasetsLoading] = useState(false);
  const [selectedDatasetId, setSelectedDatasetId] = useState("");
  const [selectedDatasetDetail, setSelectedDatasetDetail] = useState<DatasetDetail | null>(null);

  const [publishedElections, setPublishedElections] = useState<ElectionSummary[]>([]);
  const [electionsLoading, setElectionsLoading] = useState(false);
  const [selectedElectionId, setSelectedElectionId] = useState("");

  const [createdRunIds, setCreatedRunIds] = useState<string[]>([]);
  const [createdJobIds, setCreatedJobIds] = useState<string[]>([]);

  const [type, setType] = useState<"algo" | "behavior">("algo");
  const [ballotFormat, setBallotFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [tallyRule, setTallyRule] = useState("");
  const [selectedRuleIds, setSelectedRuleIds] = useState<string[]>([]);

  const [candidates, setCandidates] = useState(5);
  const [voters, setVoters] = useState(100);
  const [committeeSize, setCommitteeSize] = useState(1);
  const [quotaEnabled, setQuotaEnabled] = useState(false);
  const [quotaType, setQuotaType] = useState<"hare" | "droop">("hare");

  const [approvalMax, setApprovalMax] = useState(2);
  const [rankingTopK, setRankingTopK] = useState(3);
  const [rankingTopKEnabled, setRankingTopKEnabled] = useState(false);

  const [scoreMin, setScoreMin] = useState(0);
  const [scoreMax, setScoreMax] = useState(10);
  const [scoreStep, setScoreStep] = useState(1);

  const [seed, setSeed] = useState("");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<unknown>(null);

  const [availableRules, setAvailableRules] = useState<TallyRuleInfo[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);

  const currentRule = useMemo(
    () => selectedRuleInfo(availableRules, tallyRule),
    [availableRules, tallyRule]
  );

  const selectedPublishedElection = useMemo(
    () => publishedElections.find((item) => item.id === selectedElectionId),
    [publishedElections, selectedElectionId]
  );

  const selectedDatasetListItem = useMemo(
    () => datasets.find((item) => item.id === selectedDatasetId),
    [datasets, selectedDatasetId]
  );

  const selectedRuleInfos = useMemo(
    () =>
      selectedRuleIds
        .map((ruleId) => selectedRuleInfo(availableRules, ruleId))
        .filter((rule): rule is TallyRuleInfo => Boolean(rule)),
    [availableRules, selectedRuleIds]
  );

  const selectedRulesNeedApprovalMax = selectedRuleInfos.some(
    (rule) => rule.requires_approval_max_choices
  );

  const selectedRulesSupportRankingTopK = selectedRuleInfos.some(
    (rule) => rule.supports_ranking_top_k
  );

  const selectedRulesNeedScoreRange = selectedRuleInfos.some(
    (rule) => rule.requires_score_range
  );

  const selectedRulesSupportQuota = selectedRuleInfos.some(
    (rule) => rule.supports_quota_type
  );

  const selectedRulesNeedCommitteeSize = selectedRuleInfos.some(
    (rule) => rule.requires_committee_size
  );

  const maxCommitteeSize = Math.max(1, candidates);
  const quotaSupported = selectedRulesSupportQuota;

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
    if (rulesForSelectedBallotFormat.length === 0) {
      setSelectedRuleIds([]);
      setTallyRule("");
      return;
    }

    setSelectedRuleIds((prev) => {
      const allowed = new Set(rulesForSelectedBallotFormat.map((rule) => rule.id));
      const next = prev.filter((ruleId) => allowed.has(ruleId));

      if (next.length > 0) {
        setTallyRule(next[0]);
        return next;
      }

      const first = rulesForSelectedBallotFormat[0].id;
      setTallyRule(first);
      return [first];
    });
  }, [rulesForSelectedBallotFormat]);

  useEffect(() => {
    if (!token) return;

    const ac = new AbortController();
    setRulesLoading(true);

    api.capabilities
      .tallyRules(token, ac.signal)
      .then((items) => {
        const mergedItems = mergeRuleItems(items);
        const experimentRules = mergedItems.filter((item) => item.supports_experiment_runs);
        setAvailableRules(experimentRules);

        setTallyRule((prev) => {
          if (experimentRules.length === 0) return "";
          if (experimentRules.some((item) => item.id === prev)) return prev;
          return experimentRules[0].id;
        });
      })
      .catch((e: any) => {
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setErr((prev) => prev || "Не удалось загрузить список правил");
      })
      .finally(() => {
        setRulesLoading(false);
      });

    return () => ac.abort();
  }, [token, setToken]);

  useEffect(() => {
    if (!token) return;

    const ac = new AbortController();
    setDatasetsLoading(true);

    api.datasets
      .list(token, ac.signal)
      .then((items) => {
        setDatasets(items);
        setSelectedDatasetId((prev) => prev || items[0]?.id || "");
      })
      .catch((e: any) => {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setErr((prev) => prev || "Не удалось загрузить список датасетов");
        setDatasets([]);
      })
      .finally(() => {
        setDatasetsLoading(false);
      });

    return () => ac.abort();
  }, [token, setToken]);

  useEffect(() => {
    if (!token) return;

    const ac = new AbortController();
    setElectionsLoading(true);

    api.elections
      .list(token, ac.signal)
      .then((items) => {
        const published = items.filter((item) => String(item.status || "") === "published");
        setPublishedElections(published);
        setSelectedElectionId((prev) => prev || published[0]?.id || "");
      })
      .catch((e: any) => {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setPublishedElections([]);
      })
      .finally(() => {
        setElectionsLoading(false);
      });

    return () => ac.abort();
  }, [token, setToken]);

  useEffect(() => {
    if (!token) return;
    if (sourceKind !== "dataset") return;

    const datasetId = selectedDatasetId.trim();
    if (!datasetId) {
      setSelectedDatasetDetail(null);
      return;
    }

    const ac = new AbortController();

    api.datasets
      .get(token, datasetId, ac.signal)
      .then((dataset) => {
        setSelectedDatasetDetail(dataset);

        if (
          dataset.format === "approval" ||
          dataset.format === "ranking" ||
          dataset.format === "score"
        ) {
          setBallotFormat(dataset.format);
        }

        const candidateCount = Math.max(2, dataset.candidates.length);
        setCandidates(candidateCount);
        setCommitteeSize((prev) => Math.max(1, Math.min(prev, candidateCount)));

        const params = dataset.parameters || {};

        if (typeof params.voters === "number" && Number.isFinite(params.voters)) {
          setVoters(Math.max(1, params.voters));
        }

        if (typeof params.approval_max_choices === "number") {
          setApprovalMax(Math.max(1, Math.min(candidateCount, params.approval_max_choices)));
        }

        if (typeof params.ranking_top_k === "number") {
          setRankingTopKEnabled(true);
          setRankingTopK(Math.max(1, Math.min(candidateCount, params.ranking_top_k)));
        } else {
          setRankingTopKEnabled(false);
        }

        if (typeof params.score_min === "number") {
          setScoreMin(params.score_min);
        }

        if (typeof params.score_max === "number") {
          setScoreMax(params.score_max);
        }

        if (typeof params.score_step === "number") {
          setScoreStep(params.score_step);
        }
      })
      .catch((e: any) => {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setSelectedDatasetDetail(null);
      });

    return () => ac.abort();
  }, [token, setToken, sourceKind, selectedDatasetId]);

  useEffect(() => {
    if (sourceKind !== "published_election") return;
    if (!selectedPublishedElection) return;

    if (
      selectedPublishedElection.ballot_format === "approval" ||
      selectedPublishedElection.ballot_format === "ranking" ||
      selectedPublishedElection.ballot_format === "score"
    ) {
      setBallotFormat(selectedPublishedElection.ballot_format);
    }

    if (
      typeof selectedPublishedElection.candidate_count === "number" &&
      selectedPublishedElection.candidate_count >= 2
    ) {
      const candidateCount = selectedPublishedElection.candidate_count;
      setCandidates(candidateCount);
      setCommitteeSize((prev) => Math.max(1, Math.min(prev, candidateCount)));
    }
  }, [sourceKind, selectedPublishedElection]);

  useEffect(() => {
    if (allowedBallotFormats.length === 0) return;
    if (!allowedBallotFormats.includes(ballotFormat)) {
      setBallotFormat(allowedBallotFormats[0]);
    }
  }, [allowedBallotFormats, ballotFormat]);

  useEffect(() => {
    if (selectedRuleIds.length === 0) return;

    if (!selectedRulesSupportRankingTopK && ballotFormat === "ranking") {
      setRankingTopK(1);
      setRankingTopKEnabled(false);
    }

    if (!selectedRulesNeedApprovalMax && ballotFormat === "approval") {
      setApprovalMax(1);
    }

    if (!selectedRulesNeedScoreRange && ballotFormat === "score") {
      setScoreMin(0);
      setScoreMax(10);
      setScoreStep(1);
    }

    if (!selectedRulesSupportQuota && quotaEnabled) {
      setQuotaEnabled(false);
    }

    if (committeeSize > maxCommitteeSize) {
      setCommitteeSize(maxCommitteeSize);
    }
  }, [
    selectedRuleIds.length,
    selectedRulesSupportRankingTopK,
    selectedRulesNeedApprovalMax,
    selectedRulesNeedScoreRange,
    selectedRulesSupportQuota,
    ballotFormat,
    committeeSize,
    maxCommitteeSize,
    quotaEnabled,
  ]);

  const structuredParams = useMemo(() => {
    const params: Record<string, unknown> = {
      ballot_format: ballotFormat,
      tally_rules: selectedRuleIds,
      candidates,
      voters,
      committee_size: committeeSize,
    };

    if (quotaEnabled && selectedRulesSupportQuota) {
      params.quota_type = quotaType;
    }

    if (ballotFormat === "approval" && selectedRulesNeedApprovalMax) {
      params.approval_max_choices = approvalMax;
    }

    if (ballotFormat === "ranking" && selectedRulesSupportRankingTopK && rankingTopKEnabled) {
      params.ranking_top_k = rankingTopK;
    }

    if (ballotFormat === "score" && selectedRulesNeedScoreRange) {
      params.score_min = scoreMin;
      params.score_max = scoreMax;
      params.score_step = scoreStep;
    }

    return params;
  }, [
    ballotFormat,
    selectedRuleIds,
    candidates,
    voters,
    committeeSize,
    quotaEnabled,
    quotaType,
    approvalMax,
    rankingTopK,
    rankingTopKEnabled,
    scoreMin,
    scoreMax,
    scoreStep,
    selectedRulesSupportQuota,
    selectedRulesNeedApprovalMax,
    selectedRulesSupportRankingTopK,
    selectedRulesNeedScoreRange,
  ]);

  const finalParams = structuredParams;

  const validateStep = (targetStep: number) => {
    if (targetStep >= 0) {
      if (sourceKind === "dataset") {
        if (datasetsLoading) return "Список датасетов еще загружается";
        if (datasets.length === 0) return "Нет доступных датасетов";
        if (!selectedDatasetId.trim()) return "Выберите датасет из списка";
      }

      if (sourceKind === "published_election") {
        if (electionsLoading) return "Список опубликованных голосований еще загружается";
        if (publishedElections.length === 0) return "Нет опубликованных голосований";
        if (!selectedElectionId.trim()) return "Выберите опубликованное голосование из списка";
      }

      if (rulesLoading && availableRules.length === 0) {
        return "Список правил еще загружается";
      }
      if (availableRules.length === 0) {
        return "Нет доступных правил для экспериментальных запусков";
      }
      if (!type.trim()) return "Выберите тип эксперимента";
      if (selectedRuleIds.length === 0) return "Выберите хотя бы одно правило подсчёта";
      if (rulesForSelectedBallotFormat.length === 0) {
        return "Для выбранного формата бюллетеня нет доступных правил для экспериментальных запусков";
      }
      if (selectedRuleInfos.length !== selectedRuleIds.length) {
        return "Среди выбранных правил есть недопустимое правило подсчёта";
      }

      if (selectedRuleInfos.some((rule) => !rule.supports_experiment_runs)) {
        return "Среди выбранных правил есть правило, недоступное для экспериментальных запусков";
      }

      if (selectedRuleInfos.some((rule) => !supportsBallotFormat(rule, ballotFormat))) {
        return "Среди выбранных правил есть правило, не поддерживающее выбранный формат бюллетеня";
      }
      if (candidates < 2) return "Количество кандидатов должно быть не меньше 2";
      if (voters < 1) return "Количество избирателей должно быть не меньше 1";
      if (selectedRulesNeedCommitteeSize && committeeSize < 1) {
        return "Размер комитета должен быть не меньше 1";
      }
    }

    if (targetStep >= 1) {
      if (ballotFormat === "approval" && selectedRulesNeedApprovalMax) {
        if (approvalMax < 1) return "approval_max_choices должен быть не меньше 1";
        if (approvalMax > candidates) {
          return "approval_max_choices не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "ranking" && selectedRulesSupportRankingTopK && rankingTopKEnabled) {
        if (rankingTopK < 1) return "Ограничение top-k должно быть не меньше 1";
        if (rankingTopK > candidates) {
          return "Ограничение top-k не может превышать число кандидатов";
        }
      }

      if (ballotFormat === "score" && selectedRulesNeedScoreRange) {
        if (scoreStep <= 0) return "Шаг оценки должен быть больше 0";
        if (scoreMin > scoreMax) {
          return "Нижняя граница оценки не может быть больше верхней";
        }
        if ((scoreMax - scoreMin) % scoreStep !== 0) {
          return "Диапазон оценок должен делиться на шаг без остатка";
        }
      }
    }

    if (targetStep >= 0) {
      if (seed.trim()) {
        const seedNum = Number(seed);
        if (!Number.isFinite(seedNum)) {
          return "Seed должен быть числом";
        }
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

  const toggleRuleSelection = (ruleId: string) => {
    setSelectedRuleIds((prev) => {
      const exists = prev.includes(ruleId);
      const next = exists ? prev.filter((item) => item !== ruleId) : [...prev, ruleId];

      if (next.length > 0) {
        setTallyRule(next[0]);
      } else {
        setTallyRule("");
      }

      return next;
    });
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
    setCreatedRunIds([]);
    setCreatedJobIds([]);
    setRawResp(null);

    try {
      let datasetId = "";

      if (sourceKind === "dataset") {
        datasetId = selectedDatasetId.trim();
      }

      if (sourceKind === "published_election") {
        const title = selectedPublishedElection?.title?.trim() || selectedElectionId.trim();

        datasetId = await api.datasets.fromElection(token, {
          election_id: selectedElectionId.trim(),
          name: `Датасет из голосования: ${title}`,
          description: `Сформирован автоматически при запуске эксперимента из опубликованного голосования ${selectedElectionId.trim()}`,
        });
      }

      if (!datasetId) {
        throw new Error("Не удалось определить датасет для запуска эксперимента");
      }

      const createdExperimentIds: string[] = [];
      const allRunIds: string[] = [];
      const allJobIds: string[] = [];
      const rawCreated: unknown[] = [];

      for (const ruleId of selectedRuleIds) {
        const ruleInfo = selectedRuleInfo(availableRules, ruleId);

        const params: Record<string, unknown> = {
          ...finalParams,
          dataset_id: datasetId,
          source_kind: sourceKind,
          tally_rule: ruleId,
          tally_rule_label: ruleDisplayLabel(ruleInfo, ruleId),
        };

        delete params.tally_rules;

        if (quotaEnabled && ruleInfo?.supports_quota_type) {
          params.quota_type = quotaType;
        } else {
          delete params.quota_type;
        }

        if (!(ballotFormat === "approval" && ruleInfo?.requires_approval_max_choices)) {
          delete params.approval_max_choices;
        }

        if (!(ballotFormat === "ranking" && ruleInfo?.supports_ranking_top_k && rankingTopKEnabled)) {
          delete params.ranking_top_k;
        }

        if (!(ballotFormat === "score" && ruleInfo?.requires_score_range)) {
          delete params.score_min;
          delete params.score_max;
          delete params.score_step;
        }

        const body: {
          type: string;
          params: Record<string, unknown>;
          seed?: number;
        } = {
          type,
          params,
        };

        if (seed.trim()) {
          body.seed = Number(seed);
        }

        const experimentId = await api.experiments.create(token, body);

        const runs = await api.experimentRuns.batch(token, {
          experiment_id: experimentId,
          dataset_ids: [datasetId],
        });

        createdExperimentIds.push(experimentId);

        const runIds = runs
          .map((item) => {
            if (typeof item.id === "string") return item.id;
            if (typeof item.run_id === "string") return item.run_id;
            return "";
          })
          .filter((item) => item.trim() !== "");

        const jobIds = runs
          .map((item) => (typeof item.job_id === "string" ? item.job_id : ""))
          .filter((item) => item.trim() !== "");

        allRunIds.push(...runIds);
        allJobIds.push(...jobIds);

        rawCreated.push({
          experiment_id: experimentId,
          rule_id: ruleId,
          body,
          runs,
        });
      }

      setCreatedId(createdExperimentIds.join(", "));
      setCreatedRunIds(allRunIds);
      setCreatedJobIds(allJobIds);

      addNotification({
        kind: "success",
        title: "Эксперименты запущены",
        message: `Создано экспериментов: ${createdExperimentIds.length}. Запусков: ${allRunIds.length}`,
      });

      if (IS_DEV) {
        setRawResp({
          dataset_id: datasetId,
          experiment_ids: createdExperimentIds,
          run_ids: allRunIds,
          job_ids: allJobIds,
          created: rawCreated,
        });
      }

      setStep(STEPS.length - 1);
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else if (e?.code === "election_not_published") {
        setErr("Для создания датасета можно выбрать только опубликованное голосование");
      } else if (e?.code === "election_not_ready") {
        setErr("Выбранное голосование еще не завершено");
      } else if (e?.code === "aggregates_disabled") {
        setErr("Для выбранного голосования отключен доступ к агрегированным данным");
      } else if (e?.code === "no_accepted_ballots") {
        setErr("В выбранном голосовании нет принятых бюллетеней");
      } else {
        setErr(e?.message || "Не удалось создать и запустить эксперименты");
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
            <div style={{ ...styles.card, background: "#f9fafb" }}>
              <h3 style={{ marginTop: 0 }}>Источник данных</h3>

              <div style={{ display: "grid", gap: 10 }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="radio"
                    checked={sourceKind === "dataset"}
                    onChange={() => setSourceKind("dataset")}
                  />
                  <span>Выбрать существующий датасет</span>
                </label>

                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="radio"
                    checked={sourceKind === "published_election"}
                    onChange={() => setSourceKind("published_election")}
                  />
                  <span>Выбрать опубликованное голосование</span>
                </label>
              </div>

              {sourceKind === "dataset" ? (
                <div style={{ marginTop: 12 }}>
                  <label>Датасет</label>
                  <select
                    style={styles.input}
                    value={selectedDatasetId}
                    onChange={(e) => setSelectedDatasetId(e.target.value)}
                    disabled={datasetsLoading || datasets.length === 0}
                  >
                    {datasets.map((item) => (
                      <option key={item.id} value={item.id}>
                        {datasetOptionLabel(item)}
                      </option>
                    ))}
                  </select>

                  {datasetsLoading ? (
                    <div style={{ marginTop: 8, ...styles.muted }}>
                      Загрузка датасетов...
                    </div>
                  ) : null}

                  {!datasetsLoading && datasets.length === 0 ? (
                    <div style={{ marginTop: 8, ...styles.muted }}>
                      Нет доступных датасетов. Сначала импортируйте или сгенерируйте набор данных.
                    </div>
                  ) : null}

                  {selectedDatasetDetail ? (
                    <div style={{ marginTop: 12, ...styles.muted }}>
                      Выбран датасет: {selectedDatasetDetail.name}. Формат: {selectedDatasetDetail.format}.
                      Кандидатов: {selectedDatasetDetail.candidates.length}.
                    </div>
                  ) : null}
                </div>
              ) : null}

              {sourceKind === "published_election" ? (
                <div style={{ marginTop: 12 }}>
                  <label>Опубликованное голосование</label>
                  <select
                    style={styles.input}
                    value={selectedElectionId}
                    onChange={(e) => setSelectedElectionId(e.target.value)}
                    disabled={electionsLoading || publishedElections.length === 0}
                  >
                    {publishedElections.map((item) => (
                      <option key={item.id} value={item.id}>
                        {electionOptionLabel(item)}
                      </option>
                    ))}
                  </select>

                  {electionsLoading ? (
                    <div style={{ marginTop: 8, ...styles.muted }}>
                      Загрузка опубликованных голосований...
                    </div>
                  ) : null}

                  {!electionsLoading && publishedElections.length === 0 ? (
                    <div style={{ marginTop: 8, ...styles.muted }}>
                      Нет опубликованных голосований. Для запуска по реальным бюллетеням сначала завершите и опубликуйте голосование.
                    </div>
                  ) : null}

                  {selectedPublishedElection ? (
                    <div style={{ marginTop: 12, ...styles.muted }}>
                      При создании эксперимента из этого голосования будет автоматически сформирован датасет.
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>

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
                    <option
                      key={item.value}
                      value={item.value}
                      disabled={!allowedBallotFormats.includes(item.value)}
                    >
                      {item.label}
                    </option>
                  ))}
                </select>
              </div>

              <div style={{ ...styles.card, background: "#f9fafb" }}>
                <label style={{ display: "block", fontWeight: 600, marginBottom: 8 }}>
                  Правила подсчёта
                </label>

                <div style={{ display: "grid", gap: 8 }}>
                  {rulesForSelectedBallotFormat.map((rule) => (
                    <label key={rule.id} style={{ display: "flex", gap: 8, alignItems: "center" }}>
                      <input
                        type="checkbox"
                        checked={selectedRuleIds.includes(rule.id)}
                        onChange={() => toggleRuleSelection(rule.id)}
                      />
                      <span>{ruleDisplayLabel(rule, rule.id)}</span>
                    </label>
                  ))}
                </div>

                {rulesForSelectedBallotFormat.length === 0 ? (
                  <div style={{ marginTop: 8, ...styles.muted }}>
                    Для выбранного формата бюллетеня нет доступных правил.
                  </div>
                ) : null}

                {selectedRuleIds.length > 0 ? (
                  <div style={{ marginTop: 8, ...styles.muted }}>
                    Выбрано правил: {selectedRuleIds.length}
                  </div>
                ) : null}
              </div>

              <div>
                <label>Размер комитета</label>
                <input
                  style={styles.input}
                  type="number"
                  min={1}
                  max={maxCommitteeSize}
                  value={committeeSize}
                  onChange={(e) => setCommitteeSize(Number(e.target.value))}
                />
                <div style={{ marginTop: 8, ...styles.muted }}>
                  Максимально доступный размер комитета: {maxCommitteeSize}.
                  Некоторые алгоритмы могут возвращать одного победителя независимо от указанного размера.
                </div>
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
                <label>Seed</label>
                <input
                  style={styles.input}
                  value={seed}
                  onChange={(e) => setSeed(e.target.value)}
                  placeholder="Например: 42"
                />
                <div style={{ marginTop: 8, ...styles.muted }}>
                  Необязательное зерно воспроизводимости. Если указать одно и то же значение, эксперимент можно будет повторить с теми же исходными условиями.
                </div>
              </div>

              <div style={{ ...styles.card, background: "#f9fafb" }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={quotaEnabled}
                    disabled={!quotaSupported}
                    onChange={(e) => setQuotaEnabled(e.target.checked)}
                  />
                  <span>Использовать квоту</span>
                </label>

                <div style={{ marginTop: 6, fontSize: 13, color: "#667085" }}>
                  {selectedRuleIds.length === 0
                    ? "Сначала выберите правила подсчёта."
                    : selectedRulesSupportQuota
                      ? "Квота доступна для части выбранных правил. Она будет применена только к правилам, которые поддерживают квоту."
                      : "Выбранные правила не поддерживают выбор квоты."}
                </div>

                {quotaEnabled && quotaSupported ? (
                  <div style={{ marginTop: 12, display: "grid", gap: 6 }}>
                    <label>Тип квоты</label>
                    <select
                      style={styles.input}
                      value={quotaType}
                      onChange={(e) => setQuotaType(e.target.value as "hare" | "droop")}
                    >
                      <option value="hare">Квота Хэра</option>
                      <option value="droop">Квота Друпа</option>
                    </select>

                    <div style={{ marginTop: 6, fontSize: 13, color: "#667085" }}>
                      {quotaTypeDescription(quotaType)}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}

        {step === 1 ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div style={{ ...styles.card, background: "#f9fafb" }}>
              {formatHint(ballotFormat)}
            </div>

            {ballotFormat === "approval" && selectedRulesNeedApprovalMax ? (
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

            {ballotFormat === "ranking" && selectedRulesSupportRankingTopK ? (
              <div style={{ display: "grid", gap: 10 }}>
                <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="checkbox"
                    checked={rankingTopKEnabled}
                    onChange={(e) => setRankingTopKEnabled(e.target.checked)}
                  />
                  Ограничивать число учитываемых позиций top-k
                </label>

                <div style={styles.muted}>
                  Поле необязательно. Если ограничение выключено, используется полное ранжирование.
                </div>

                {rankingTopKEnabled ? (
                  <>
                    <label>Ограничение top-k</label>
                    <input
                      style={styles.input}
                      type="number"
                      min={1}
                      max={candidates}
                      value={rankingTopK}
                      onChange={(e) => setRankingTopK(Number(e.target.value))}
                    />
                  </>
                ) : null}
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
            <SummaryGrid
              items={[
                {
                  label: "Источник данных",
                  value: sourceKindLabel(sourceKind),
                },
                {
                  label: sourceKind === "dataset" ? "Датасет" : "Опубликованное голосование",
                  value:
                    sourceKind === "dataset"
                      ? selectedDatasetListItem
                        ? datasetOptionLabel(selectedDatasetListItem)
                        : "—"
                      : selectedPublishedElection
                        ? electionOptionLabel(selectedPublishedElection)
                        : "—",
                },
                { label: "Тип эксперимента", value: type === "algo" ? "Алгоритмический" : "Поведенческий" },
                {
                  label: "Формат бюллетеня",
                  value:
                    ballotFormat === "approval"
                      ? "Одобрение"
                      : ballotFormat === "ranking"
                        ? "Ранжирование"
                        : "Оценивание",
                },
                {
                  label: "Правила подсчёта",
                  value:
                    selectedRuleInfos.length > 0
                      ? selectedRuleInfos.map((rule) => ruleDisplayLabel(rule, rule.id)).join(", ")
                      : "—",
                },
                { label: "Количество кандидатов", value: String(candidates) },
                { label: "Количество избирателей", value: String(voters) },
                { label: "Размер комитета", value: String(committeeSize) },
                {
                  label: "Тип квоты",
                  value: quotaEnabled && selectedRulesSupportQuota ? quotaType : "не используется",
                },
                { label: "Seed", value: seed.trim() || "не задан" },
                {
                  label: "Ограничение top-k",
                  value:
                    ballotFormat === "ranking" && selectedRulesSupportRankingTopK
                      ? rankingTopKEnabled
                        ? String(rankingTopK)
                        : "Не используется"
                      : "Не используется",
                },
              ]}
            />

            <div style={styles.card}>
              <h3 style={{ marginTop: 0 }}>Итоговые параметры эксперимента</h3>
              <JsonBlock value={finalParams} />
            </div>

            {createdId ? (
              <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0" }}>
                <div style={{ fontWeight: 700 }}>Эксперименты созданы и запущены</div>
                <div style={{ marginTop: 6 }}>Experiment ID: {createdId}</div>

                {createdRunIds.length > 0 ? (
                  <div style={{ marginTop: 6 }}>
                    Run ID: {createdRunIds.join(", ")}
                  </div>
                ) : null}

                {createdJobIds.length > 0 ? (
                  <div style={{ marginTop: 6 }}>
                    Job ID: {createdJobIds.join(", ")}
                  </div>
                ) : null}

                <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btnPrimary} onClick={() => nav("/research/runs")}>
                    К запускам
                  </button>
                  <button style={styles.btn} onClick={() => nav("/research/experiments")}>
                    К списку экспериментов
                  </button>
                  <button style={styles.btn} onClick={() => nav("/monitoring/jobs")}>
                    К задачам
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
                {loading ? "Создание…" : createdId ? "Уже создано" : "Создать и запустить эксперименты"}
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