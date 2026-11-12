import type { FileKind } from "./types";

const number1 = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 1, maximumFractionDigits: 1 });

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${number1.format(bytes / 1024)} KB`;
  if (bytes < 1024 ** 3) return `${number1.format(bytes / 1024 ** 2)} MB`;
  return `${number1.format(bytes / 1024 ** 3)} GB`;
}

const dateTime = new Intl.DateTimeFormat("id-ID", {
  day: "2-digit",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});
const dateOnly = new Intl.DateTimeFormat("id-ID", { day: "2-digit", month: "short", year: "numeric" });

export const formatDateTime = (iso: string) => dateTime.format(new Date(iso));
export const formatDate = (iso: string) => dateOnly.format(new Date(iso));

export function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : "";
}

export const kindLabels: Record<FileKind, string> = {
  document: "Dokumen",
  image: "Gambar",
  spreadsheet: "Spreadsheet",
  archive: "Arsip",
  other: "Lainnya",
};

export const actionLabels: Record<string, string> = {
  "auth.login": "Masuk",
  "auth.login_failed": "Gagal masuk",
  "auth.logout": "Keluar",
  "auth.password_change": "Mengganti kata sandi",
  "file.upload": "Mengunggah berkas",
  "file.delete": "Menghapus berkas",
  "file.share": "Membuat tautan berbagi",
  "user.create": "Menambah pengguna",
  "user.update": "Mengubah pengguna",
  "user.password_reset": "Mengatur ulang kata sandi",
};

export const roleLabel = (role: string) => (role === "admin" ? "Administrator" : "Anggota");

export function formatDuration(hours: number): string {
  if (hours % 24 === 0) return `${hours / 24} hari`;
  return `${hours} jam`;
}
