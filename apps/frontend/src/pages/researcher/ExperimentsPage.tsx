import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { Experiment } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { Badge } from "../../shared/ui/Badge";
import { KeyValueList } from "../../shared/ui/KeyValueList";
import { SummaryGrid } from "../../shared/ui/SummaryGrid";
import { styles } from "../../shared/ui/styles";
import { formatDateTime } from "../../shared/utils/dateTime";
import { tallyRuleLabel } from "../../shared/utils/tallyRuleLabel";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function shortId(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw) return "—";
  return raw.length > 12 ? `${raw.slice(0, 8)}…${raw.slice(-4)}` : raw;
}

function experimentTypeLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    algo: "Алгоритмический эксперимент",
    behavior: "Поведенческий эксперимент",
  };

  return labels[raw] || "Эксперимент";
}

function statusLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    draft: "Черновик",
    queued: "В очереди",
    running: "Выполняется",
    done: "Завершен",
    error: "Ошибка",
  };

  return labels[raw] || raw || "Неизвестно";
}

function ballotFormatLabel(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";

  const labels: Record<string, string> = {
    approval: "Approval",
    ranking: "Ranking",
    score: "Score",
  };

  return labels[raw] || raw || "—";
}

function paramsObject(value: unknown): Record<string, unknown> {
  if (!value) return {};

  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      return isObject(parsed) ? parsed : {};
    } catch {
      return {};
    }
  }

  return isObject(value) ? value : {};
}

function numberOrDash(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? String(value) : "—";
}

function dateStartMs(value: string) {
  if (!value.trim()) return null;
  const ms = Date.parse(`${value}T00:00:00`);
  return Number.isFinite(ms) ? ms : null;
}

function dateEndMs(value: string) {
  if (!value.trim()) return null;
  const ms = Date.parse(`${value}T23:59:59.999`);
  return Number.isFinite(ms) ? ms : null;
}

function experimentTitle(item: Experiment) {
  const params = paramsObject(item.params);
  const rule = typeof params.tally_rule === "string" ? params.tally_rule : "";
  const format = typeof params.ballot_format === "string" ? params.ballot_format : "";

  if (rule && format) {
    return `${tallyRuleLabel(rule)} · ${ballotFormatLabel(format)}`;
  }

  if (rule) {
    return tallyRuleLabel(rule);
  }

  if (format) {
    return `${experimentTypeLabel(item.type)} · ${ballotFormatLabel(format)}`;
  }

  return experimentTypeLabel(item.type);
}

function experimentSubtitle(item: Experiment) {
  const params = paramsObject(item.params);
  const parts: string[] = [];

  if (typeof params.candidates === "number") {
    parts.push(`${params.candidates} кандидатов`);
  }

  if (typeof params.voters === "number") {
    parts.push(`${params.voters} избирателей`);
  }

  if (typeof params.committee_size === "number") {
    parts.push(`комитет ${params.committee_size}`);
  }

  if (typeof item.seed === "number") {
    parts.push(`seed ${item.seed}`);
  }

  return parts.length > 0 ? parts.join(" · ") : "Параметры эксперимента";
}

function formatParams(params: unknown) {
  const obj = paramsObject(params);
  const entries = Object.entries(obj);

  if (entries.length === 0) {
    return <span style={styles.muted}>Параметры не указаны</span>;
  }

  const labels: Record<string, string> = {
    ballot_format: "Формат бюллетеня",
    tally_rule: "Правило подсчета",
    committee_size: "Размер комитета",
    quota_type: "Квота",
    candidates: "Кандидаты",
    voters: "Избиратели",
    approval_max_choices: "Максимум одобрений",
    ranking_top_k: "Глубина ранжирования",
    score_min: "Минимальная оценка",
    score_max: "Максимальная оценка",
    score_step: "Шаг оценки",
    score_allow_skip: "Можно пропускать оценки",
  };

  return (
    <div style={{ display: "grid", gap: 6 }}>
      {entries.map(([key, value]) => {
        let renderedValue: React.ReactNode = typeof value === "string" ? value : JSON.stringify(value);

        if (key === "tally_rule" && typeof value === "string") {
          renderedValue = tallyRuleLabel(value);
        }

        if (key === "ballot_format") {
          renderedValue = ballotFormatLabel(value);
        }

        if (typeof value === "boolean") {
          renderedValue = value ? "Да" : "Нет";
        }

        return (
          <div key={key}>
            <b>{labels[key] || key}:</b> <span>{renderedValue}</span>
          </div>
        );
      })}
    </div>
  );
}

