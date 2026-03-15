export function downloadBlob(filename: string, blob: Blob) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function downloadTextFile(
  filename: string,
  text: string,
  mime = "text/plain;charset=utf-8"
) {
  downloadBlob(filename, new Blob([text], { type: mime }));
}

export function downloadJsonFile(filename: string, value: unknown) {
  downloadTextFile(filename, `${JSON.stringify(value, null, 2)}\n`, "application/json;charset=utf-8");
}

function escapeCsvCell(value: unknown) {
  const text = value == null ? "" : String(value);
  if (/[",\n;]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`;
  }
  return text;
}

export function toCsv(rows: Array<Record<string, unknown>>) {
  if (rows.length === 0) {
    return "";
  }

  const headerSet = new Set<string>();
  for (const row of rows) {
    Object.keys(row).forEach((key) => headerSet.add(key));
  }

  const headers = Array.from(headerSet);
  const lines: string[] = [];

  lines.push(headers.map((header) => escapeCsvCell(header)).join(","));

  for (const row of rows) {
    lines.push(
      headers.map((header) => escapeCsvCell(row[header])).join(",")
    );
  }

  return `${lines.join("\n")}\n`;
}

export function downloadCsvFile(filename: string, rows: Array<Record<string, unknown>>) {
  const csv = toCsv(rows);
  downloadTextFile(filename, csv, "text/csv;charset=utf-8");
}