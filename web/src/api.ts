import type { AuditEntry, FileKind, FileListResponse, Role, ServiceInfo, ShareLink, StoredFile, User } from "./types";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
  ) {
    super(message);
  }
}

/** Called whenever the server reports the session is gone. */
let onUnauthorized: () => void = () => {};
export const setUnauthorizedHandler = (fn: () => void) => {
  onUnauthorized = fn;
};

// Every state-changing request carries this header; the server rejects
// requests without it as a CSRF defence.
const CSRF = { "X-Requested-With": "file-storage" };

async function toError(res: Response): Promise<ApiError> {
  try {
    const body = await res.json();
    return new ApiError(body.error?.message ?? res.statusText, res.status, body.error?.code ?? "unknown");
  } catch {
    return new ApiError(`Server membalas ${res.status}.`, res.status, "unknown");
  }
}

async function request<T>(method: string, path: string, body?: unknown, handle401 = true): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: body === undefined ? CSRF : { ...CSRF, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError("Server aplikasi tidak dapat dihubungi. Periksa layanan aplikasi.", 0, "network");
  }
  if (!res.ok) {
    const err = await toError(res);
    if (res.status === 401 && handle401) onUnauthorized();
    throw err;
  }
  return (res.status === 204 ? undefined : await res.json()) as T;
}

export const auth = {
  me: () => request<User>("GET", "/api/auth/me", undefined, false),
  login: (email: string, password: string) => request<User>("POST", "/api/auth/login", { email, password }, false),
  logout: () => request<void>("POST", "/api/auth/logout"),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>("POST", "/api/auth/password", { currentPassword, newPassword }),
};

export const getInfo = () => request<ServiceInfo>("GET", "/api/info");

/** True when the app can reach its bucket. Never throws. */
export async function getHealth(): Promise<boolean> {
  try {
    return (await fetch("/api/health")).ok;
  } catch {
    return false;
  }
}

export const fileUrl = (id: string, download = false) => `/api/files/${encodeURIComponent(id)}${download ? "?download" : ""}`;

export const filesApi = {
  list: (kind: FileKind | "", q: string) => {
    const params = new URLSearchParams();
    if (kind) params.set("kind", kind);
    if (q.trim()) params.set("q", q.trim());
    const qs = params.toString();
    return request<FileListResponse>("GET", `/api/files${qs ? `?${qs}` : ""}`);
  },
  remove: (id: string) => request<void>("DELETE", `/api/files/${encodeURIComponent(id)}`),
  share: (id: string, hours: number) => request<ShareLink>("POST", `/api/files/${encodeURIComponent(id)}/share`, { hours }),

  /** Uploads with progress reporting (fetch has no upload progress). */
  upload(files: File[], onProgress: (fraction: number) => void): Promise<StoredFile[]> {
    return new Promise((resolve, reject) => {
      const form = new FormData();
      files.forEach((f) => form.append("files", f, f.name));
      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/api/files");
      xhr.setRequestHeader("X-Requested-With", CSRF["X-Requested-With"]);
      xhr.responseType = "json";
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(e.loaded / e.total);
      };
      xhr.onload = () => {
        if (xhr.status === 201) return resolve(xhr.response.uploaded as StoredFile[]);
        if (xhr.status === 401) onUnauthorized();
        const err = xhr.response?.error;
        reject(new ApiError(err?.message ?? `Server membalas ${xhr.status}.`, xhr.status, err?.code ?? "unknown"));
      };
      xhr.onerror = () => reject(new ApiError("Koneksi terputus saat mengunggah. Silakan coba lagi.", 0, "network"));
      xhr.send(form);
    });
  },
};

export const usersApi = {
  list: () => request<{ users: User[] }>("GET", "/api/users"),
  create: (u: { name: string; email: string; password: string; role: Role }) => request<User>("POST", "/api/users", u),
  update: (id: number, patch: { role?: Role; active?: boolean }) => request<User>("PATCH", `/api/users/${id}`, patch),
  resetPassword: (id: number, password: string) => request<void>("POST", `/api/users/${id}/password`, { password }),
};

export const auditApi = {
  list: (limit = 200) => request<{ entries: AuditEntry[] }>("GET", `/api/audit?limit=${limit}`),
};

export const errorMessage = (err: unknown) =>
  err instanceof ApiError ? err.message : "Terjadi kesalahan yang tidak terduga.";
