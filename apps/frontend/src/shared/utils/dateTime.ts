export function formatDateTime(value: unknown) {
  if (typeof value !== "string" || !value.trim()) return "—";

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  return date.toLocaleString("ru-RU", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatDate(value: unknown) {
  if (typeof value !== "string" || !value.trim()) return "—";

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  return date.toLocaleDateString("ru-RU", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
}