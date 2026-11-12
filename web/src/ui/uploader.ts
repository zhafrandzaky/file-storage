import { uploadFiles, ApiError } from "../api";
import { h } from "../dom";
import { formatSize } from "../format";
import { icon } from "../icons";

interface UploaderOptions {
  maxBytes: number;
  onUploaded(keys: string[]): void;
  onError(message: string): void;
}

export function createUploader(opts: UploaderOptions): HTMLElement {
  let selected: File[] = [];
  let busy = false;

  const input = h("input", { class: "sr-only", type: "file", id: "files", multiple: true });
  const title = h("strong", {});
  const hint = h("span", {});
  const bar = h("div", { class: "progress-bar" });
  const progress = h(
    "div",
    { class: "progress", role: "progressbar", "aria-label": "Progres unggahan", "aria-valuemin": "0", "aria-valuemax": "100", hidden: true },
    bar,
  );
  const submit = h("button", { class: "btn btn-primary", type: "submit", disabled: true }, icon("upload"), h("span", {}, "Unggah"));
  const label = submit.querySelector("span")!;

  const drop = h(
    "div",
    { class: "drop" },
    h("div", { class: "drop-icon" }, icon("folderIn")),
    h("div", { class: "drop-text" }, title, hint, progress),
    h("div", { class: "drop-actions" }, h("label", { class: "btn btn-plain", for: "files" }, "Pilih berkas"), input, submit),
  );

  const form = h(
    "form",
    { class: "folder", "aria-label": "Unggah dokumen" },
    h("div", { class: "folder-tab", "aria-hidden": "true" }, "Unggah dokumen"),
    drop,
  );

  const total = () => selected.reduce((sum, f) => sum + f.size, 0);
  const limitMB = Math.round(opts.maxBytes / 1024 ** 2);

  const render = () => {
    if (selected.length === 0) {
      title.textContent = "Tarik berkas ke sini";
      hint.textContent = `PDF, gambar, dokumen, atau spreadsheet. Maksimal ${limitMB} MB per unggahan.`;
    } else {
      title.textContent = selected.length === 1 ? selected[0].name : `${selected.length} berkas dipilih`;
      hint.textContent =
        total() > opts.maxBytes
          ? `Total ${formatSize(total())} melebihi batas ${limitMB} MB. Kurangi jumlah berkas.`
          : `Total ${formatSize(total())}. Klik Unggah untuk menyimpan ke bucket.`;
    }
    submit.disabled = busy || selected.length === 0 || total() > opts.maxBytes;
    label.textContent = busy ? "Mengunggah" : "Unggah";
  };

  const select = (files: FileList | null) => {
    if (busy || !files) return;
    selected = [...files];
    render();
  };

  input.addEventListener("change", () => select(input.files));

  for (const type of ["dragenter", "dragover"]) {
    drop.addEventListener(type, (e) => {
      e.preventDefault();
      drop.classList.add("is-over");
    });
  }
  for (const type of ["dragleave", "drop"]) {
    drop.addEventListener(type, (e) => {
      e.preventDefault();
      drop.classList.remove("is-over");
    });
  }
  drop.addEventListener("drop", (e) => select((e as DragEvent).dataTransfer?.files ?? null));

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (submit.disabled) return;
    busy = true;
    progress.hidden = false;
    bar.style.width = "0%";
    render();
    try {
      const keys = await uploadFiles(selected, (fraction) => {
        const pct = Math.round(fraction * 100);
        bar.style.width = `${pct}%`;
        progress.setAttribute("aria-valuenow", String(pct));
      });
      selected = [];
      input.value = "";
      opts.onUploaded(keys);
    } catch (err) {
      opts.onError(err instanceof ApiError ? err.message : "Unggahan gagal. Coba lagi.");
    } finally {
      busy = false;
      progress.hidden = true;
      render();
    }
  });

  render();
  return form;
}
