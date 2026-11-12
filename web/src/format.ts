import type { FileKind } from "./types";

const number1 = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 1, maximumFractionDigits: 1 });

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${number1.format(bytes / 1024)} KB`;
  if (bytes < 1024 ** 3) return `${number1.format(bytes / 1024 ** 2)} MB`;
  return `${number1.format(bytes / 1024 ** 3)} GB`;
}

const dateFmt = new Intl.DateTimeFormat("id-ID", {
  day: "2-digit",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

export function formatDate(iso: string): string {
  return dateFmt.format(new Date(iso));
}

export function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : "";
}

const kinds: Record<string, FileKind> = {
  pdf: "pdf",
  png: "image", jpg: "image", jpeg: "image", gif: "image", webp: "image", svg: "image", bmp: "image",
  xls: "sheet", xlsx: "sheet", csv: "sheet", ods: "sheet",
  doc: "doc", docx: "doc", odt: "doc", txt: "doc", md: "doc", rtf: "doc",
  zip: "archive", rar: "archive", "7z": "archive", tar: "archive", gz: "archive",
};

export const kindOf = (ext: string): FileKind => kinds[ext] ?? "file";
