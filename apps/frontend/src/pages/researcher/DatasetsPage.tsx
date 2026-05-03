import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import type {
  DatasetDetail,
  DatasetGenerateReq,
  DatasetListItem,
  ElectionSummary,
  TallyRuleInfo,
} from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { mergeRuleItems } from "../../shared/utils/mergeRuleItems";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";
import { useNotifications } from "../../app/notifications";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { styles } from "../../shared/ui/styles";

const GENERATION_MODELS = [
  { value: "uniform", label: "Равномерная" },
  { value: "consensus", label: "Консенсусная" },
  { value: "polarized", label: "Поляризованная" },
] as const;

const MAX_GENERATED_VOTERS = 1_000_000_000;

type CreatedSyntheticRun = {
  rule: string;
  experimentId: string;
  runId: string;
  jobId: string;
};

type DatasetBallotFormat = "approval" | "ranking" | "score";

function extractCreatedRun(value: unknown): { runId: string; jobId: string } {
  if (!value || typeof value !== "object") {
    return { runId: "", jobId: "" };
  }

  const rec = value as Record<string, unknown>;

  const runId =
    typeof rec.run_id === "string"
      ? rec.run_id
      : typeof rec.id === "string"
      ? rec.id
      : "";

  const jobId = typeof rec.job_id === "string" ? rec.job_id : "";

  return { runId, jobId };
}

function formatLabel(value: string) {
  switch (value) {
    case "approval":
      return "Одобрение";
    case "ranking":
      return "Ранжирование";
    case "score":
      return "Оценивание";
    default:
      return value;
  }
}

function sourceLabel(value: string) {
  switch (value) {
    case "generate":
      return "Сгенерирован";
    case "import":
      return "Импортирован";
    case "external":
      return "Внешний";
    case "election":
      return "Из голосования";
    default:
      return value;
  }
}

function shortId(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
}

