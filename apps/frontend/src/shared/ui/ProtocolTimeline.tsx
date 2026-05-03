import React from "react";
import { styles } from "./styles";

type ProtocolEntry = {
  key: string;
  title: string;
  subtitle?: string;
  details?: Array<{ label: string; value: React.ReactNode }>;
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
  return safeStringify(normalizeKeyValueShape(value));
}

function isKeyValueEntry(value: unknown): value is { Key: unknown; Value: unknown } {
  return (
    isObject(value) &&
    Object.prototype.hasOwnProperty.call(value, "Key") &&
    Object.prototype.hasOwnProperty.call(value, "Value")
  );
}

function normalizeKeyValueShape(value: unknown): unknown {
  if (Array.isArray(value)) {
    if (value.length > 0 && value.every(isKeyValueEntry)) {
      const out: Record<string, unknown> = {};

      for (const item of value) {
        const key = typeof item.Key === "string" ? item.Key : compact(item.Key);
        if (!key || key === "—") continue;
        out[key] = normalizeKeyValueShape(item.Value);
      }

      return out;
    }

    return value.map((item) => normalizeKeyValueShape(item));
  }

  if (isObject(value)) {
    if (isKeyValueEntry(value)) {
      const key = typeof value.Key === "string" ? value.Key : compact(value.Key);
      return {
        [key]: normalizeKeyValueShape(value.Value),
      };
    }

    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      out[key] = normalizeKeyValueShape(item);
    }
    return out;
  }

  return value;
}

function titleCaseAction(value: unknown) {
  const raw = compact(value);
  const labels: Record<string, string> = {
    single_step: "Один шаг",
    declare_winner: "Победитель определен",
    eliminate: "Кандидат исключен",
    select: "Кандидат выбран",
    round: "Раунд",
    final: "Итог",
  };

  return labels[raw] || raw.replace(/_/g, " ");
}

function labelForKey(key: string) {
  const labels: Record<string, string> = {
    kind: "Тип протокола",
    step: "Номер шага",
    round: "Раунд",
    title: "Название",
    action: "Действие",
    note: "Примечание",
    remaining_candidate_ids: "Оставшиеся кандидаты",
    selected_candidate_ids: "Выбранные кандидаты",
    winner_ids: "Победители",
    winners: "Победители",
    eliminated_candidate_ids: "Исключенные кандидаты",
    scores: "Оценки",
    final: "Итог",
  };

  return labels[key] || key.replace(/_/g, " ");
}

function candidateLabel(item: Record<string, unknown>) {
  const candidateName =
    typeof item.candidate_name === "string" && item.candidate_name.trim()
      ? item.candidate_name.trim()
      : "";

  const candidateID =
    typeof item.candidate_id === "string" && item.candidate_id.trim()
      ? item.candidate_id.trim()
      : "";

  if (candidateName && candidateID) return `${candidateName} (${candidateID})`;
  if (candidateName) return candidateName;
  if (candidateID) return candidateID;

  return "Кандидат";
}

function scoreValue(item: Record<string, unknown>) {
  const scoreKind =
    typeof item.score_kind === "string" && item.score_kind.trim()
      ? item.score_kind.trim()
      : "";

  if (scoreKind === "vector" && Array.isArray(item.values)) {
    return `[${item.values.map((v) => compact(v)).join(", ")}]`;
  }

  if (Array.isArray(item.values)) {
    return `[${item.values.map((v) => compact(v)).join(", ")}]`;
  }

  return compact(item.value ?? item.score ?? item.points ?? item.count);
}

function renderInlineList(value: unknown): React.ReactNode {
  const normalized = normalizeKeyValueShape(value);

  if (Array.isArray(normalized)) {
    if (normalized.length === 0) return "—";

    return (
      <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
        {normalized.map((item, index) => (
          <span
            key={`${compact(item)}-${index}`}
            style={{
              border: "1px solid #e5e7eb",
              borderRadius: 999,
              padding: "2px 8px",
              background: "#f9fafb",
              fontSize: 13,
            }}
          >
            {compact(item)}
          </span>
        ))}
      </div>
    );
  }

  return compact(normalized);
}

function renderScores(value: unknown): React.ReactNode {
  const normalized = normalizeKeyValueShape(value);

  if (!Array.isArray(normalized)) {
    return compact(normalized);
  }

  const rows = normalized
    .map((item) => (isObject(item) ? item : null))
    .filter((item): item is Record<string, unknown> => item != null);

  if (rows.length === 0) {
    return compact(normalized);
  }

  return (
    <div style={{ display: "grid", gap: 6 }}>
      {rows.map((item, index) => (
        <div
          key={`${compact(item.candidate_id)}-${index}`}
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(120px, 1fr) auto",
            gap: 12,
            alignItems: "center",
            padding: "6px 8px",
            border: "1px solid #eef2f7",
            borderRadius: 10,
            background: "#f9fafb",
          }}
        >
          <span>{candidateLabel(item)}</span>
          <b>{scoreValue(item)}</b>
        </div>
      ))}
    </div>
  );
}

