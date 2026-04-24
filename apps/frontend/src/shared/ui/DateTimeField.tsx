import React, { useMemo } from "react";
import { styles } from "./styles";

type Props = {
  label: React.ReactNode;
  value: string;
  onChange: (next: string) => void;
  disabled?: boolean;
  minuteStep?: number;
};

function pad2(v: number) {
  return String(v).padStart(2, "0");
}

function clampMinuteToStep(minute: number, step: number) {
  const normalizedStep = Math.max(1, Math.min(60, step));
  const rounded = Math.round(minute / normalizedStep) * normalizedStep;
  if (rounded >= 60) return 60 - normalizedStep;
  if (rounded < 0) return 0;
  return rounded;
}

function normalizeLocalDateTime(value: string, step: number) {
  if (!value) return "";

  const match = value.match(/^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2})/);
  if (!match) return value;

  const [, datePart, hh, mm] = match;
  const hour = Number(hh);
  const minute = Number(mm);

  if (!Number.isFinite(hour) || !Number.isFinite(minute)) return value;

  const nextMinute = clampMinuteToStep(minute, step);
  return `${datePart}T${pad2(hour)}:${pad2(nextMinute)}`;
}

function splitValue(value: string, step: number) {
  const normalized = normalizeLocalDateTime(value, step);
  if (!normalized) {
    return { date: "", time: "" };
  }

  const [date, time] = normalized.split("T");
  return {
    date: date || "",
    time: (time || "").slice(0, 5),
  };
}

function buildTimeOptions(step: number) {
  const normalizedStep = Math.max(1, Math.min(60, step));
  const items: string[] = [];

  for (let hour = 0; hour < 24; hour += 1) {
    for (let minute = 0; minute < 60; minute += normalizedStep) {
      items.push(`${pad2(hour)}:${pad2(minute)}`);
    }
  }

  return items;
}

export function DateTimeField({
  label,
  value,
  onChange,
  disabled,
  minuteStep = 5,
}: Props) {
  const normalizedStep = Math.max(1, Math.min(60, minuteStep));
  const parts = useMemo(() => splitValue(value, normalizedStep), [value, normalizedStep]);
  const timeOptions = useMemo(() => buildTimeOptions(normalizedStep), [normalizedStep]);

  const setDate = (nextDate: string) => {
    if (!nextDate) {
      onChange("");
      return;
    }
    const nextTime = parts.time || "00:00";
    onChange(`${nextDate}T${nextTime}`);
  };

  const setTime = (nextTime: string) => {
    if (!parts.date) {
      const today = new Date();
      const localDate = `${today.getFullYear()}-${pad2(today.getMonth() + 1)}-${pad2(today.getDate())}`;
      onChange(`${localDate}T${nextTime}`);
      return;
    }
    onChange(`${parts.date}T${nextTime}`);
  };

  return (
    <div style={{ display: "grid", gap: 6 }}>
      <label>{label}</label>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "minmax(180px, 1fr) minmax(120px, 160px)",
          gap: 8,
        }}
      >
        <input
          style={styles.input}
          type="date"
          value={parts.date}
          onChange={(e) => setDate(e.target.value)}
          disabled={disabled}
        />

        <select
          style={styles.input}
          value={parts.time}
          onChange={(e) => setTime(e.target.value)}
          disabled={disabled}
        >
          {!parts.time ? <option value="">Выберите время</option> : null}
          {timeOptions.map((time) => (
            <option key={time} value={time}>
              {time}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}