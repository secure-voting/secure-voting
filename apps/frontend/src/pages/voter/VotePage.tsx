import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { BallotMeta, MyBallotResp } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { styles } from "../../shared/ui/styles";

function helperText(meta: BallotMeta | null) {
  if (!meta) return "";

  if (meta.ballot_format === "approval") {
    if (meta.approval_max_choices != null) {
      return `Выберите не более ${meta.approval_max_choices} кандидатов.`;
    }
    return "Выберите поддерживаемых кандидатов.";
  }

  if (meta.ballot_format === "ranking") {
    if (meta.ranking_top_k != null) {
      return `Распределите кандидатов по позициям от 1 до ${meta.ranking_top_k} без пропусков между заполненными местами.`;
    }
    return "Распределите кандидатов по позициям в порядке предпочтения.";
  }

  if (meta.ballot_format === "score") {
    return `Допустимый диапазон оценок: ${meta.score_min ?? "?"}..${meta.score_max ?? "?"}, шаг ${meta.score_step ?? "?"}.`;
  }

  return "";
}

function buildInitialRankingSlots(meta: BallotMeta): string[] {
  const slotsCount =
    meta.ranking_top_k && meta.ranking_top_k > 0
      ? meta.ranking_top_k
      : meta.candidates.length;

  return Array.from({ length: slotsCount }, () => "");
}

function ballotFormatLabel(value?: string) {
  switch (value) {
    case "approval":
      return "Одобрение";
    case "ranking":
      return "Ранжирование";
    case "score":
      return "Оценивание";
    default:
      return value || "—";
  }
}

function ballotStatusLabel(value?: string) {
  switch (value) {
    case "accepted":
      return "Голос учтен";
    case "draft":
      return "Черновик";
    case "rejected":
      return "Отклонен";
    default:
      return value || "—";
  }
}

function ruleLabel(value?: string) {
  return value || "—";
}

function candidateDescription(meta?: Record<string, unknown> | null) {
  if (!meta || typeof meta !== "object") return "";
  const value = meta.description;
  return typeof value === "string" ? value : "";
}

