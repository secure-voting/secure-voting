import type React from "react";

export const styles: Record<string, React.CSSProperties> = {
  page: {
    fontFamily: "system-ui, -apple-system, Segoe UI, Roboto, sans-serif",
    padding: 20,
    maxWidth: 1180,
    margin: "0 auto",
    background: "#f8fafc",
    minHeight: "100vh",
    color: "#111827",
  },

  topbar: {
    display: "flex",
    alignItems: "center",
    gap: 12,
    justifyContent: "space-between",
    padding: "16px 18px",
    border: "1px solid #e5e7eb",
    borderRadius: 18,
    marginBottom: 16,
    background: "rgba(255, 255, 255, 0.96)",
    boxShadow: "0 10px 30px rgba(15, 23, 42, 0.06)",
  },

  title: {
    margin: 0,
    fontSize: 22,
    fontWeight: 850,
    letterSpacing: "-0.02em",
    color: "#0f172a",
  },

  btn: {
    padding: "9px 14px",
    borderRadius: 12,
    border: "1px solid #d1d5db",
    background: "#ffffff",
    color: "#111827",
    cursor: "pointer",
    fontWeight: 600,
    lineHeight: 1.2,
    boxShadow: "0 1px 2px rgba(15, 23, 42, 0.04)",
  },

  btnPrimary: {
    padding: "9px 14px",
    borderRadius: 12,
    border: "1px solid #111827",
    background: "#111827",
    color: "white",
    cursor: "pointer",
    fontWeight: 700,
    lineHeight: 1.2,
    boxShadow: "0 8px 18px rgba(17, 24, 39, 0.16)",
  },

  btnDanger: {
    padding: "9px 14px",
    borderRadius: 12,
    border: "1px solid #991b1b",
    background: "#991b1b",
    color: "white",
    cursor: "pointer",
    fontWeight: 700,
    lineHeight: 1.2,
    boxShadow: "0 8px 18px rgba(153, 27, 27, 0.14)",
  },

  input: {
    padding: "9px 11px",
    borderRadius: 11,
    border: "1px solid #d1d5db",
    width: "100%",
    boxSizing: "border-box",
    background: "white",
    color: "#111827",
    outline: "none",
  },

  card: {
    border: "1px solid #e5e7eb",
    borderRadius: 16,
    padding: 16,
    background: "white",
    boxShadow: "0 8px 24px rgba(15, 23, 42, 0.04)",
  },

  grid2: {
    display: "grid",
    gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
    gap: 12,
  },

  muted: {
    color: "#6b7280",
  },

  hr: {
    border: "none",
    borderTop: "1px solid #e5e7eb",
    margin: "12px 0",
  },

  pre: {
    whiteSpace: "pre-wrap",
    wordBreak: "break-word",
    background: "#0b1020",
    color: "#e5e7eb",
    padding: 12,
    borderRadius: 12,
    fontSize: 12,
    overflowX: "auto",
  },
};