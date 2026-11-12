import type { FileList, ServiceInfo } from "./types";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
  ) {
    super(message);
  }
}

async function parseError(res: Response): Promise<ApiError> {
  try {
    const body = await res.json();
    return new ApiError(body.error?.message ?? res.statusText, res.status, body.error?.code ?? "unknown");
  } catch {
    return new ApiError(`Server membalas ${res.status}.`, res.status, "unknown");
  }
}

async function request<T>(input: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(input, init);
  } catch {
    throw new ApiError("Server aplikasi tidak dapat dihubungi. Periksa layanan aplikasi.", 0, "network");
  }
  if (!res.ok) throw await parseError(res);
  return (res.status === 204 ? undefined : await res.json()) as T;
}

export const fileUrl = (key: string, download = false) =>
  `/api/files/${encodeURIComponent(key)}${download ? "?download" : ""}`;

export const getInfo = () => request<ServiceInfo>("/api/info");

/** True when the app can reach its bucket. Never throws. */
export async function getHealth(): Promise<boolean> {
  try {
    const res = await fetch("/api/health");
    return res.ok;
  } catch {
    return false;
  }
}

export const listFiles = () => request<FileList>("/api/files");

export const deleteFile = (key: string) =>
  request<void>(`/api/files/${encodeURIComponent(key)}`, { method: "DELETE" });

/** Uploads files with progress reporting (fetch has no upload progress). */
export function uploadFiles(files: File[], onProgress: (fraction: number) => void): Promise<string[]> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    files.forEach((f) => form.append("files", f, f.name));

    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/files");
    xhr.responseType = "json";
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded / e.total);
    };
    xhr.onload = () => {
      if (xhr.status === 201) {
        resolve(xhr.response.uploaded as string[]);
      } else {
        const err = xhr.response?.error;
        reject(new ApiError(err?.message ?? `Server membalas ${xhr.status}.`, xhr.status, err?.code ?? "unknown"));
      }
    };
    xhr.onerror = () => reject(new ApiError("Koneksi terputus saat mengunggah. Silakan coba lagi.", 0, "network"));
    xhr.send(form);
  });
}
