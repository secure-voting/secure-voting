import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api } from "../../shared/api/client";
import type { BallotMeta, MyBallotResp } from "../../shared/api/types";
import { useAuth } from "../../app/auth";
import { useNotifications } from "../../app/notifications";
import { Badge } from "../../shared/ui/Badge";
import { ErrorBanner } from "../../shared/ui/ErrorBanner";
import { JsonBlock } from "../../shared/ui/JsonBlock";
import { styles } from "../../shared/ui/styles";

const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function helperText(meta: BallotMeta | null) {
  if (!meta) return "";
  if (meta.ballot_format === "approval") {
    if (meta.approval_max_choices != null) {
      return `Выберите не более ${meta.approval_max_choices} кандидатов`;
    }
    return "Выберите поддерживаемых кандидатов";
  }
  if (meta.ballot_format === "ranking") {
    if (meta.ranking_top_k != null) {
      return `Распределите кандидатов по позициям от 1 до ${meta.ranking_top_k}. Можно заполнить только первые места.`;
    }
    return "Распределите кандидатов по позициям в порядке предпочтения";
  }
  if (meta.ballot_format === "score") {
    return `Допустимый диапазон оценок: ${meta.score_min ?? "?"}..${meta.score_max ?? "?"}, шаг ${meta.score_step ?? "?"}`;
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
  const [submitResp, setSubmitResp] = useState<unknown>(null);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    if (!token) return;

    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);
    setSubmitResp(null);

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
      if (e?.status === 401) setToken(null);
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
    if (!meta) return "Нет метаданных бюллетеня";

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
        return "Превышен top-k";
      }

      const uniq = new Set(ranking);
      if (uniq.size !== ranking.length) {
        return "В ранжировании есть повторы";
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
        return "Некорректные параметры оценки";
      }

      for (const c of meta.candidates) {
        const v = scores[c.id];
        if (v === undefined || v === null) {
          if (meta.score_allow_skip) continue;
          return "Заполните все оценки";
        }
        if (!Number.isFinite(v)) return "Некорректное значение оценки";
        if (v < min || v > max) return "Оценка вне диапазона";
        if ((v - min) % step !== 0) return "Оценка не соответствует шагу";
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
    setSubmitResp(null);

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

      const resp = await api.ballots.submit(
        token,
        electionId,
        body,
        api.ballots.newIdempotencyKey()
      );

      if (IS_DEV) setSubmitResp(resp);

      addNotification({
        kind: "success",
        title: "Голос отправлен",
        message: `Бюллетень для голосования ${electionId} успешно зафиксирован`,
      });

      await reload();
    } catch (e: any) {
      if (e?.status === 401) setToken(null);

      if (e?.code === "already_submitted") {
        setErr("Голос уже отправлен");
      } else if (e?.code === "not_active") {
        setErr("Голосование сейчас недоступно");
      } else if (e?.code === "idempotency_in_progress") {
        setErr("Запрос уже обрабатывается, попробуйте ещё раз");
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
            <Badge text={`status: ${my.status}`} />
            {my.submitted_at ? <span style={styles.muted}>submitted_at: {my.submitted_at}</span> : null}
            {my.updated_at ? <span style={styles.muted}>updated_at: {my.updated_at}</span> : null}
          </div>
        ) : null}

        {meta ? (
          <>
            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`format: ${meta.ballot_format}`} />
              <Badge text={`rule: ${meta.tally_rule}`} />
              {meta.ballot_format === "approval" && meta.approval_max_choices != null ? (
                <Badge text={`max: ${meta.approval_max_choices}`} />
              ) : null}
              {meta.ballot_format === "ranking" && meta.ranking_top_k != null ? (
                <Badge text={`top-k: ${meta.ranking_top_k}`} />
              ) : null}
              {meta.ballot_format === "score" ? (
                <Badge
                  text={`score: ${meta.score_min ?? "?"}..${meta.score_max ?? "?"} step ${meta.score_step ?? "?"} skip ${String(meta.score_allow_skip)}`}
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
                <h3 style={{ marginTop: 0 }}>Одобрительный бюллетень</h3>
                {approvalRemaining != null ? (
                  <div style={styles.muted}>Осталось доступных отметок: {approvalRemaining}</div>
                ) : null}

                <div style={{ marginTop: 10, display: "grid", gap: 8 }}>
                  {meta.candidates.map((c) => {
                    const checked = approvalSet.includes(c.id);
                    const max = meta.approval_max_choices ?? null;
                    const disabled =
                      !checked && max != null && max > 0 && approvalSet.length >= max;

                    return (
                      <label
                        key={c.id}
                        style={{
                          display: "flex",
                          gap: 10,
                          alignItems: "center",
                          padding: 10,
                          border: "1px solid #e5e7eb",
                          borderRadius: 12,
                          cursor: disabled ? "not-allowed" : "pointer",
                          userSelect: "none",
                          opacity: disabled ? 0.6 : 1,
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          disabled={disabled}
                          onChange={() => toggleApproval(c.id)}
                        />
                        <div style={{ flex: 1 }}>
                          <b>{c.name}</b>
                          <div style={styles.muted}>{c.id}</div>
                        </div>
                      </label>
                    );
                  })}
                </div>
              </>
            ) : null}

            {meta.ballot_format === "ranking" ? (
              <>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 10,
                    alignItems: "baseline",
                    flexWrap: "wrap",
                  }}
                >
                  <div>
                    <h3 style={{ marginTop: 0, marginBottom: 6 }}>Ранжированный бюллетень</h3>
                    {rankingRemaining != null ? (
                      <div style={styles.muted}>Осталось незаполненных позиций: {rankingRemaining}</div>
                    ) : null}
                  </div>

                  <button style={styles.btn} onClick={clearRankingAll} type="button">
                    Очистить всё
                  </button>
                </div>

                <div style={{ marginTop: 10, display: "grid", gap: 10 }}>
                  {rankingSlots.map((candidateId, index) => {
                    const currentValue = candidateId || "";
                    const options = meta.candidates.filter((candidate) => {
                      if (candidate.id === currentValue) return true;
                      return !usedRankingCandidateIds.has(candidate.id);
                    });

                    return (
                      <div
                        key={`rank-slot-${index}`}
                        style={{ ...styles.card, padding: 12, background: "#f9fafb" }}
                      >
                        <div
                          style={{
                            display: "grid",
                            gridTemplateColumns: "110px 1fr auto",
                            gap: 10,
                            alignItems: "center",
                          }}
                        >
                          <div>
                            <b>Место #{index + 1}</b>
                          </div>

                          <select
                            style={styles.input}
                            value={currentValue}
                            onChange={(e) => setRankingSlotValue(index, e.target.value)}
                          >
                            <option value="">Не выбрано</option>
                            {options.map((candidate) => (
                              <option key={candidate.id} value={candidate.id}>
                                {candidate.name}
                              </option>
                            ))}
                          </select>

                          <button
                            style={styles.btn}
                            type="button"
                            onClick={() => clearRankingSlot(index)}
                            disabled={!currentValue}
                          >
                            Сбросить
                          </button>
                        </div>

                        {currentValue ? (
                          <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                            Назначен кандидат:{" "}
                            {meta.candidates.find((c) => c.id === currentValue)?.name ?? currentValue}
                          </div>
                        ) : (
                          <div style={{ marginTop: 8, ...styles.muted, fontSize: 12 }}>
                            Эта позиция пока не заполнена
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>

                <div style={{ marginTop: 12 }}>
                  <h4 style={{ margin: "0 0 8px 0" }}>Кандидаты и занятые позиции</h4>
                  <div style={{ display: "grid", gap: 8 }}>
                    {meta.candidates.map((candidate) => {
                      const rankNo = assignedRankingMap.get(candidate.id);

                      return (
                        <div
                          key={candidate.id}
                          style={{
                            ...styles.card,
                            padding: 10,
                            display: "flex",
                            justifyContent: "space-between",
                            gap: 10,
                            alignItems: "center",
                          }}
                        >
                          <div>
                            <b>{candidate.name}</b>
                            <div style={styles.muted}>{candidate.id}</div>
                          </div>

                          {rankNo ? (
                            <Badge text={`место ${rankNo}`} />
                          ) : (
                            <span style={styles.muted}>не назначен</span>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              </>
            ) : null}

            {meta.ballot_format === "score" ? (
              <>
                <h3 style={{ marginTop: 0 }}>Оценочный бюллетень</h3>
                <div style={styles.muted}>
                  Выставьте оценки кандидатам в допустимом диапазоне
                </div>

                <div style={{ marginTop: 10, display: "grid", gap: 10 }}>
                  {meta.candidates.map((c) => {
                    const v = scores[c.id];
                    const min = meta.score_min ?? 0;
                    const max = meta.score_max ?? 10;
                    const step = meta.score_step ?? 1;
                    const missing =
                      !meta.score_allow_skip && (v === undefined || v === null);

                    return (
                      <div
                        key={c.id}
                        style={{
                          ...styles.card,
                          padding: 12,
                          borderColor: missing ? "#fecaca" : "#e5e7eb",
                          background: missing ? "#fff1f2" : "white",
                        }}
                      >
                        <div
                          style={{
                            display: "flex",
                            justifyContent: "space-between",
                            gap: 10,
                            alignItems: "baseline",
                          }}
                        >
                          <div>
                            <b>{c.name}</b>
                            <div style={styles.muted}>{c.id}</div>
                          </div>

                          <div style={{ width: 220 }}>
                            <input
                              style={styles.input}
                              type="number"
                              min={min}
                              max={max}
                              step={step}
                              value={v ?? ""}
                              onChange={(e) => {
                                const raw = e.target.value;
                                if (raw.trim() === "") {
                                  if (meta.score_allow_skip) {
                                    setScores((prev) => {
                                      const next = { ...prev };
                                      delete next[c.id];
                                      return next;
                                    });
                                  }
                                  return;
                                }

                                const num = Number(raw);
                                if (Number.isFinite(num)) setScore(c.id, num);
                              }}
                            />
                          </div>
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
                  : "Голос уже учтён"}
            </button>

            {!canSubmit ? (
              <div style={{ marginTop: 8, ...styles.muted }}>
                Повторная отправка для данного пользователя недоступна
              </div>
            ) : null}

            {IS_DEV && submitResp ? (
              <>
                <hr style={styles.hr} />
                <h3 style={{ marginTop: 0 }}>Submit response</h3>
                <JsonBlock value={submitResp} />
              </>
            ) : null}
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug</h3>
          <div style={{ display: "grid", gap: 10 }}>
            <div>
              <div style={styles.muted}>Ballot meta</div>
              {meta ? <JsonBlock value={meta} /> : <div style={styles.muted}>Empty</div>}
            </div>
            <div>
              <div style={styles.muted}>My ballot</div>
              {my ? <JsonBlock value={my} /> : <div style={styles.muted}>Empty</div>}
            </div>
            {meta?.ballot_format === "ranking" ? (
              <div>
                <div style={styles.muted}>Ranking slots</div>
                <JsonBlock value={rankingSlots} />
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}