export function VotePage() {
  const { id } = useParams();
  const electionId = String(id || "");
  const { token, setToken } = useAuth();
  const { addNotification } = useNotifications();

  const [meta, setMeta] = useState<BallotMeta | null>(null);
  const [my, setMy] = useState<MyBallotResp | null>(null);

  const [approvalSet, setApprovalSet] = useState<string[]>([]);
  const [rankingSlots, setRankingSlots] = useState<string[]>([]);
  const [scores, setScores] = useState<Record<string, number>>({});

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const m = await api.elections.ballotMeta(token, electionId, ac.signal);
      const mb = await api.ballots.me(token, electionId, ac.signal);

      setMeta(m);
      setMy(mb);

      if (m.ballot_format === "ranking") {
        setRankingSlots(buildInitialRankingSlots(m));
      } else {
        setRankingSlots([]);
      }
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) {
        setToken(null);
        return;
      }
      setErr(e?.message || "Не удалось загрузить бюллетень");
      setMeta(null);
      setMy(null);
      setRankingSlots([]);
    } finally {
      setLoading(false);
    }
  }, [token, electionId, setToken]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  useEffect(() => {
    if (!meta) return;
    if (meta.ballot_format !== "score") return;
    if (meta.score_min == null) return;

    setScores((prev) => {
      if (Object.keys(prev).length > 0) return prev;
      const next: Record<string, number> = {};
      for (const c of meta.candidates) next[c.id] = meta.score_min as number;
      return next;
    });
  }, [meta]);

  const toggleApproval = (cid: string) => {
    if (!meta) return;
    const max = meta.approval_max_choices ?? null;

    setApprovalSet((prev) => {
      const has = prev.includes(cid);
      if (has) return prev.filter((x) => x !== cid);
      if (max != null && max > 0 && prev.length >= max) return prev;
      return [...prev, cid];
    });
  };

  const setRankingSlotValue = (slotIndex: number, candidateId: string) => {
    setRankingSlots((prev) => {
      const next = [...prev];

      if (!candidateId) {
        next[slotIndex] = "";
        return next;
      }

      for (let i = 0; i < next.length; i += 1) {
        if (i !== slotIndex && next[i] === candidateId) {
          next[i] = "";
        }
      }

      next[slotIndex] = candidateId;
      return next;
    });
  };

  const clearRankingSlot = (slotIndex: number) => {
    setRankingSlots((prev) => {
      const next = [...prev];
      next[slotIndex] = "";
      return next;
    });
  };

  const clearRankingAll = () => {
    setRankingSlots((prev) => prev.map(() => ""));
  };

  const setScore = (cid: string, v: number) => {
    setScores((prev) => ({ ...prev, [cid]: v }));
  };

  const ranking = useMemo(() => rankingSlots.filter(Boolean), [rankingSlots]);

  const assignedRankingMap = useMemo(() => {
    const map = new Map<string, number>();
    rankingSlots.forEach((candidateId, index) => {
      if (candidateId) map.set(candidateId, index + 1);
    });
    return map;
  }, [rankingSlots]);

  const validateBeforeSubmit = (): string | null => {
    if (!meta) return "Нет данных бюллетеня";

    if (meta.ballot_format === "approval") {
      if (approvalSet.length === 0) return "Выберите хотя бы одного кандидата";
      const max = meta.approval_max_choices ?? null;
      if (max != null && max > 0 && approvalSet.length > max) return "Превышен лимит выбора";
      return null;
    }

    if (meta.ballot_format === "ranking") {
      if (ranking.length === 0) return "Заполните хотя бы одну позицию ранжирования";

      const topK = meta.ranking_top_k ?? null;
      if (topK != null && topK > 0 && ranking.length > topK) {
        return "Превышено допустимое число позиций";
      }

      const uniq = new Set(ranking);
      if (uniq.size !== ranking.length) {
        return "В ранжировании есть повторяющиеся кандидаты";
      }

      for (let i = 0; i < rankingSlots.length; i += 1) {
        if (!rankingSlots[i]) {
          const hasFilledAfter = rankingSlots.slice(i + 1).some(Boolean);
          if (hasFilledAfter) {
            return "Позиции ранжирования должны заполняться сверху вниз без пропусков";
          }
        }
      }

      return null;
    }

    if (meta.ballot_format === "score") {
      const min = meta.score_min;
      const max = meta.score_max;
      const step = meta.score_step;

      if (min == null || max == null || step == null || step <= 0) {
        return "Некорректные параметры оценивания";
      }

      for (const c of meta.candidates) {
        const v = scores[c.id];
        if (v === undefined || v === null) {
          if (meta.score_allow_skip) continue;
          return "Заполните все оценки";
        }
        if (!Number.isFinite(v)) return "Некорректное значение оценки";
        if (v < min || v > max) return "Оценка вне допустимого диапазона";
        if ((v - min) % step !== 0) return "Оценка не соответствует заданному шагу";
      }

      return null;
    }

    return "Неизвестный формат бюллетеня";
  };

  const submit = async () => {
    if (!token || !meta) return;

    const validationError = validateBeforeSubmit();
    if (validationError) {
      setErr(validationError);
      return;
    }

    setLoading(true);
    setErr(null);

    try {
      const body: Record<string, unknown> = {};

      if (meta.ballot_format === "approval") {
        body.approval_set = approvalSet;
      }

      if (meta.ballot_format === "ranking") {
        body.ranking = ranking;
      }

      if (meta.ballot_format === "score") {
        const out: Record<string, number> = {};
        for (const c of meta.candidates) {
          const val = scores[c.id];
          if (val === undefined || val === null) continue;
          out[c.id] = val;
        }
        body.scores = out;
      }

      await api.ballots.submit(
        token,
        electionId,
        body,
        api.ballots.newIdempotencyKey()
      );

      addNotification({
        kind: "success",
        title: "Голос отправлен",
        message: `Бюллетень для голосования ${electionId} успешно зафиксирован`,
      });

      await reload();
    } catch (e: any) {
      if (e?.status === 401) {
        setToken(null);
        return;
      }

      if (e?.code === "already_submitted") {
        setErr("Голос уже отправлен");
      } else if (e?.code === "election_not_active" || e?.code === "not_active") {
        setErr("Голосование сейчас недоступно");
      } else if (e?.code === "idempotency_in_progress") {
        setErr("Запрос уже обрабатывается, попробуйте еще раз");
      } else {
        setErr(e?.message || "Не удалось отправить бюллетень");
      }
    } finally {
      setLoading(false);
    }
  };

  const canSubmit = my?.status !== "accepted";
  const description = helperText(meta);

  const approvalRemaining = useMemo(() => {
    if (!meta || meta.ballot_format !== "approval") return null;
    if (meta.approval_max_choices == null) return null;
    return Math.max(0, meta.approval_max_choices - approvalSet.length);
  }, [meta, approvalSet.length]);

  const rankingRemaining = useMemo(() => {
    if (!meta || meta.ballot_format !== "ranking") return null;
    const topK =
      meta.ranking_top_k && meta.ranking_top_k > 0
        ? meta.ranking_top_k
        : meta.candidates.length;
    return Math.max(0, topK - ranking.length);
  }, [meta, ranking.length]);

  const usedRankingCandidateIds = useMemo(
    () => new Set(rankingSlots.filter(Boolean)),
    [rankingSlots]
  );

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
          <h2 style={{ margin: 0 }}>Заполнение бюллетеня</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <Link to={`/elections/${electionId}`} style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Карточка</button>
            </Link>
            <Link to="/elections" style={{ textDecoration: "none" }}>
              <button style={styles.btn}>Назад</button>
            </Link>
            <button style={styles.btn} onClick={reload} disabled={loading}>
              Обновить
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Загрузка…</div> : null}

        {my ? (
          <div style={{ marginTop: 10, display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
            <Badge text={`Состояние: ${ballotStatusLabel(my.status)}`} />
            {my.submitted_at ? <span style={styles.muted}>Отправлено: {my.submitted_at}</span> : null}
            {my.updated_at ? <span style={styles.muted}>Обновлено: {my.updated_at}</span> : null}
          </div>
        ) : null}

        {meta ? (
          <>
            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`Формат: ${ballotFormatLabel(meta.ballot_format)}`} />
              <Badge text={`Правило: ${ruleLabel(meta.tally_rule)}`} />
              {meta.ballot_format === "approval" && meta.approval_max_choices != null ? (
                <Badge text={`Максимум выборов: ${meta.approval_max_choices}`} />
              ) : null}
              {meta.ballot_format === "ranking" && meta.ranking_top_k != null ? (
                <Badge text={`Максимум позиций: ${meta.ranking_top_k}`} />
              ) : null}
              {meta.ballot_format === "score" ? (
                <Badge
                  text={`Оценки: ${meta.score_min ?? "?"}..${meta.score_max ?? "?"}, шаг ${meta.score_step ?? "?"}, пропуск ${meta.score_allow_skip ? "разрешен" : "запрещен"}`}
                />
              ) : null}
            </div>

            {description ? (
              <div style={{ marginTop: 10, ...styles.card, background: "#f9fafb" }}>
                {description}
              </div>
            ) : null}

            <hr style={styles.hr} />

            {meta.ballot_format === "approval" ? (
              <>
                <div style={{ marginBottom: 10 }}>
                  <b>Выбор кандидатов</b>
                  {approvalRemaining != null ? (
                    <div style={{ ...styles.muted, marginTop: 4 }}>
                      Осталось отметить: {approvalRemaining}
                    </div>
                  ) : null}
                </div>

                <div style={{ display: "grid", gap: 8 }}>
                  {meta.candidates.map((c) => {
                    const checked = approvalSet.includes(c.id);
                    return (
                      <label
                        key={c.id}
                        style={{
                          ...styles.card,
                          padding: 10,
                          display: "flex",
                          gap: 10,
                          alignItems: "flex-start",
                          cursor: "pointer",
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleApproval(c.id)}
                          disabled={loading || !canSubmit}
                        />
                        <div>
                          <div style={{ fontWeight: 700 }}>{c.name}</div>
                          {candidateDescription(c.meta) ? (
                            <div style={{ marginTop: 4, ...styles.muted }}>{candidateDescription(c.meta)}</div>
                          ) : null}
                        </div>
                      </label>
                    );
                  })}
                </div>
              </>
            ) : null}

            {meta.ballot_format === "ranking" ? (
              <>
                <div style={{ marginBottom: 10, display: "flex", justifyContent: "space-between", gap: 8, flexWrap: "wrap" }}>
                  <div>
                    <b>Ранжирование кандидатов</b>
                    {rankingRemaining != null ? (
                      <div style={{ ...styles.muted, marginTop: 4 }}>
                        Осталось заполнить позиций: {rankingRemaining}
                      </div>
                    ) : null}
                  </div>

                  <button style={styles.btn} type="button" onClick={clearRankingAll} disabled={loading || !canSubmit}>
                    Очистить все
                  </button>
                </div>

                <div style={{ display: "grid", gap: 10 }}>
                  {rankingSlots.map((candidateId, index) => (
                    <div key={`slot-${index}`} style={{ ...styles.card, padding: 12 }}>
                      <div style={{ marginBottom: 8, fontWeight: 700 }}>Позиция {index + 1}</div>

                      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                        <select
                          style={styles.input}
                          value={candidateId}
                          onChange={(e) => setRankingSlotValue(index, e.target.value)}
                          disabled={loading || !canSubmit}
                        >
                          <option value="">Не выбрано</option>
                          {meta.candidates.map((candidate) => {
                            const isUsedElsewhere =
                              usedRankingCandidateIds.has(candidate.id) && candidateId !== candidate.id;

                            return (
                              <option
                                key={candidate.id}
                                value={candidate.id}
                                disabled={isUsedElsewhere}
                              >
                                {candidate.name}
                              </option>
                            );
                          })}
                        </select>

                        <button
                          type="button"
                          style={styles.btn}
                          onClick={() => clearRankingSlot(index)}
                          disabled={loading || !canSubmit || !candidateId}
                        >
                          Очистить
                        </button>
                      </div>

                      {candidateId ? (
                        <div style={{ marginTop: 8, ...styles.muted }}>
                          Назначен кандидат:{" "}
                          {meta.candidates.find((c) => c.id === candidateId)?.name || candidateId}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>

                <div style={{ marginTop: 12, display: "grid", gap: 8 }}>
                  <b>Текущее распределение мест</b>
                  <div style={{ display: "grid", gap: 6 }}>
                    {meta.candidates.map((candidate) => {
                      const place = assignedRankingMap.get(candidate.id);
                      return (
                        <div key={candidate.id} style={{ ...styles.card, padding: 10 }}>
                          <div style={{ fontWeight: 700 }}>{candidate.name}</div>
                          <div style={{ ...styles.muted, marginTop: 4 }}>
                            {place ? `Назначено место: ${place}` : "Пока не назначено"}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </>
            ) : null}

            {meta.ballot_format === "score" ? (
              <>
                <div style={{ marginBottom: 10 }}>
                  <b>Оценивание кандидатов</b>
                </div>

                <div style={{ display: "grid", gap: 8 }}>
                  {meta.candidates.map((c) => {
                    const value = scores[c.id];
                    return (
                      <div key={c.id} style={{ ...styles.card, padding: 12 }}>
                        <div style={{ fontWeight: 700 }}>{c.name}</div>
                        {candidateDescription(c.meta) ? (
                          <div style={{ marginTop: 4, ...styles.muted }}>{candidateDescription(c.meta)}</div>
                        ) : null}

                        <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
                          <input
                            style={{ ...styles.input, maxWidth: 160 }}
                            type="number"
                            value={value ?? ""}
                            min={meta.score_min ?? undefined}
                            max={meta.score_max ?? undefined}
                            step={meta.score_step ?? undefined}
                            disabled={loading || !canSubmit}
                            onChange={(e) => {
                              const raw = e.target.value;
                              if (raw === "") return;
                              const num = Number(raw);
                              if (Number.isFinite(num)) setScore(c.id, num);
                            }}
                          />
                          <span style={styles.muted}>
                            Диапазон: {meta.score_min ?? "?"}..{meta.score_max ?? "?"}, шаг {meta.score_step ?? "?"}
                          </span>
                        </div>

                        {meta.score_allow_skip ? (
                          <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                            Допускается пропуск оценки
                          </div>
                        ) : null}
                      </div>
                    );
                  })}
                </div>
              </>
            ) : null}

            <hr style={styles.hr} />

            <button
              style={styles.btnPrimary}
              onClick={submit}
              disabled={loading || !canSubmit}
            >
              {loading
                ? "Отправка…"
                : canSubmit
                  ? "Отправить бюллетень"
                  : "Голос уже учтен"}
            </button>

            {!canSubmit ? (
              <div style={{ marginTop: 8, display: "grid", gap: 6 }}>
                <div style={{ color: "#15803d", fontWeight: 600 }}>
                  Голос учтен
                </div>
                <div style={styles.muted}>
                  Повторная отправка для данного пользователя недоступна
                </div>
              </div>
            ) : null}
          </>
        ) : null}
      </div>
    </div>
  );
}