export function ExperimentsPage() {
  const { token, setToken, me } = useAuth();
  const nav = useNavigate();

  const [items, setItems] = useState<Experiment[]>([]);
  const [selected, setSelected] = useState<Experiment | null>(null);

  const [ruleFilter, setRuleFilter] = useState("");
  const [ballotFormatFilter, setBallotFormatFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [createdFrom, setCreatedFrom] = useState("");
  const [createdTo, setCreatedTo] = useState("");

  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);
  const detailAbortRef = useRef<AbortController | null>(null);

  const detailSectionRef = useRef<HTMLDivElement | null>(null);

  const scrollToExperimentDetail = useCallback(() => {
    window.requestAnimationFrame(() => {
      detailSectionRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
  }, []);

  const loadList = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const list = await api.experiments.list(token, ac.signal);
      setItems(list);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить список экспериментов");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, setToken]);

  useEffect(() => {
    loadList();
    return () => {
      abortRef.current?.abort();
      detailAbortRef.current?.abort();
    };
  }, [loadList]);

  const loadDetail = async (id: string) => {
    if (!token) return;

    detailAbortRef.current?.abort();
    const ac = new AbortController();
    detailAbortRef.current = ac;

    setDetailLoading(true);
    setErr(null);

    try {
      const item = await api.experiments.get(token, id, ac.signal);
      setSelected(item);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) setToken(null);
      setErr(e?.message || "Не удалось загрузить эксперимент");
      setSelected(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const openExperimentCard = async (id: string) => {
    await loadDetail(id);
    scrollToExperimentDetail();
  };

  const goToExperimentRuns = (experimentId: string) => {
    nav("/research/runs", {
      state: {
        experimentIdFilter: experimentId,
      },
    });
  };

  const ruleOptions = useMemo(() => {
    const map = new Map<string, string>();
  
    for (const item of items) {
      const params = paramsObject(item.params);
      const rule = typeof params.tally_rule === "string" ? params.tally_rule.trim() : "";
  
      if (rule) {
        map.set(rule, tallyRuleLabel(rule));
      }
    }
  
    return Array.from(map.entries())
      .map(([value, label]) => ({ value, label }))
      .sort((a, b) => a.label.localeCompare(b.label, "ru"));
  }, [items]);
  
  const filteredItems = useMemo(() => {
    const fromMs = dateStartMs(createdFrom);
    const toMs = dateEndMs(createdTo);
  
    return items.filter((item) => {
      const params = paramsObject(item.params);
  
      const rule = typeof params.tally_rule === "string" ? params.tally_rule.trim() : "";
      const format = typeof params.ballot_format === "string" ? params.ballot_format.trim() : "";
      const experimentType = typeof item.type === "string" ? item.type.trim() : "";
  
      if (ruleFilter && rule !== ruleFilter) return false;
      if (ballotFormatFilter && format !== ballotFormatFilter) return false;
      if (typeFilter && experimentType !== typeFilter) return false;
  
      const createdRaw = (item as any)?.created_at;
      const createdMs = typeof createdRaw === "string" ? Date.parse(createdRaw) : NaN;
  
      if (fromMs != null && (!Number.isFinite(createdMs) || createdMs < fromMs)) {
        return false;
      }
  
      if (toMs != null && (!Number.isFinite(createdMs) || createdMs > toMs)) {
        return false;
      }
  
      return true;
    });
  }, [items, ruleFilter, ballotFormatFilter, typeFilter, createdFrom, createdTo]);

  const selectedParams = useMemo(() => paramsObject(selected?.params), [selected]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline", flexWrap: "wrap" }}>
          <h2 style={{ margin: 0 }}>Эксперименты</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={loadList} disabled={loading}>
              Обновить
            </button>
            {me?.role === "researcher" ? (
              <Link to="/research/experiments/create" style={{ textDecoration: "none" }}>
                <button style={styles.btnPrimary}>Создать</button>
              </Link>
            ) : null}
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={{ marginTop: 12, ...styles.card, background: "#f9fafb" }}>
          <div style={{ fontWeight: 700, marginBottom: 10 }}>Фильтры</div>

          <div style={styles.grid2}>
            <div>
              <label>Правило подсчёта</label>
              <select
                style={styles.input}
                value={ruleFilter}
                onChange={(e) => setRuleFilter(e.target.value)}
              >
                <option value="">Все правила</option>
                {ruleOptions.map((item) => (
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
                value={ballotFormatFilter}
                onChange={(e) => setBallotFormatFilter(e.target.value)}
              >
                <option value="">Все форматы</option>
                <option value="approval">Одобрение</option>
                <option value="ranking">Ранжирование</option>
                <option value="score">Оценивание</option>
              </select>
            </div>

            <div>
              <label>Тип эксперимента</label>
              <select
                style={styles.input}
                value={typeFilter}
                onChange={(e) => setTypeFilter(e.target.value)}
              >
                <option value="">Все типы</option>
                <option value="algo">Алгоритмический</option>
                <option value="behavior">Поведенческий</option>
              </select>
            </div>

            <div>
              <label>Создан с</label>
              <input
                style={styles.input}
                type="date"
                value={createdFrom}
                onChange={(e) => setCreatedFrom(e.target.value)}
              />
            </div>

            <div>
              <label>Создан по</label>
              <input
                style={styles.input}
                type="date"
                value={createdTo}
                onChange={(e) => setCreatedTo(e.target.value)}
              />
            </div>

            <div style={{ display: "flex", alignItems: "end" }}>
              <button
                type="button"
                style={styles.btn}
                onClick={() => {
                  setRuleFilter("");
                  setBallotFormatFilter("");
                  setTypeFilter("");
                  setCreatedFrom("");
                  setCreatedTo("");
                }}
              >
                Сбросить фильтры
              </button>
            </div>
          </div>

          <div style={{ marginTop: 10, ...styles.muted }}>
            Показано: {filteredItems.length} из {items.length}
          </div>
        </div>

        <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
          {loading ? <div style={styles.muted}>Загрузка…</div> : null}
          {!loading && items.length === 0 ? <div style={styles.muted}>Список пуст</div> : null}
          {!loading && items.length > 0 && filteredItems.length === 0 ? (
            <div style={styles.muted}>По заданным фильтрам ничего не найдено</div>
          ) : null}

          {filteredItems.map((item) => (
            <div key={item.id} style={{ ...styles.card, padding: 12 }}>
              <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "flex-start", flexWrap: "wrap" }}>
                <div>
                  <div style={{ fontWeight: 800, fontSize: 16 }}>{experimentTitle(item)}</div>
                  <div style={{ ...styles.muted, marginTop: 4 }}>{experimentSubtitle(item)}</div>
                </div>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <Badge text={experimentTypeLabel(item.type)} />
                  <Badge text={statusLabel(item.status)} />
                </div>
              </div>

              <div style={{ marginTop: 10 }}>
                <SummaryGrid
                  items={[
                    { label: "Создан", value: formatDateTime(item.created_at) },
                    { label: "Формат", value: ballotFormatLabel(paramsObject(item.params).ballot_format) },
                    { label: "Правило", value: tallyRuleLabel(String(paramsObject(item.params).tally_rule || "")) },
                    { label: "Комитет", value: numberOrDash(paramsObject(item.params).committee_size) },
                  ]}
                />
              </div>

              <details style={{ marginTop: 10 }}>
                <summary style={{ cursor: "pointer", ...styles.muted }}>
                  Технические сведения
                </summary>
                <div style={{ marginTop: 8, display: "grid", gap: 4, ...styles.muted, fontSize: 12 }}>
                  <span>Идентификатор эксперимента: {item.id}</span>
                  <span>Владелец: {item.created_by}</span>
                  {item.seed != null ? <span>Seed: {item.seed}</span> : null}
                </div>
              </details>

              <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
                <button style={styles.btnPrimary} onClick={() => openExperimentCard(item.id)} disabled={detailLoading}>
                  Открыть карточку
                </button>
                <button style={styles.btn} onClick={() => goToExperimentRuns(item.id)}>
                  Запуски и результаты
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div ref={detailSectionRef} style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Карточка эксперимента</h3>
        {detailLoading ? <div style={styles.muted}>Загрузка…</div> : null}

        {selected ? (
          <div style={{ display: "grid", gap: 12 }}>
            <div>
              <div style={{ fontWeight: 800, fontSize: 18 }}>{experimentTitle(selected)}</div>
              <div style={{ ...styles.muted, marginTop: 4 }}>{experimentSubtitle(selected)}</div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={experimentTypeLabel(selected.type)} />
              <Badge text={statusLabel(selected.status)} />
              {selected.seed != null ? <Badge text={`seed: ${selected.seed}`} /> : null}
            </div>

            <SummaryGrid
              items={[
                { label: "Создан", value: formatDateTime(selected.created_at) },
                { label: "Формат бюллетеня", value: ballotFormatLabel(selectedParams.ballot_format) },
                { label: "Правило подсчета", value: tallyRuleLabel(String(selectedParams.tally_rule || "")) },
                { label: "Размер комитета", value: numberOrDash(selectedParams.committee_size) },
                { label: "Кандидаты", value: numberOrDash(selectedParams.candidates) },
                { label: "Избиратели", value: numberOrDash(selectedParams.voters) },
              ]}
            />

            <div>
              <h4 style={{ marginBottom: 8 }}>Параметры</h4>
              {formatParams(selected.params)}
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
              <button style={styles.btnPrimary} onClick={() => goToExperimentRuns(selected.id)}>
                Открыть запуски и результаты
              </button>
            </div>

            <details>
              <summary style={{ cursor: "pointer", ...styles.muted }}>
                Технические сведения
              </summary>
              <div style={{ marginTop: 10 }}>
                <KeyValueList
                  items={[
                    { label: "ID эксперимента", value: selected.id },
                    { label: "Короткий ID", value: shortId(selected.id) },
                    { label: "Владелец", value: selected.created_by },
                    { label: "Создан", value: formatDateTime(selected.created_at) },
                  ]}
                />
              </div>
            </details>
          </div>
        ) : (
          <div style={styles.muted}>Выберите эксперимент из списка</div>
        )}
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