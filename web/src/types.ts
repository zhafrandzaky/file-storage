export type Role = "admin" | "member";

export interface User {
  id: number;
  name: string;
  email: string;
  role: Role;
  active: boolean;
  createdAt: string;
}

export type FileKind = "document" | "image" | "spreadsheet" | "archive" | "other";

export interface StoredFile {
  id: string;
  name: string;
  size: number;
  mimeType: string;
  kind: FileKind;
  ownerId: number;
  ownerName: string;
  createdAt: string;
}

export interface FileSummary {
  count: number;
  totalSize: number;
  byKind: Partial<Record<FileKind, number>>;
}

export interface FileListResponse {
  files: StoredFile[];
  summary: FileSummary;
}

export interface ServiceInfo {
  bucket: string;
  endpoint?: string;
  maxUploadBytes: number;
  maxFileBytes: number;
  allowedExtensions: string[];
  shareEnabled: boolean;
  shareMaxHours: number;
}

export interface ShareLink {
  url: string;
  expiresAt: string;
}

export interface AuditEntry {
  id: number;
  actorId?: number;
  actorEmail: string;
  action: string;
  target: string;
  detail: string;
  ip: string;
  createdAt: string;
}
