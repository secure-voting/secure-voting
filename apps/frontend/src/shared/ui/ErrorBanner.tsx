import React from "react";
import { styles } from "./styles";

export function ErrorBanner({ error }: { error: string | null }) {
  if (!error) return null;
  return (
    <div
      style={{
        ...styles.card,
        borderColor: "#fecaca",
        background: "#fff1f2",
        color: "#7f1d1d",
        marginBottom: 12,
      }}
    >
      <b>Ошибка:</b> {error}
    </div>
  );
}