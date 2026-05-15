import { jsPDF } from "jspdf";
import * as XLSX from "xlsx";

const UTF8_BOM = "\uFEFF";

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

function shouldAddUtf8Bom(mime: string) {
  const lower = mime.toLowerCase();
  return (
    lower.includes("text/plain") ||
    lower.includes("text/csv") ||
    lower.includes("application/json")
  );
}

export function downloadTextFile(
  filename: string,
  text: string,
  mime = "text/plain;charset=utf-8"
) {
  const parts = shouldAddUtf8Bom(mime) ? [UTF8_BOM, text] : [text];
  downloadBlob(filename, new Blob(parts, { type: mime }));
}

export function downloadJsonFile(filename: string, value: unknown) {
  downloadTextFile(
    filename,
    `${JSON.stringify(value, null, 2)}\n`,
    "application/json;charset=utf-8"
  );
}

function escapeCsvCell(value: unknown, delimiter: string) {
  const text = value == null ? "" : String(value);
  if (text.includes('"') || text.includes("\n") || text.includes("\r") || text.includes(delimiter)) {
    return `"${text.replace(/"/g, '""')}"`;
  }
  return text;
}

export function toCsv(rows: Array<Record<string, unknown>>, delimiter = ";") {
  if (rows.length === 0) {
    return "";
  }

  const headerSet = new Set<string>();
  for (const row of rows) {
    Object.keys(row).forEach((key) => headerSet.add(key));
  }

  const headers = Array.from(headerSet);
  const lines: string[] = [];

  lines.push(headers.map((header) => escapeCsvCell(header, delimiter)).join(delimiter));

  for (const row of rows) {
    lines.push(
      headers.map((header) => escapeCsvCell(row[header], delimiter)).join(delimiter)
    );
  }

  return `${lines.join("\r\n")}\r\n`;
}

export function downloadCsvFile(filename: string, rows: Array<Record<string, unknown>>) {
  const csv = toCsv(rows, ";");
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

  const range = XLSX.utils.decode_range(worksheet["!ref"] || "A1:A1");
  const widths = [];

  for (let col = range.s.c; col <= range.e.c; col++) {
    let max = 10;
    for (let row = range.s.r; row <= range.e.r; row++) {
      const cell = worksheet[XLSX.utils.encode_cell({ r: row, c: col })];
      if (!cell || cell.v == null) continue;
      max = Math.max(max, String(cell.v).length);
    }
    widths.push({ wch: Math.min(Math.max(max + 2, 10), 60) });
  }

  worksheet["!cols"] = widths;

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

function createPdfCanvas(width: number, height: number, scale: number) {
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(width * scale);
  canvas.height = Math.round(height * scale);

  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("Не удалось создать canvas для PDF");
  }

  ctx.scale(scale, scale);
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, width, height);
  ctx.fillStyle = "#111827";
  ctx.textBaseline = "top";

  return { canvas, ctx };
}

function wrapText(
  ctx: CanvasRenderingContext2D,
  text: string,
  maxWidth: number
): string[] {
  if (!text) return [""];

  const words = text.split(/\s+/);
  const lines: string[] = [];
  let line = "";

  for (const word of words) {
    const testLine = line ? `${line} ${word}` : word;

    if (ctx.measureText(testLine).width <= maxWidth) {
      line = testLine;
      continue;
    }

    if (line) {
      lines.push(line);
      line = "";
    }

    if (ctx.measureText(word).width <= maxWidth) {
      line = word;
      continue;
    }

    let chunk = "";
    for (const char of word) {
      const testChunk = chunk + char;
      if (ctx.measureText(testChunk).width <= maxWidth) {
        chunk = testChunk;
      } else {
        if (chunk) lines.push(chunk);
        chunk = char;
      }
    }
    line = chunk;
  }

  if (line) lines.push(line);

  return lines.length > 0 ? lines : [""];
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
  const scale = 2;

  const pages: HTMLCanvasElement[] = [];
  let { canvas, ctx } = createPdfCanvas(pageWidth, pageHeight, scale);
  let y = margin;

  ctx.font = "700 16px Arial, sans-serif";
  for (const line of wrapText(ctx, title, maxWidth)) {
    if (y > pageHeight - margin) {
      pages.push(canvas);
      ({ canvas, ctx } = createPdfCanvas(pageWidth, pageHeight, scale));
      y = margin;
      ctx.font = "700 16px Arial, sans-serif";
    }
    ctx.fillText(line, margin, y);
    y += 20;
  }

  y += 8;
  ctx.font = "400 10px Arial, sans-serif";

  const paragraphs = text.split("\n");

  for (const paragraph of paragraphs) {
    const wrapped = wrapText(ctx, paragraph || " ", maxWidth);

    for (const line of wrapped) {
      if (y > pageHeight - margin) {
        pages.push(canvas);
        ({ canvas, ctx } = createPdfCanvas(pageWidth, pageHeight, scale));
        ctx.font = "400 10px Arial, sans-serif";
        y = margin;
      }

      ctx.fillText(line, margin, y);
      y += lineHeight;
    }
  }

  pages.push(canvas);

  pages.forEach((page, index) => {
    if (index > 0) {
      doc.addPage();
    }

    doc.addImage(
      page.toDataURL("image/png"),
      "PNG",
      0,
      0,
      pageWidth,
      pageHeight
    );
  });

  doc.save(filename);
}