function formatDateTime(value: unknown) {
  if (typeof value !== "string" || !value.trim()) return "—";

  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;

  return d.toLocaleString("ru-RU", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function boolLabel(value: unknown) {
  return value === true ? "Да" : value === false ? "Нет" : String(value ?? "—");
}

function parameterLabel(key: string) {
  const labels: Record<string, string> = {
    generation_model: "Модель генерации",
    approval_max_choices: "Максимум одобрений",
    ranking_top_k: "Глубина ранжирования",
    score_min: "Минимальная оценка",
    score_max: "Максимальная оценка",
    score_step: "Шаг оценки",
    voters: "Число профилей",
    candidates: "Кандидаты",
    seed: "Seed",
  };

  return labels[key] || key;
}

function generationModelLabel(value: string) {
  switch (value) {
    case "uniform":
      return "Равномерная";
    case "consensus":
      return "Консенсусная";
    case "polarized":
      return "Поляризованная";
    default:
      return value;
  }
}

function generationModelDescription(value: string) {
  switch (value) {
    case "uniform":
      return "Профили предпочтений формируются случайно и независимо.";
    case "consensus":
      return "Предпочтения концентрируются вокруг общего порядка кандидатов.";
    case "polarized":
      return "Предпочтения делятся на несколько противоположных групп.";
    default:
      return "";
  }
}

function ruleLabelRu(id: string, fallback?: string) {
  if (fallback?.trim()) return fallback.trim();

  const label = tallyRuleLabel(id);
  return label !== "—" ? label : id;
}

function getGenerationModelFromParameters(parameters?: Record<string, unknown>) {
  if (!parameters || typeof parameters !== "object") return "";
  const value = parameters.generation_model;
  return typeof value === "string" ? value : "";
}

function getRankingTopKFromParameters(parameters?: Record<string, unknown>) {
  if (!parameters || typeof parameters !== "object") return null;
  const value = parameters.ranking_top_k;
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function isDatasetBallotFormat(value: unknown): value is DatasetBallotFormat {
  return value === "approval" || value === "ranking" || value === "score";
}

function datasetBallotFormat(value?: DatasetDetail | null): DatasetBallotFormat | null {
  return isDatasetBallotFormat(value?.format) ? value.format : null;
}

function numberParameter(parameters: Record<string, unknown> | undefined, key: string): number | null {
  if (!parameters || typeof parameters !== "object") return null;

  const value = parameters[key];
  if (typeof value === "number" && Number.isFinite(value)) return value;

  if (typeof value === "string" && value.trim()) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }

  return null;
}

function ruleSupportsFormat(rule: TallyRuleInfo, format: DatasetBallotFormat) {
  return rule.supports_experiment_runs && rule.ballot_formats.includes(format);
}

function renderParameters(value: Record<string, unknown> | undefined) {
  if (!value || Object.keys(value).length === 0) {
    return <span style={styles.muted}>Нет дополнительных параметров</span>;
  }

  return (
    <div style={{ display: "grid", gap: 6 }}>
      {Object.entries(value).map(([key, val]) => {
        let renderedValue: React.ReactNode =
          typeof val === "string" ? val : JSON.stringify(val);

        if (typeof val === "boolean") {
          renderedValue = boolLabel(val);
        }

        if (key === "generation_model" && typeof val === "string") {
          renderedValue = generationModelLabel(val);
        }

        return (
          <div key={key}>
            <b>{parameterLabel(key)}:</b> <span>{renderedValue}</span>
          </div>
        );
      })}
    </div>
  );
}

export function DatasetsPage() {
  const { token, setToken } = useAuth();
  const nav = useNavigate();
  const { addNotification } = useNotifications();

  const [items, setItems] = useState<DatasetListItem[]>([]);
  const [selected, setSelected] = useState<DatasetDetail | null>(null);

  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const [importName, setImportName] = useState("");
  const [importDescription, setImportDescription] = useState("");
  const [importFormat, setImportFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [importFile, setImportFile] = useState<File | null>(null);

  const [genName, setGenName] = useState("Synthetic dataset");
  const [genDescription, setGenDescription] = useState("");
  const [genFormat, setGenFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [genModel, setGenModel] = useState<"uniform" | "consensus" | "polarized">("uniform");
  const [genVoters, setGenVoters] = useState(100);
  const [genSeed, setGenSeed] = useState("");
  const [genCandidatesText, setGenCandidatesText] = useState("c1,Alice\nc2,Bob\nc3,Carol");
  const [genApprovalMax, setGenApprovalMax] = useState(2);
  const [genRankingTopK, setGenRankingTopK] = useState(3);
  const [genScoreMin, setGenScoreMin] = useState(0);
  const [genScoreMax, setGenScoreMax] = useState(10);
  const [genScoreStep, setGenScoreStep] = useState(1);

  const [runRules, setRunRules] = useState<string[]>([]);
  const [runCommitteeSize, setRunCommitteeSize] = useState(1);
  const [runRankingTopK, setRunRankingTopK] = useState(3);
  const [runApprovalMaxChoices, setRunApprovalMaxChoices] = useState(2);
  const [runScoreMin, setRunScoreMin] = useState(0);
  const [runScoreMax, setRunScoreMax] = useState(10);
  const [runScoreStep, setRunScoreStep] = useState(1);

  const [runLoading, setRunLoading] = useState(false);
  const [createdRuns, setCreatedRuns] = useState<CreatedSyntheticRun[]>([]);
  const [availableRunRules, setAvailableRunRules] = useState<TallyRuleInfo[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);

  const [electionItems, setElectionItems] = useState<ElectionSummary[]>([]);
  const [electionsLoading, setElectionsLoading] = useState(false);

  const [electionDatasetElectionId, setElectionDatasetElectionId] = useState("");
  const [electionDatasetName, setElectionDatasetName] = useState("");
  const [electionDatasetDescription, setElectionDatasetDescription] = useState("");
  const [electionDatasetLoading, setElectionDatasetLoading] = useState(false);
  const [electionDatasetErr, setElectionDatasetErr] = useState<string | null>(null);

  const [importErr, setImportErr] = useState<string | null>(null);
  const [generateErr, setGenerateErr] = useState<string | null>(null);
  const [runErr, setRunErr] = useState<string | null>(null);

  const [genRankingTopKEnabled, setGenRankingTopKEnabled] = useState(false);
  const [runRankingTopKEnabled, setRunRankingTopKEnabled] = useState(false);

  const listAbortRef = useRef<AbortController | null>(null);
  const detailAbortRef = useRef<AbortController | null>(null);

  const loadList = useCallback(async () => {
    if (!token) return;

    listAbortRef.current?.abort();
    const ac = new AbortController();
    listAbortRef.current = ac;

    setLoading(true);
    setErr(null);
    setInfo(null);

    try {
      const list = await api.datasets.list(token, ac.signal);
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить наборы данных");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    loadList();
    return () => {
      listAbortRef.current?.abort();
      detailAbortRef.current?.abort();
    };
  }, [loadList]);

  useEffect(() => {
    if (!token) return;

    const ac = new AbortController();
    setElectionsLoading(true);

    api.elections
      .list(token, ac.signal)
      .then((list) => {
        setElectionItems(
          list.filter((item) =>
            ["closed", "results_ready", "published"].includes(String(item.status || ""))
          )
        );
      })
      .catch((e: any) => {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setElectionItems([]);
      })
      .finally(() => {
        setElectionsLoading(false);
      });

    return () => ac.abort();
  }, [token, setToken]);

  useEffect(() => {
    if (!token) return;

    const ac = new AbortController();
    setRulesLoading(true);

    api.capabilities
      .tallyRules(token, ac.signal)
      .then((rules) => {
        const mergedRules = mergeRuleItems(rules);
        setAvailableRunRules(mergedRules.filter((rule) => rule.supports_experiment_runs));
      })
      .catch((e: any) => {
        if (e?.name === "AbortError") return;
        if (e?.status === 401) {
          setToken(null);
          return;
        }
        setErr((prev) => prev || "Не удалось загрузить список правил для экспериментов");
        setAvailableRunRules([]);
      })
      .finally(() => {
        setRulesLoading(false);
      });

    return () => ac.abort();
  }, [token, setToken]);

  useEffect(() => {
    if (!selected) {
      setRunRules([]);
      return;
    }

    const format = datasetBallotFormat(selected);
    if (!format) {
      setRunRules([]);
      return;
    }

    const formatRules = availableRunRules.filter((rule) => ruleSupportsFormat(rule, format));
    const allowed = new Set(formatRules.map((rule) => rule.id));

    setRunRules((prev) => {
      const next = prev.filter((rule) => allowed.has(rule));
      if (next.length > 0) return next;
      return formatRules.slice(0, 3).map((rule) => rule.id);
    });

    const candidateCount = Math.max(1, selected.candidates.length);

    if (format === "approval") {
      const fromDataset = numberParameter(selected.parameters, "approval_max_choices");
      setRunApprovalMaxChoices(Math.max(1, Math.min(candidateCount, fromDataset ?? 2)));
    }

    if (format === "ranking") {
      const fromDataset = numberParameter(selected.parameters, "ranking_top_k");
      if (fromDataset != null) {
        setRunRankingTopKEnabled(true);
        setRunRankingTopK(Math.max(1, Math.min(candidateCount, fromDataset)));
      } else {
        setRunRankingTopKEnabled(false);
        setRunRankingTopK(Math.max(1, Math.min(candidateCount, 3)));
      }
    }

    if (format === "score") {
      const min = numberParameter(selected.parameters, "score_min");
      const max = numberParameter(selected.parameters, "score_max");
      const step = numberParameter(selected.parameters, "score_step");

      setRunScoreMin(min ?? 0);
      setRunScoreMax(max ?? 10);
      setRunScoreStep(step ?? 1);
    }
  }, [selected, availableRunRules]);

  const loadDetail = async (id: string) => {
    if (!token) return;

    detailAbortRef.current?.abort();
    const ac = new AbortController();
    detailAbortRef.current = ac;

    setDetailLoading(true);
    setErr(null);
    setInfo(null);

    try {
      const ds = await api.datasets.get(token, id, ac.signal);
      setSelected(ds);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить набор данных");
      setSelected(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleDownload = async (id: string) => {
    if (!token) return;

    setErr(null);
    setInfo(null);

    try {
      const { blob, filename } = await api.datasets.download(token, id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);

      setInfo("Файл успешно скачан");
      addNotification({
        kind: "success",
        title: "Набор данных скачан",
        message: `Файл ${filename} успешно подготовлен к загрузке`,
      });
    } catch (e: any) {
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось скачать набор данных");
    }
  };

  const handleImport = async () => {
    if (!token) return;

    setImportErr(null);
    setGenerateErr(null);
    setRunErr(null);
    setErr(null);
    setInfo(null);

    if (!importName.trim()) {
      setImportErr("Введите название набора данных");
      return;
    }
    if (!importFile) {
      setImportErr("Выберите файл для импорта");
      return;
    }

    setLoading(true);

    try {
      const id = await api.datasets.importFile(token, {
        name: importName.trim(),
        description: importDescription.trim(),
        format: importFormat,
        file: importFile,
      });

      setInfo(`Набор данных импортирован. Технический ID: ${shortId(id)}`);
      setImportName("");
      setImportDescription("");
      setImportFile(null);

      addNotification({
        kind: "success",
        title: "Импорт набора данных завершен",
        message: `Новый набор данных создан. Технический ID: ${shortId(id)}`,
      });

      await loadList();
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setImportErr(e?.message || "Не удалось импортировать набор данных");
      }
    } finally {
      setLoading(false);
    }
  };

  const parseCandidates = (): Array<{ id: string; name: string }> => {
    const lines = genCandidatesText
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);

    const parsed = lines.map((line) => {
      const [idPart, ...nameParts] = line.split(",");
      const id = idPart?.trim();
      const name = nameParts.join(",").trim();
      return { id, name };
    });

    if (parsed.length < 2) {
      throw new Error("Нужно указать минимум двух кандидатов");
    }

    for (const item of parsed) {
      if (!item.id || !item.name) {
        throw new Error("Каждая строка кандидата должна иметь формат id,name");
      }
    }

    const ids = new Set(parsed.map((item) => item.id));
    if (ids.size !== parsed.length) {
      throw new Error("Идентификаторы кандидатов не должны повторяться");
    }

    return parsed;
  };

  const toggleRunRule = (rule: string) => {
    setRunRules((prev) =>
      prev.includes(rule) ? prev.filter((item) => item !== rule) : [...prev, rule]
    );
  };

  const handleGenerate = async () => {
    if (!token) return;

    setGenerateErr(null);
    setImportErr(null);
    setRunErr(null);
    setErr(null);
    setInfo(null);

    if (!genName.trim()) {
      setGenerateErr("Введите название синтетического набора данных");
      return;
    }

    if (genVoters < 1) {
      setGenerateErr("Количество профилей должно быть не меньше 1");
      return;
    }

    if (genVoters > MAX_GENERATED_VOTERS) {
      setGenerateErr(`Количество профилей не может превышать ${MAX_GENERATED_VOTERS.toLocaleString("ru-RU")}`);
      return;
    }

    setLoading(true);

    try {
      const candidates = parseCandidates();

      const body: DatasetGenerateReq = {
        name: genName.trim(),
        description: genDescription.trim(),
        format: genFormat,
        voters: genVoters,
        generation_model: genModel,
        candidates,
      };

      if (genSeed.trim()) {
        const parsedSeed = Number(genSeed.trim());
        if (!Number.isInteger(parsedSeed)) {
          throw new Error("Seed должен быть целым числом");
        }
        if (parsedSeed < 0) {
          throw new Error("Seed должен быть неотрицательным числом");
        }
        body.seed = parsedSeed;
      }

      if (genFormat === "approval") {
        body.approval_max_choices = genApprovalMax;
      }

      if (genFormat === "ranking" && genRankingTopKEnabled) {
        body.ranking_top_k = genRankingTopK;
      }

      if (genFormat === "score") {
        body.score_min = genScoreMin;
        body.score_max = genScoreMax;
        body.score_step = genScoreStep;
      }

      const id = await api.datasets.generate(token, body);

      if (genFormat === "approval") {
        setRunApprovalMaxChoices(genApprovalMax);
      }

      if (genFormat === "ranking") {
        setRunRankingTopKEnabled(genRankingTopKEnabled);
        setRunRankingTopK(genRankingTopK);
      }

      if (genFormat === "score") {
        setRunScoreMin(genScoreMin);
        setRunScoreMax(genScoreMax);
        setRunScoreStep(genScoreStep);
      }

      setCreatedRuns([]);
      setInfo(`Синтетический набор данных создан. Технический ID: ${shortId(id)}`);

      addNotification({
        kind: "success",
        title: "Синтетический набор данных создан",
        message: `Создан синтетический набор данных. Технический ID: ${shortId(id)}`,
      });

      await loadList();
      await loadDetail(id);
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setGenerateErr(e?.message || "Не удалось сгенерировать набор данных");
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCreateDatasetFromElection = async () => {
    if (!token) return;

    setElectionDatasetErr(null);
    setImportErr(null);
    setGenerateErr(null);
    setRunErr(null);
    setErr(null);
    setInfo(null);

    const electionId = electionDatasetElectionId.trim();
    if (!electionId) {
      setElectionDatasetErr("Выберите голосование для создания датасета");
      return;
    }

    setElectionDatasetLoading(true);

    try {
      const id = await api.datasets.fromElection(token, {
        election_id: electionId,
        name: electionDatasetName.trim() || undefined,
        description: electionDatasetDescription.trim() || undefined,
      });

      setInfo(`Датасет из голосования создан. Технический ID: ${shortId(id)}`);
      setElectionDatasetName("");
      setElectionDatasetDescription("");

      addNotification({
        kind: "success",
        title: "Датасет из голосования создан",
        message: `Реальные бюллетени анонимизированы и перенесены в исследовательский датасет. Технический ID: ${shortId(id)}`,
      });

      await loadList();
      await loadDetail(id);
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else if (e?.code === "election_not_published") {
        setElectionDatasetErr("Исследователь может использовать только опубликованные голосования.");
      } else if (e?.code === "election_not_ready") {
        setElectionDatasetErr("Голосование еще не завершено.");
      } else if (e?.code === "aggregates_disabled") {
        setElectionDatasetErr("Для этого голосования отключен доступ к агрегированным результатам.");
      } else if (e?.code === "no_accepted_ballots") {
        setElectionDatasetErr("В голосовании нет принятых бюллетеней.");
      } else if (e?.code === "forbidden") {
        setElectionDatasetErr("Недостаточно прав для создания датасета из этого голосования.");
      } else {
        setElectionDatasetErr(e?.message || "Не удалось создать датасет из голосования");
      }
    } finally {
      setElectionDatasetLoading(false);
    }
  };

  const handleCreateAndRunExperiments = async () => {
    if (!token) return;

    setRunErr(null);
    setImportErr(null);
    setGenerateErr(null);
    setErr(null);
    setInfo(null);

    if (!selected) {
      setRunErr("Сначала выберите набор данных");
      return;
    }

    const format = datasetBallotFormat(selected);
    if (!format) {
      setRunErr("Формат выбранного набора данных не поддерживается");
      return;
    }

    if (rulesLoading) {
      setRunErr("Список правил для экспериментов еще загружается");
      return;
    }

    const formatRules = availableRunRules.filter((rule) => ruleSupportsFormat(rule, format));
    if (formatRules.length === 0) {
      setRunErr(`Нет доступных правил для формата "${formatLabel(format)}"`);
      return;
    }

    const allowedRuleIds = new Set(formatRules.map((rule) => rule.id));
    if (runRules.some((rule) => !allowedRuleIds.has(rule))) {
      setRunErr("Выбрано недопустимое правило подсчета для текущего формата набора данных");
      return;
    }

    if (runRules.length === 0) {
      setRunErr("Выберите хотя бы одно правило");
      return;
    }

    if (runCommitteeSize < 1) {
      setRunErr("Размер комитета должен быть не меньше 1");
      return;
    }

    const selectedRuleInfos = formatRules.filter((rule) => runRules.includes(rule.id));

    if (format === "approval") {
      if (runApprovalMaxChoices < 1) {
        setRunErr("Максимум одобрений должен быть не меньше 1");
        return;
      }

      if (runApprovalMaxChoices > selected.candidates.length) {
        setRunErr("Максимум одобрений не может превышать число кандидатов");
        return;
      }
    }

    if (format === "ranking" && runRankingTopKEnabled) {
      if (runRankingTopK < 1) {
        setRunErr("Ограничение top-k должно быть не меньше 1");
        return;
      }

      if (runRankingTopK > selected.candidates.length) {
        setRunErr("Ограничение top-k не может превышать число кандидатов");
        return;
      }
    }

    if (format === "score") {
      if (runScoreMax <= runScoreMin) {
        setRunErr("Максимальная оценка должна быть больше минимальной");
        return;
      }

      if (runScoreStep <= 0) {
        setRunErr("Шаг оценки должен быть больше 0");
        return;
      }

      const range = runScoreMax - runScoreMin;
      const steps = range / runScoreStep;
      if (!Number.isInteger(steps)) {
        setRunErr("Диапазон оценок должен делиться на шаг без остатка");
        return;
      }
    }

    setRunLoading(true);
    setCreatedRuns([]);

    try {
      const created: CreatedSyntheticRun[] = [];

      for (const rule of runRules) {
        const ruleInfo = selectedRuleInfos.find((item) => item.id === rule);

        const params: Record<string, unknown> = {
          ballot_format: format,
          tally_rule: rule,
          committee_size: runCommitteeSize,
        };

        if (format === "approval") {
          params.approval_max_choices = runApprovalMaxChoices;
        }

        if (format === "ranking" && runRankingTopKEnabled) {
          params.ranking_top_k = runRankingTopK;
        }

        if (format === "score") {
          params.score_min = runScoreMin;
          params.score_max = runScoreMax;
          params.score_step = runScoreStep;
        }

        const experimentId = await api.experiments.create(token, {
          type: "algo",
          params,
        });

        const batchItems = await api.experimentRuns.batch(token, {
          experiment_id: experimentId,
          dataset_ids: [selected.id],
        });

        const first = Array.isArray(batchItems) ? batchItems[0] : null;
        const { runId, jobId } = extractCreatedRun(first);

        if (!runId) {
          throw new Error(`Не удалось получить run_id для правила ${ruleLabelRu(rule, ruleInfo?.label)}`);
        }

        created.push({
          rule,
          experimentId,
          runId,
          jobId,
        });
      }

      setCreatedRuns(created);

      addNotification({
        kind: "success",
        title: "Эксперименты запущены",
        message: `Создано и запущено экспериментов: ${created.length}`,
      });

      nav("/research/runs", {
        state: {
          createdRuns: created,
          autoOpenRunId: created[0]?.runId || "",
        },
      });
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
      } else {
        setRunErr(e?.message || "Не удалось создать и запустить эксперименты");
      }
    } finally {
      setRunLoading(false);
    }
  };

  const generatedCandidatesCount = genCandidatesText
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean).length;

  const selectedFormat = datasetBallotFormat(selected);
  const formatRunRules = selectedFormat
    ? availableRunRules.filter((rule) => ruleSupportsFormat(rule, selectedFormat))
    : [];

  const selectedRuleInfos = formatRunRules.filter((rule) => runRules.includes(rule.id));
  const needsApprovalMaxChoices =
    selectedFormat === "approval" &&
    selectedRuleInfos.some((rule) => rule.requires_approval_max_choices);
  const needsScoreRange =
    selectedFormat === "score" &&
    selectedRuleInfos.some((rule) => rule.requires_score_range);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Наборы данных</h2>
          <button style={styles.btn} onClick={loadList} disabled={loading}>
            Обновить
          </button>
        </div>

        <ErrorBanner error={err} />
        {info ? (
          <div style={{ ...styles.card, background: "#f0fdf4", borderColor: "#bbf7d0", marginBottom: 12 }}>
            {info}
          </div>
        ) : null}

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && items.length === 0 ? <div style={styles.muted}>Список пуст</div> : null}

          {items.map((item) => (
            <div key={item.id} style={{ ...styles.card, padding: 12 }}>
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  gap: 10,
                  alignItems: "flex-start",
                  flexWrap: "wrap",
                }}
              >
                <div>
                  <div style={{ fontWeight: 800 }}>{item.name}</div>
                  <div style={{ ...styles.muted, marginTop: 4 }}>
                    {sourceLabel(item.source)} · {formatLabel(item.format)}
                  </div>
                </div>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <Badge text={sourceLabel(item.source)} />
                  <Badge text={formatLabel(item.format)} />
                </div>
              </div>

              <div style={{ marginTop: 10 }}>
                <KeyValueList
                  items={[
                    { label: "Создан", value: formatDateTime(item.created_at) },
                    { label: "Формат", value: formatLabel(item.format) },
                    { label: "Источник", value: sourceLabel(item.source) },
                  ]}
                />
              </div>

              <details style={{ marginTop: 10 }}>
                <summary style={{ cursor: "pointer", ...styles.muted }}>
                  Технические сведения
                </summary>
                <div style={{ marginTop: 8 }}>
                  <KeyValueList
                    items={[
                      { label: "ID набора данных", value: item.id },
                      { label: "Источник", value: item.source },
                      { label: "Формат", value: item.format },
                      { label: "Создан", value: item.created_at },
                    ]}
                  />
                </div>
              </details>

              <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                <button style={styles.btnPrimary} onClick={() => loadDetail(item.id)} disabled={detailLoading}>
                  Открыть карточку
                </button>
                <button style={styles.btn} onClick={() => handleDownload(item.id)}>
                  Скачать файл
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div style={styles.grid2}>
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Импорт набора данных</h3>

          <label>Название</label>
          <input style={styles.input} value={importName} onChange={(e) => setImportName(e.target.value)} />

          <div style={{ height: 10 }} />

          <label>Описание</label>
          <input style={styles.input} value={importDescription} onChange={(e) => setImportDescription(e.target.value)} />

          <div style={{ height: 10 }} />

          <label>Формат</label>
          <select style={styles.input} value={importFormat} onChange={(e) => setImportFormat(e.target.value as any)}>
            <option value="approval">Одобрение</option>
            <option value="ranking">Ранжирование</option>
            <option value="score">Оценивание</option>
          </select>
          <div style={{ marginTop: 8, ...styles.muted }}>
            Для JSON формат задается вручную. Для PrefLib/Pabulib он может быть определен содержимым файла.
          </div>

          <div style={{ height: 10 }} />

          <label>Файл</label>
          <input
            style={styles.input}
            type="file"
            accept=".json,.soc,.soi,.pb"
            onChange={(e) => {
              const file = e.target.files?.[0] ?? null;
              setImportFile(file);

              const name = file?.name?.toLowerCase() ?? "";
              if (name.endsWith(".soc") || name.endsWith(".soi")) {
                setImportFormat("ranking");
              }
            }}
          />

          <div style={{ marginTop: 8, ...styles.muted }}>
            Поддерживаются JSON, PrefLib (.soc, .soi), Pabulib (.pb)
          </div>

          <div style={{ marginTop: 6, ...styles.muted }}>
            Для файлов .soc/.soi формат будет ranking. Для .pb формат определяется сервером по vote_type.
          </div>

          {importFile ? (
            <div style={{ marginTop: 8, ...styles.muted }}>
              {importFile.name} · {importFile.size} байт
            </div>
          ) : null}

          <div style={{ height: 12 }} />

          <button style={styles.btnPrimary} onClick={handleImport} disabled={loading}>
            Импортировать
          </button>
          {importErr ? (
            <div style={{ marginTop: 10, color: "#b91c1c", fontSize: 13 }}>{importErr}</div>
          ) : null}
        </div>

        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Генерация синтетики</h3>

          <label>Название</label>
          <input style={styles.input} value={genName} onChange={(e) => setGenName(e.target.value)} />

          <div style={{ height: 10 }} />

          <label>Описание</label>
          <input style={styles.input} value={genDescription} onChange={(e) => setGenDescription(e.target.value)} />

          <div style={{ height: 10 }} />

          <label>Формат</label>
          <select style={styles.input} value={genFormat} onChange={(e) => setGenFormat(e.target.value as any)}>
            <option value="approval">Одобрение</option>
            <option value="ranking">Ранжирование</option>
            <option value="score">Оценивание</option>
          </select>

          <div style={{ height: 10 }} />

          <label>Модель генерации</label>
          <select
            style={styles.input}
            value={genModel}
            onChange={(e) => setGenModel(e.target.value as "uniform" | "consensus" | "polarized")}
          >
            {GENERATION_MODELS.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>

          <div style={{ marginTop: 8, ...styles.muted }}>
            {generationModelDescription(genModel)}
          </div>

          <div style={{ height: 10 }} />

          <label>Число профилей</label>
          <input
            style={styles.input}
            type="number"
            min={1}
            max={MAX_GENERATED_VOTERS}
            value={genVoters}
            onChange={(e) => setGenVoters(Number(e.target.value))}
          />
          <div style={{ marginTop: 6, fontSize: 13, color: "#667085" }}>
            Для больших наборов профили сохраняются в отдельной коллекции dataset_ballots. Полный raw JSON в карточке датасета сохраняется только для небольших наборов.
          </div>

          <div style={{ height: 10 }} />

          <label>Seed (зерно генерации)</label>
          <input style={styles.input} value={genSeed} onChange={(e) => setGenSeed(e.target.value)} />
          <div style={{ marginTop: 8, ...styles.muted }}>
            Если поле оставить пустым, сервер сгенерирует seed автоматически.
          </div>

          <div style={{ height: 10 }} />

          <label>Кандидаты (каждая строка: технический ID, имя)</label>
          <textarea
            style={{
              ...styles.input,
              minHeight: 120,
              fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
            }}
            value={genCandidatesText}
            onChange={(e) => setGenCandidatesText(e.target.value)}
          />

          <div style={{ height: 10 }} />

          {genFormat === "approval" ? (
            <>
              <label>Максимум одобрений</label>
              <input
                style={styles.input}
                type="number"
                min={1}
                value={genApprovalMax}
                onChange={(e) => setGenApprovalMax(Number(e.target.value))}
              />
              <div style={{ height: 10 }} />
            </>
          ) : null}

          {genFormat === "ranking" ? (
            <>
              <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <input
                  type="checkbox"
                  checked={genRankingTopKEnabled}
                  onChange={(e) => setGenRankingTopKEnabled(e.target.checked)}
                />
                Ограничивать число учитываемых позиций top-k
              </label>

              <div style={{ marginTop: 8, ...styles.muted }}>
                Это поле необязательно. Если ограничение выключено, будет использоваться полное ранжирование.
              </div>

              {genRankingTopKEnabled ? (
                <>
                  <div style={{ height: 10 }} />
                  <label>Ограничение top-k</label>
                  <input
                    style={styles.input}
                    type="number"
                    min={1}
                    max={Math.max(1, generatedCandidatesCount)}
                    value={genRankingTopK}
                    onChange={(e) => setGenRankingTopK(Number(e.target.value))}
                  />
                </>
              ) : null}

              <div style={{ height: 10 }} />
            </>
          ) : null}

          {genFormat === "score" ? (
            <>
              <label>Минимальная оценка</label>
              <input
                style={styles.input}
                type="number"
                value={genScoreMin}
                onChange={(e) => setGenScoreMin(Number(e.target.value))}
              />
              <div style={{ height: 10 }} />

              <label>Максимальная оценка</label>
              <input
                style={styles.input}
                type="number"
                value={genScoreMax}
                onChange={(e) => setGenScoreMax(Number(e.target.value))}
              />
              <div style={{ height: 10 }} />

              <label>Шаг оценки</label>
              <input
                style={styles.input}
                type="number"
                min={1}
                value={genScoreStep}
                onChange={(e) => setGenScoreStep(Number(e.target.value))}
              />
              <div style={{ height: 10 }} />
            </>
          ) : null}

          <button style={styles.btnPrimary} onClick={handleGenerate} disabled={loading}>
            Сгенерировать
          </button>
          {generateErr ? (
            <div style={{ marginTop: 10, color: "#b91c1c", fontSize: 13 }}>{generateErr}</div>
          ) : null}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Датасет из реального голосования</h3>

        <div style={{ ...styles.muted, marginBottom: 12 }}>
          Завершенное или опубликованное голосование можно перенести в исследовательский датасет.
          Бюллетени сохраняются без идентификаторов пользователей.
        </div>

        <label>Голосование</label>
        <select
          style={styles.input}
          value={electionDatasetElectionId}
          onChange={(e) => {
            const value = e.target.value;
            setElectionDatasetElectionId(value);

            const selectedElection = electionItems.find((item) => item.id === value);
            if (selectedElection && !electionDatasetName.trim()) {
              setElectionDatasetName(`Dataset from election: ${selectedElection.title}`);
            }
          }}
        >
          <option value="">
            {electionsLoading ? "Загрузка голосований..." : "Выберите голосование"}
          </option>
          {electionItems.map((item) => (
            <option key={item.id} value={item.id}>
              {item.title} · {formatLabel(String(item.ballot_format || ""))} · {item.status}
            </option>
          ))}
        </select>

        <div style={{ height: 10 }} />

        <label>Название датасета</label>
        <input
          style={styles.input}
          value={electionDatasetName}
          onChange={(e) => setElectionDatasetName(e.target.value)}
          placeholder="Например: Dataset from published election"
        />

        <div style={{ height: 10 }} />

        <label>Описание</label>
        <input
          style={styles.input}
          value={electionDatasetDescription}
          onChange={(e) => setElectionDatasetDescription(e.target.value)}
          placeholder="Краткое описание назначения датасета"
        />

        <div style={{ marginTop: 8, ...styles.muted }}>
          Для исследователя доступны опубликованные голосования с разрешенными агрегатами.
          Администратор может экспортировать свои завершенные голосования.
        </div>

        <div style={{ height: 12 }} />

        <button
          style={styles.btnPrimary}
          onClick={handleCreateDatasetFromElection}
          disabled={electionDatasetLoading || !electionDatasetElectionId.trim()}
        >
          {electionDatasetLoading ? "Создание датасета..." : "Создать датасет из голосования"}
        </button>

        {electionDatasetErr ? (
          <div style={{ marginTop: 10, color: "#b91c1c", fontSize: 13 }}>
            {electionDatasetErr}
          </div>
        ) : null}
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Карточка набора данных</h3>
        {detailLoading ? <div style={styles.muted}>Загрузка…</div> : null}

        {selected ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div>
              <div style={{ fontWeight: 700, fontSize: 18 }}>{selected.name}</div>
              <div style={styles.muted}>{selected.description || "Описание отсутствует"}</div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={sourceLabel(selected.source)} />
              <Badge text={formatLabel(selected.format)} />
              {selected.seed != null ? <Badge text={`Seed: ${selected.seed}`} /> : null}
              {getGenerationModelFromParameters(selected.parameters) ? (
                <Badge text={`Модель: ${generationModelLabel(getGenerationModelFromParameters(selected.parameters))}`} />
              ) : null}
              {getRankingTopKFromParameters(selected.parameters) != null ? (
                <Badge text={`top-k: ${getRankingTopKFromParameters(selected.parameters)}`} />
              ) : null}
            </div>

            <KeyValueList
              items={[
                { label: "Источник", value: sourceLabel(selected.source) },
                { label: "Формат", value: formatLabel(selected.format) },
                { label: "Дата создания", value: formatDateTime(selected.created_at) },
                { label: "Число кандидатов", value: String(selected.candidates.length) },
                { label: "Seed", value: selected.seed != null ? String(selected.seed) : "—" },
              ]}
            />

            <details>
              <summary style={{ cursor: "pointer", ...styles.muted }}>
                Технические сведения
              </summary>
              <div style={{ marginTop: 10 }}>
                <KeyValueList
                  items={[
                    { label: "ID набора данных", value: selected.id },
                    { label: "Источник", value: selected.source },
                    { label: "Формат", value: selected.format },
                    { label: "Создан", value: selected.created_at },
                  ]}
                />
              </div>
            </details>

            <div>
              <h4 style={{ marginBottom: 8 }}>Кандидаты</h4>
              {selected.candidates.length > 0 ? (
                <div style={{ display: "grid", gap: 6 }}>
                  {selected.candidates.map((candidate) => (
                    <div key={candidate.id} style={{ ...styles.card, padding: 10 }}>
                      <b>{candidate.name}</b>
                      <details style={{ marginTop: 4 }}>
                        <summary style={{ cursor: "pointer", ...styles.muted }}>
                          Технические сведения
                        </summary>
                        <div style={{ marginTop: 4, ...styles.muted }}>
                          ID кандидата: {candidate.id}
                        </div>
                      </details>
                    </div>
                  ))}
                </div>
              ) : (
                <div style={styles.muted}>Список кандидатов пуст</div>
              )}
            </div>

            <div>
              <h4 style={{ marginBottom: 8 }}>Параметры</h4>
              {renderParameters(selected.parameters)}
            </div>

            {selectedFormat ? (
              <div style={{ ...styles.card, padding: 12 }}>
                <h4 style={{ marginTop: 0, marginBottom: 8 }}>Создание и запуск экспериментов</h4>

                <div style={{ ...styles.muted, marginBottom: 12 }}>
                  Для выбранного набора данных можно сразу создать и запустить серию экспериментов
                  по правилам, которые поддерживают формат "{formatLabel(selectedFormat)}".
                </div>

                <div style={{ display: "grid", gap: 12 }}>
                  <div>
                    <div style={{ fontWeight: 600, marginBottom: 8 }}>Правила подсчёта</div>

                    {rulesLoading ? (
                      <div style={styles.muted}>Загрузка списка правил…</div>
                    ) : null}

                    {!rulesLoading && formatRunRules.length === 0 ? (
                      <div style={styles.muted}>
                        Нет доступных правил для формата "{formatLabel(selectedFormat)}"
                      </div>
                    ) : null}

                    {formatRunRules.length > 0 ? (
                      <div
                        style={{
                          display: "grid",
                          gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
                          gap: 8,
                        }}
                      >
                        {formatRunRules.map((rule) => {
                          const checked = runRules.includes(rule.id);

                          return (
                            <label
                              key={rule.id}
                              style={{
                                ...styles.card,
                                padding: 10,
                                display: "flex",
                                gap: 8,
                                alignItems: "center",
                                cursor: "pointer",
                              }}
                            >
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={() => toggleRunRule(rule.id)}
                              />
                              <span>{ruleLabelRu(rule.id, rule.label)}</span>
                            </label>
                          );
                        })}
                      </div>
                    ) : null}
                  </div>

                  <div style={styles.grid2}>
                    <div>
                      <label>Размер комитета</label>
                      <input
                        style={styles.input}
                        type="number"
                        min={1}
                        value={runCommitteeSize}
                        onChange={(e) => setRunCommitteeSize(Number(e.target.value))}
                      />
                      <div style={{ marginTop: 8, ...styles.muted }}>
                        Внимание: даже если здесь указать значение больше 1, многие правила на практике возвращают одного победителя.
                        Это связано с природой конкретного алгоритма, а не с ошибкой интерфейса.
                      </div>
                    </div>

                    {selectedFormat === "approval" ? (
                      <div>
                        <label>Максимум одобрений</label>
                        <input
                          style={styles.input}
                          type="number"
                          min={1}
                          max={selected.candidates.length}
                          value={runApprovalMaxChoices}
                          onChange={(e) => setRunApprovalMaxChoices(Number(e.target.value))}
                        />
                        <div style={{ marginTop: 8, ...styles.muted }}>
                          {needsApprovalMaxChoices
                            ? "Выбранные правила требуют параметр максимального числа одобрений."
                            : "Параметр будет передан в эксперимент для approval-набора."}
                        </div>
                      </div>
                    ) : null}

                    {selectedFormat === "ranking" ? (
                      <div>
                        <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                          <input
                            type="checkbox"
                            checked={runRankingTopKEnabled}
                            onChange={(e) => setRunRankingTopKEnabled(e.target.checked)}
                          />
                          Ограничивать число учитываемых первых k позиций
                        </label>

                        <div style={{ marginTop: 8, ...styles.muted }}>
                          Поле необязательно. Если ограничение выключено, в эксперимент будет передано полное ранжирование.
                        </div>

                        {runRankingTopKEnabled ? (
                          <>
                            <div style={{ height: 10 }} />
                            <label>Ограничение на количество k позиций</label>
                            <input
                              style={styles.input}
                              type="number"
                              min={1}
                              max={selected.candidates.length}
                              value={runRankingTopK}
                              onChange={(e) => setRunRankingTopK(Number(e.target.value))}
                            />
                          </>
                        ) : null}
                      </div>
                    ) : null}

                    {selectedFormat === "score" ? (
                      <div>
                        <label>Диапазон оценок</label>
                        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8 }}>
                          <input
                            style={styles.input}
                            type="number"
                            value={runScoreMin}
                            onChange={(e) => setRunScoreMin(Number(e.target.value))}
                            aria-label="Минимальная оценка"
                          />
                          <input
                            style={styles.input}
                            type="number"
                            value={runScoreMax}
                            onChange={(e) => setRunScoreMax(Number(e.target.value))}
                            aria-label="Максимальная оценка"
                          />
                          <input
                            style={styles.input}
                            type="number"
                            min={1}
                            value={runScoreStep}
                            onChange={(e) => setRunScoreStep(Number(e.target.value))}
                            aria-label="Шаг оценки"
                          />
                        </div>
                        <div style={{ marginTop: 8, ...styles.muted }}>
                          {needsScoreRange
                            ? "Выбранные правила требуют диапазон оценок."
                            : "Диапазон будет передан в эксперимент для score-набора."}
                        </div>
                      </div>
                    ) : null}
                  </div>

                  <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                    <button
                      style={styles.btnPrimary}
                      onClick={handleCreateAndRunExperiments}
                      disabled={runLoading}
                    >
                      {runLoading ? "Создание и запуск…" : "Создать и запустить эксперименты"}
                    </button>

                    {runErr ? (
                      <div style={{ marginTop: 10, color: "#b91c1c", fontSize: 13 }}>{runErr}</div>
                    ) : null}

                    {createdRuns.length > 0 ? (
                      <button style={styles.btn} onClick={() => nav("/research/runs")}>
                        К запускам
                      </button>
                    ) : null}
                  </div>

                  {createdRuns.length > 0 ? (
                    <div style={{ display: "grid", gap: 8 }}>
                      <div style={{ fontWeight: 600 }}>Созданные запуски</div>

                      {createdRuns.map((item) => (
                        <div key={`${item.rule}-${item.runId}`} style={{ ...styles.card, padding: 10 }}>
                          <div>
                            <b>{ruleLabelRu(item.rule)}</b>
                          </div>
                          <div style={{ ...styles.muted, marginTop: 4 }}>
                            Запуск создан и передан в очередь выполнения
                          </div>

                          <details style={{ marginTop: 8 }}>
                            <summary style={{ cursor: "pointer", ...styles.muted }}>
                              Технические сведения
                            </summary>
                            <div style={{ marginTop: 8 }}>
                              <KeyValueList
                                items={[
                                  { label: "ID эксперимента", value: item.experimentId },
                                  { label: "ID запуска", value: item.runId },
                                  { label: "ID задачи", value: item.jobId || "—" },
                                  { label: "Правило", value: item.rule },
                                ]}
                              />
                            </div>
                          </details>
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
          </div>
        ) : (
          <div style={styles.muted}>Ничего не выбрано</div>
        )}
      </div>
    </div>
  );
}