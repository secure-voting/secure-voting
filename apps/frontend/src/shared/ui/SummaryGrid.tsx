import React from "react";
import { styles } from "./styles";

export function SummaryGrid({
  items,
}: {
  items: Array<{
    label: string;
    value: React.ReactNode;
  }>;
}) {
  if (items.length === 0) {
    return <div style={styles.muted}>Нет данных для отображения</div>;
  }

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
        gap: 10,
      }}
    >
      {items.map((item) => (
        <div key={item.label} style={{ ...styles.card, padding: 12 }}>
          <div style={{ ...styles.muted, fontSize: 12, marginBottom: 6 }}>{item.label}</div>
          <div style={{ fontWeight: 700, wordBreak: "break-word" }}>{item.value}</div>
        </div>
      ))}
    </div>
  );
}