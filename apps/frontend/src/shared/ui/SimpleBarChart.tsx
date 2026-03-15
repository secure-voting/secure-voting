import React from "react";
import { styles } from "./styles";

export type SimpleBarChartItem = {
  label: string;
  value: number;
};

export function SimpleBarChart({
  title,
  items,
  emptyText = "Недостаточно данных для построения графика",
  valueFormatter,
}: {
  title?: string;
  items: SimpleBarChartItem[];
  emptyText?: string;
  valueFormatter?: (value: number) => string;
}) {
  const normalized = items.filter(
    (item) => Number.isFinite(item.value) && item.value >= 0
  );
  const max = normalized.reduce((acc, item) => Math.max(acc, item.value), 0);

  return (
    <div style={styles.card}>
      {title ? <h4 style={{ marginTop: 0, marginBottom: 12 }}>{title}</h4> : null}

      {normalized.length === 0 || max <= 0 ? (
        <div style={styles.muted}>{emptyText}</div>
      ) : (
        <div style={{ display: "grid", gap: 10 }}>
          {normalized.map((item) => {
            const width = Math.max(2, (item.value / max) * 100);

            return (
              <div key={item.label} style={{ display: "grid", gap: 6 }}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 10,
                    alignItems: "baseline",
                  }}
                >
                  <span style={{ fontWeight: 600, wordBreak: "break-word" }}>{item.label}</span>
                  <span style={styles.muted}>
                    {valueFormatter ? valueFormatter(item.value) : item.value}
                  </span>
                </div>

                <div
                  style={{
                    width: "100%",
                    height: 12,
                    borderRadius: 999,
                    background: "#f3f4f6",
                    overflow: "hidden",
                  }}
                >
                  <div
                    style={{
                      width: `${width}%`,
                      height: "100%",
                      borderRadius: 999,
                      background: "#111827",
                    }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}