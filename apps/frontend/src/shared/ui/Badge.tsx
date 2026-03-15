import React from "react";

export function Badge({ text }: { text: string }) {
  return (
    <span
      style={{
        display: "inline-block",
        padding: "2px 8px",
        borderRadius: 999,
        border: "1px solid #e5e7eb",
        fontSize: 12,
      }}
    >
      {text}
    </span>
  );
}