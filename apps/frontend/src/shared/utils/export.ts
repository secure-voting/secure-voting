import { jsPDF } from "jspdf";
import * as XLSX from "xlsx";

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

function normalizeRows(rows: Array<Record<string, unknown>>) {
  return rows.map((row) => {
    const normalized: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(row)) {
      if (value == null) {
        normalized[key] = "";
      } else if (
        typeof value === "string" ||
        typeof value === "number" ||
        typeof value === "boolean"
      ) {
        normalized[key] = value;
      } else {
        try {
          normalized[key] = JSON.stringify(value);
        } catch {
          normalized[key] = String(value);
        }
      }
    }
    return normalized;
  });
}

export function downloadXlsxFile(
  filename: string,
  rows: Array<Record<string, unknown>>,
  sheetName = "Sheet1"
) {
  const workbook = XLSX.utils.book_new();

  const worksheet =
    rows.length > 0
      ? XLSX.utils.json_to_sheet(normalizeRows(rows))
      : XLSX.utils.aoa_to_sheet([[]]);

  XLSX.utils.book_append_sheet(workbook, worksheet, sheetName);

  const data = XLSX.write(workbook, {
    bookType: "xlsx",
    type: "array",
  });

  downloadBlob(
    filename,
    new Blob([data], {
      type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    })
  );
}

export function downloadPdfTextFile(
  filename: string,
  title: string,
  text: string
) {
  const doc = new jsPDF({
    unit: "pt",
    format: "a4",
  });

  const pageWidth = doc.internal.pageSize.getWidth();
  const pageHeight = doc.internal.pageSize.getHeight();
  const margin = 40;
  const lineHeight = 14;
  const maxWidth = pageWidth - margin * 2;

  let y = margin;

  doc.setFont("helvetica", "bold");
  doc.setFontSize(16);
  doc.text(title, margin, y);
  y += 24;

  doc.setFont("helvetica", "normal");
  doc.setFontSize(10);

  const paragraphs = text.split("\n");
  for (const paragraph of paragraphs) {
    const lines = doc.splitTextToSize(paragraph || " ", maxWidth) as string[];

    for (const line of lines) {
      if (y > pageHeight - margin) {
        doc.addPage();
        y = margin;
      }
      doc.text(line, margin, y);
      y += lineHeight;
    }
  }

  doc.save(filename);
}