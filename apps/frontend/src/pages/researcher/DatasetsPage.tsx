import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import type {
  DatasetDetail,
  DatasetGenerateReq,
  DatasetListItem,
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

type CreatedSyntheticRun = {
  rule: string;
  experimentId: string;
  runId: string;
  jobId: string;
};

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
  const label = tallyRuleLabel(id);
  return label !== "—" ? label : fallback || id;
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
  const [runRules, setRunRules] = useState<string[]>(["plurality", "borda", "black"]);
  const [runCommitteeSize, setRunCommitteeSize] = useState(1);
  const [runRankingTopK, setRunRankingTopK] = useState(3);
  const [runLoading, setRunLoading] = useState(false);
  const [createdRuns, setCreatedRuns] = useState<CreatedSyntheticRun[]>([]);
  const [availableRunRules, setAvailableRunRules] = useState<TallyRuleInfo[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);

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
    setRulesLoading(true);

    api.capabilities
      .tallyRules(token, ac.signal)
      .then((items) => {
        const mergedItems = mergeRuleItems(items);
        const rankingExperimentRules = mergedItems.filter(
          (item) =>
            item.supports_experiment_runs &&
            item.ballot_formats.includes("ranking") &&
            !item.requires_approval_max_choices &&
            !item.requires_score_range
        );

        setAvailableRunRules(rankingExperimentRules);

        setRunRules((prev) => {
          const allowed = new Set(rankingExperimentRules.map((item) => item.id));
          const next = prev.filter((item) => allowed.has(item));
          if (next.length > 0) return next;
          return rankingExperimentRules.slice(0, 3).map((item) => item.id);
        });
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

    const ids = new Set(parsed.map((x) => x.id));
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

      if (genFormat === "ranking") {
        setRunRankingTopKEnabled(genRankingTopKEnabled);
        setRunRankingTopK(genRankingTopK);
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

    if (selected.format !== "ranking") {
      setRunErr("Серия экспериментов из этой формы сейчас поддерживается только для ранжированных наборов");
      return;
    }

    if (rulesLoading) {
      setRunErr("Список правил для экспериментов еще загружается");
      return;
    }

    if (availableRunRules.length === 0) {
      setRunErr("Нет доступных правил для ranking-экспериментов");
      return;
    }

    const allowedRuleIds = new Set(availableRunRules.map((item) => item.id));
    if (runRules.some((rule) => !allowedRuleIds.has(rule))) {
      setRunErr("Выбрано недопустимое правило подсчета");
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

    if (runRankingTopKEnabled) {
      if (runRankingTopK < 1) {
        setRunErr("Ограничение top-k должно быть не меньше 1");
        return;
      }

      if (runRankingTopK > selected.candidates.length) {
        setRunErr("Ограничение top-k не может превышать число кандидатов");
        return;
      }
    }

    setRunLoading(true);
    setCreatedRuns([]);

    try {
      const created: CreatedSyntheticRun[] = [];

      for (const rule of runRules) {
        const params: Record<string, unknown> = {
          ballot_format: "ranking",
          tally_rule: rule,
          committee_size: runCommitteeSize,
        };

        if (runRankingTopKEnabled) {
          params.ranking_top_k = runRankingTopK;
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
          throw new Error(`Не удалось получить run_id для правила ${ruleLabelRu(rule)}`);
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
            value={genVoters}
            onChange={(e) => setGenVoters(Number(e.target.value))}
          />

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

            {selected.format === "ranking" ? (
              <div style={{ ...styles.card, padding: 12 }}>
                <h4 style={{ marginTop: 0, marginBottom: 8 }}>Создание и запуск экспериментов</h4>

                <div style={{ ...styles.muted, marginBottom: 12 }}>
                  Для ранжированного набора данных можно сразу создать и запустить серию экспериментов.
                </div>

                <div style={{ display: "grid", gap: 12 }}>
                  <div>
                  <div style={{ fontWeight: 600, marginBottom: 8 }}>Правила подсчёта</div>

                    {rulesLoading ? (
                      <div style={styles.muted}>Загрузка списка правил…</div>
                    ) : null}

                    {!rulesLoading && availableRunRules.length === 0 ? (
                      <div style={styles.muted}>Нет доступных правил для ranking-экспериментов</div>
                    ) : null}

                    {availableRunRules.length > 0 ? (
                      <div
                        style={{
                          display: "grid",
                          gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
                          gap: 8,
                        }}
                      >
                        {availableRunRules.map((rule) => {
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