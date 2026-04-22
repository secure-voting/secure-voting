import React from "react";
import { styles } from "./styles";

type ProtocolEntry = {
  key: string;
  title: string;
  subtitle?: string;
  details?: Array<{ label: string; value: string }>;
};

function safeStringify(value: unknown) {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function compact(value: unknown): string {
  if (value == null) return "—";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return safeStringify(value);
}

function pickRecordArray(value: Record<string, unknown>, keys: string[]): unknown[] | null {
  for (const key of keys) {
    const candidate = value[key];
    if (Array.isArray(candidate)) return candidate;
  }
  return null;
}

function normalizeScoreArray(value: unknown): string {
  if (!Array.isArray(value)) return compact(value);

  return value
    .map((item) => {
      if (!isObject(item)) return compact(item);
      const candidateName =
        typeof item.candidate_name === "string" && item.candidate_name.trim()
          ? item.candidate_name.trim()
          : typeof item.candidate_id === "string" && item.candidate_id.trim()
            ? item.candidate_id.trim()
            : "Кандидат";
      const score = item.value ?? item.score ?? item.points ?? item.count;
      return `${candidateName}: ${compact(score)}`;
    })
    .join(" · ");
}

function normalizeStepObject(obj: Record<string, unknown>, fallbackIndex: number): ProtocolEntry {
  const stepNo =
    obj.round ??
    obj.step ??
    obj.iteration ??
    obj.index ??
    obj.order ??
    fallbackIndex + 1;

  const action =
    obj.action ??
    obj.event ??
    obj.phase ??
    obj.status ??
    obj.kind ??
    obj.type;

  const selected =
    obj.selected_candidate_ids ??
    obj.selected ??
    obj.winner_ids ??
    obj.winner ??
    obj.winners ??
    obj.elected ??
    obj.elected_candidates;

  const eliminated =
    obj.eliminated_candidate_ids ??
    obj.eliminated ??
    obj.excluded ??
    obj.removed ??
    obj.loser ??
    obj.losers;

  const titleParts = [`Шаг ${compact(stepNo)}`];
  if (action != null) titleParts.push(`· ${compact(action)}`);

  const details: Array<{ label: string; value: string }> = [];

  if (obj.title != null) {
    details.push({ label: "Заголовок", value: compact(obj.title) });
  }

  if (selected != null) {
    details.push({ label: "Выбраны", value: compact(selected) });
  }

  if (eliminated != null) {
    details.push({ label: "Исключены", value: compact(eliminated) });
  }

  if (obj.remaining_candidate_ids != null) {
    details.push({
      label: "Оставшиеся кандидаты",
      value: compact(obj.remaining_candidate_ids),
    });
  }

  if (obj.scores != null) {
    details.push({
      label: "Оценки",
      value: normalizeScoreArray(obj.scores),
    });
  }

  if (obj.note != null) {
    details.push({ label: "Примечание", value: compact(obj.note) });
  }

  for (const [key, value] of Object.entries(obj)) {
    if (
      [
        "round",
        "step",
        "iteration",
        "index",
        "order",
        "action",
        "event",
        "phase",
        "status",
        "kind",
        "type",
        "title",
        "selected_candidate_ids",
        "selected",
        "winner_ids",
        "winner",
        "winners",
        "elected",
        "elected_candidates",
        "eliminated_candidate_ids",
        "eliminated",
        "excluded",
        "removed",
        "loser",
        "losers",
        "remaining_candidate_ids",
        "scores",
        "note",
      ].includes(key)
    ) {
      continue;
    }

    if (value == null) continue;
    details.push({ label: key, value: compact(value) });
  }

  return {
    key: `step-${fallbackIndex}`,
    title: titleParts.join(" "),
    details,
  };
}

function normalizeProtocol(protocol: unknown): ProtocolEntry[] {
  if (protocol == null) return [];

  if (Array.isArray(protocol)) {
    return protocol.map((item, index) => {
      if (isObject(item)) return normalizeStepObject(item, index);
      return {
        key: `step-${index}`,
        title: `Шаг ${index + 1}`,
        subtitle: compact(item),
      };
    });
  }

  if (isObject(protocol)) {
    const nestedArray = pickRecordArray(protocol, ["steps", "rounds", "events", "items", "protocol"]);
    if (nestedArray) {
      return normalizeProtocol(nestedArray);
    }

    return [normalizeStepObject(protocol, 0)];
  }

  return [
    {
      key: "step-0",
      title: "Протокол",
      subtitle: compact(protocol),
    },
  ];
}

export function ProtocolTimeline({ protocol }: { protocol: unknown }) {
  const entries = normalizeProtocol(protocol);

  if (entries.length === 0) {
    return <div style={styles.muted}>Подробный протокол для данного метода отсутствует</div>;
  }

  return (
    <div style={{ display: "grid", gap: 10 }}>
      {entries.map((entry) => (
        <div key={entry.key} style={{ ...styles.card, padding: 12 }}>
          <div style={{ fontWeight: 700 }}>{entry.title}</div>

          {entry.subtitle ? (
            <div style={{ marginTop: 6, ...styles.muted }}>{entry.subtitle}</div>
          ) : null}

          {entry.details && entry.details.length > 0 ? (
            <div style={{ marginTop: 10, display: "grid", gap: 6 }}>
              {entry.details.map((detail, index) => (
                <div key={`${entry.key}-${detail.label}-${index}`}>
                  <b>{detail.label}:</b> <span>{detail.value}</span>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ))}
    </div>
  );
}