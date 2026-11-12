import { errorMessage, filesApi } from "../api";
import { h } from "../dom";
import { extensionOf, formatSize } from "../format";
import { icon } from "../icons";
import type { ServiceInfo, StoredFile } from "../types";

interface UploaderOptions {
  info: ServiceInfo;
  onUploaded(files: StoredFile[]): void;
  onError(message: string): void;
}

const MB = 1024 ** 2;

export function createUploader({ info, onUploaded, onError }: UploaderOptions): HTMLElement {
  let selected: File[] = [];
  let busy = false;

  const allowed = new Set(info.allowedExtensions);
  const input = h("input", {
    class: "sr-only",
    type: "file",
    id: "files",
    multiple: true,
    accept: info.allowedExtensions.map((e) => `.${e}`).join(","),
  });
  const title = h("strong", {});
  const hint = h("span", {});
  const bar = h("div", { class: "progress-bar" });
  const progress = h(
    "div",
    { class: "progress", role: "progressbar", "aria-label": "Progres unggahan", "aria-valuemin": "0", "aria-valuemax": "100", hidden: true },
    bar,
  );
  const label = h("span", {}, "Unggah");
  const submit = h("button", { class: "btn btn-primary", type: "submit", disabled: true }, icon("upload"), label);

  const dropzone = h(
    "div",
    { class: "dropzone" },
    h("div", { class: "dropzone-icon" }, icon("uploadTray")),
    h("div", { class: "dropzone-text" }, title, hint, progress),
    h("div", { class: "dropzone-actions" }, h("label", { class: "btn btn-plain", for: "files" }, "Pilih berkas"), input, submit),
  );
  const form = h(
    "form",
    { class: "upload-panel", "aria-label": "Unggah berkas" },
    h("div", { class: "upload-tab", "aria-hidden": "true" }, "Unggah berkas"),
    dropzone,
  );

  const sample = ["pdf", "docx", "xlsx", "png", "zip"].filter((e) => allowed.has(e)).map((e) => e.toUpperCase());
  const others = info.allowedExtensions.length - sample.length;
  const defaultHint =
    `${sample.join(", ")}${others > 0 ? `, dan ${others} jenis lainnya` : ""}. ` +
    `Maksimal ${Math.round(info.maxFileBytes / MB)} MB per berkas.`;

  /** Returns a problem with the selection, or null when it can be uploaded. */
  const problem = (): string | null => {
    const rejected = selected.find((f) => !allowed.has(extensionOf(f.name)));
    if (rejected) return `Jenis berkas ${rejected.name} tidak diizinkan.`;
    const big = selected.find((f) => f.size > info.maxFileBytes);
    if (big) return `${big.name} melebihi batas ${Math.round(info.maxFileBytes / MB)} MB per berkas.`;
    const total = selected.reduce((s, f) => s + f.size, 0);
    if (total > info.maxUploadBytes) return `Total ${formatSize(total)} melebihi batas ${Math.round(info.maxUploadBytes / MB)} MB per unggahan.`;
    return null;
  };

  const render = () => {
    const issue = problem();
    if (selected.length === 0) {
      title.textContent = "Tarik dan lepas berkas di area ini";
      hint.textContent = defaultHint;
    } else {
      const total = selected.reduce((s, f) => s + f.size, 0);
      title.textContent = selected.length === 1 ? selected[0].name : `${selected.length} berkas dipilih`;
      hint.textContent = issue ?? `Total ${formatSize(total)}. Pilih Unggah untuk menyimpan ke bucket.`;
    }
    hint.classList.toggle("is-error", issue !== null && selected.length > 0);
    submit.disabled = busy || selected.length === 0 || issue !== null;
    label.textContent = busy ? "Mengunggah" : "Unggah";
  };

  const select = (files: FileList | null) => {
    if (busy || !files) return;
    selected = [...files];
    render();
  };
  input.addEventListener("change", () => select(input.files));

  for (const type of ["dragenter", "dragover"]) {
    dropzone.addEventListener(type, (e) => {
      e.preventDefault();
      dropzone.classList.add("is-over");
    });
  }
  for (const type of ["dragleave", "drop"]) {
    dropzone.addEventListener(type, (e) => {
      e.preventDefault();
      dropzone.classList.remove("is-over");
    });
  }
  dropzone.addEventListener("drop", (e) => select((e as DragEvent).dataTransfer?.files ?? null));

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (submit.disabled) return;
    busy = true;
    progress.hidden = false;
    bar.style.width = "0%";
    render();
    try {
      const stored = await filesApi.upload(selected, (fraction) => {
        const pct = Math.round(fraction * 100);
        bar.style.width = `${pct}%`;
        progress.setAttribute("aria-valuenow", String(pct));
      });
      selected = [];
      input.value = "";
      onUploaded(stored);
    } catch (err) {
      onError(errorMessage(err));
    } finally {
      busy = false;
      progress.hidden = true;
      render();
    }
  });

  render();
  return form;
}
