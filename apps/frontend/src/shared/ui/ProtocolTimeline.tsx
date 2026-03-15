import React from "react";
import { JsonBlock } from "./JsonBlock";
import { styles } from "./styles";

type ProtocolEntry = {
  key: string;
  title: string;
  subtitle?: string;
  details?: Array<{ label: string; value: string }>;
  raw?: unknown;
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
    obj.selected ??
    obj.winner ??
    obj.winners ??
    obj.elected ??
    obj.elected_candidates;

  const eliminated =
    obj.eliminated ??
    obj.excluded ??
    obj.removed ??
    obj.loser ??
    obj.losers;

  const titleParts = [`Шаг ${compact(stepNo)}`];
  if (action != null) titleParts.push(`· ${compact(action)}`);

  const details: Array<{ label: string; value: string }> = [];

  if (selected != null) {
    details.push({ label: "Selected", value: compact(selected) });
  }

  if (eliminated != null) {
    details.push({ label: "Eliminated", value: compact(eliminated) });
  }

  for (const [key, value] of Object.entries(obj)) {
    if (
      ["round", "step", "iteration", "index", "order", "action", "event", "phase", "status", "kind", "type", "selected", "winner", "winners", "elected", "elected_candidates", "eliminated", "excluded", "removed", "loser", "losers"].includes(
        key
      )
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
    raw: obj,
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
        raw: item,
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
      raw: protocol,
    },
  ];
}

export function ProtocolTimeline({ protocol }: { protocol: unknown }) {
  const entries = normalizeProtocol(protocol);

  if (entries.length === 0) {
    return <div style={styles.muted}>Протокол шагов отсутствует</div>;
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

          {entry.raw != null && !entry.details?.length && !entry.subtitle ? (
            <div style={{ marginTop: 10 }}>
              <JsonBlock value={entry.raw} />
            </div>
          ) : null}
        </div>
      ))}
    </div>
  );
}