function renderValue(key: string, value: unknown): React.ReactNode {
  const normalized = normalizeKeyValueShape(value);

  if (
    key === "winner_ids" ||
    key === "winners" ||
    key === "selected_candidate_ids" ||
    key === "remaining_candidate_ids" ||
    key === "eliminated_candidate_ids"
  ) {
    return renderInlineList(normalized);
  }

  if (key === "scores") {
    return renderScores(normalized);
  }

  if (Array.isArray(normalized)) {
    return renderInlineList(normalized);
  }

  if (isObject(normalized)) {
    return (
      <pre
        style={{
          margin: 0,
          whiteSpace: "pre-wrap",
          wordBreak: "break-word",
          fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
          fontSize: 13,
        }}
      >
        {JSON.stringify(normalized, null, 2)}
      </pre>
    );
  }

  return compact(normalized);
}

function pickRecordArray(value: Record<string, unknown>, keys: string[]): unknown[] | null {
  for (const key of keys) {
    const candidate = value[key];
    if (Array.isArray(candidate)) return candidate;
  }
  return null;
}

function normalizeStepObject(raw: Record<string, unknown>, fallbackIndex: number): ProtocolEntry {
  const obj = normalizeKeyValueShape(raw);
  const normalizedObj = isObject(obj) ? obj : raw;

  const stepNo =
    normalizedObj.round ??
    normalizedObj.step ??
    normalizedObj.iteration ??
    normalizedObj.index ??
    normalizedObj.order ??
    fallbackIndex + 1;

  const action =
    normalizedObj.action ??
    normalizedObj.event ??
    normalizedObj.phase ??
    normalizedObj.status ??
    normalizedObj.kind ??
    normalizedObj.type;

  const title =
    typeof normalizedObj.title === "string" && normalizedObj.title.trim()
      ? normalizedObj.title.trim()
      : `Шаг ${compact(stepNo)}`;

  const titleParts = [title];
  if (action != null) titleParts.push(`· ${titleCaseAction(action)}`);

  const priorityKeys = [
    "winner_ids",
    "winners",
    "selected_candidate_ids",
    "eliminated_candidate_ids",
    "remaining_candidate_ids",
    "scores",
    "note",
  ];

  const skipped = new Set([
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
  ]);

  const details: Array<{ label: string; value: React.ReactNode }> = [];

  for (const key of priorityKeys) {
    if (normalizedObj[key] == null) continue;

    details.push({
      label: labelForKey(key),
      value: renderValue(key, normalizedObj[key]),
    });
    skipped.add(key);
  }

  for (const [key, value] of Object.entries(normalizedObj)) {
    if (skipped.has(key)) continue;
    if (value == null) continue;

    details.push({
      label: labelForKey(key),
      value: renderValue(key, value),
    });
  }

  return {
    key: `step-${fallbackIndex}`,
    title: titleParts.join(" "),
    details,
  };
}

function normalizeProtocol(protocol: unknown): ProtocolEntry[] {
  const normalized = normalizeKeyValueShape(protocol);

  if (normalized == null) return [];

  if (Array.isArray(normalized)) {
    return normalized.map((item, index) => {
      if (isObject(item)) return normalizeStepObject(item, index);

      return {
        key: `step-${index}`,
        title: `Шаг ${index + 1}`,
        subtitle: compact(item),
      };
    });
  }

  if (isObject(normalized)) {
    const entries: ProtocolEntry[] = [];

    if (normalized.kind != null) {
      entries.push({
        key: "kind",
        title: "Тип протокола",
        details: [{ label: "Значение", value: renderValue("kind", normalized.kind) }],
      });
    }

    const nestedArray = pickRecordArray(normalized, ["steps", "rounds", "events", "items", "protocol"]);
    if (nestedArray) {
      entries.push(...normalizeProtocol(nestedArray));
    }

    if (normalized.final != null) {
      const finalValue = normalizeKeyValueShape(normalized.final);
      if (isObject(finalValue)) {
        entries.push({
          key: "final",
          title: "Итог",
          details: Object.entries(finalValue).map(([key, value]) => ({
            label: labelForKey(key),
            value: renderValue(key, value),
          })),
        });
      } else {
        entries.push({
          key: "final",
          title: "Итог",
          subtitle: compact(finalValue),
        });
      }
    }

    if (entries.length > 0) {
      return entries;
    }

    return [normalizeStepObject(normalized, 0)];
  }

  return [
    {
      key: "step-0",
      title: "Протокол",
      subtitle: compact(normalized),
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
            <div style={{ marginTop: 10, display: "grid", gap: 8 }}>
              {entry.details.map((detail, index) => (
                <div
                  key={`${entry.key}-${detail.label}-${index}`}
                  style={{
                    display: "grid",
                    gridTemplateColumns: "160px minmax(0, 1fr)",
                    gap: 10,
                    alignItems: "start",
                  }}
                >
                  <b>{detail.label}</b>
                  <div style={{ minWidth: 0, wordBreak: "break-word" }}>{detail.value}</div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ))}
    </div>
  );
}