export interface StoredFile {
  key: string;
  size: number;
  lastModified: string;
}

export interface FileList {
  files: StoredFile[];
  totalSize: number;
}

export interface ServiceInfo {
  bucket: string;
  endpoint: string;
  maxUploadBytes: number;
}

export type FileKind = "pdf" | "image" | "sheet" | "doc" | "archive" | "file";
