import React from "react";
import { styles } from "./styles";

export type ActionMenuItem = {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  hidden?: boolean;
  variant?: "default" | "primary" | "danger";
};

type ActionMenuProps = {
  label?: string;
  items: ActionMenuItem[];
};

function buttonStyle(variant: ActionMenuItem["variant"]): React.CSSProperties {
  if (variant === "primary") return styles.btnPrimary;
  if (variant === "danger") return styles.btnDanger;
  return styles.btn;
}

export function ActionMenu({ label = "Дополнительно", items }: ActionMenuProps) {
  const visibleItems = items.filter((item) => !item.hidden);

  if (visibleItems.length === 0) return null;

  return (
    <details style={{ position: "relative" }}>
      <summary
        style={{
          ...styles.btn,
          listStyle: "none",
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          userSelect: "none",
        }}
      >
        {label}
      </summary>

      <div
        style={{
          position: "absolute",
          right: 0,
          top: "calc(100% + 6px)",
          minWidth: 220,
          display: "grid",
          gap: 6,
          padding: 8,
          border: "1px solid #e5e7eb",
          borderRadius: 14,
          background: "#ffffff",
          boxShadow: "0 16px 36px rgba(15, 23, 42, 0.14)",
          zIndex: 20,
        }}
      >
        {visibleItems.map((item) => (
          <button
            key={item.label}
            type="button"
            style={{
              ...buttonStyle(item.variant),
              width: "100%",
              textAlign: "left",
              justifyContent: "flex-start",
              boxShadow: "none",
            }}
            onClick={item.onClick}
            disabled={item.disabled}
          >
            {item.label}
          </button>
        ))}
      </div>
    </details>
  );
}