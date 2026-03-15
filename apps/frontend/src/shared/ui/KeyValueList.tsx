import React from "react";
import { styles } from "./styles";

export function KeyValueList({
  items,
}: {
  items: Array<{
    label: string;
    value: React.ReactNode;
  }>;
}) {
  return (
    <div style={{ display: "grid", gap: 8 }}>
      {items.map((item) => (
        <div
          key={item.label}
          style={{
            display: "grid",
            gridTemplateColumns: "180px 1fr",
            gap: 12,
            alignItems: "start",
            padding: "8px 0",
            borderBottom: "1px solid #f3f4f6",
          }}
        >
          <div style={{ ...styles.muted, fontSize: 13 }}>{item.label}</div>
          <div>{item.value}</div>
        </div>
      ))}
    </div>
  );
}