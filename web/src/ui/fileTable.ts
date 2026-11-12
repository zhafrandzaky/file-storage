import { fileUrl } from "../api";
import { h } from "../dom";
import { extensionOf, formatDate, formatSize, kindOf } from "../format";
import { docSheet, icon } from "../icons";
import type { FileList, StoredFile } from "../types";

interface FileTableOptions {
  onDelete(key: string): void;
}

export interface FileTable {
  el: HTMLElement;
  render(list: FileList): void;
  loading(): void;
  failed(message: string, retry: () => void): void;
}

export function createFileTable(opts: FileTableOptions): FileTable {
  const summary = h("p", {});
  const searchInput = h("input", { type: "search", placeholder: "Cari berdasarkan nama", autocomplete: "off" });
  const search = h(
    "label",
    { class: "search", hidden: true },
    h("span", { class: "sr-only" }, "Cari berkas"),
    icon("search"),
    searchInput,
  );
  const body = h("div", { class: "docs-body" });

  const el = h(
    "section",
    { class: "docs", "aria-labelledby": "docs-title" },
    h("div", { class: "docs-head" }, h("div", {}, h("h2", { id: "docs-title" }, "Berkas"), summary), search),
    body,
  );

  let rows: HTMLTableRowElement[] = [];
  let noMatch: HTMLElement | null = null;

  const applySearch = () => {
    const term = searchInput.value.trim().toLowerCase();
    let shown = 0;
    for (const tr of rows) {
      const hit = tr.dataset.name!.toLowerCase().includes(term);
      tr.hidden = !hit;
      if (hit) shown++;
    }
    if (noMatch) noMatch.hidden = shown > 0;
  };
  searchInput.addEventListener("input", applySearch);

  const emptyState = (iconName: "cabinet" | "search" | "alert", heading: string, text: string, ...extra: Node[]) =>
    h("div", { class: "empty" }, icon(iconName), h("strong", {}, heading), h("p", {}, text), ...extra);

  const row = (f: StoredFile) => {
    const ext = extensionOf(f.key);
    const del = h("button", { class: "iconbtn del", type: "button", title: "Hapus", "aria-label": `Hapus ${f.key}` }, icon("trash"));
    del.addEventListener("click", () => opts.onDelete(f.key));

    const tr = h(
      "tr",
      {},
      h(
        "td",
        {},
        h(
          "div",
          { class: "name" },
          h("span", { class: `kind kind-${kindOf(ext)}`, "aria-hidden": "true" }, docSheet(), h("b", {}, ext || "file")),
          h(
            "div",
            {},
            h("a", { href: fileUrl(f.key), target: "_blank", rel: "noopener" }, f.key),
            h("small", {}, `${formatSize(f.size)}, ${formatDate(f.lastModified)}`),
          ),
        ),
      ),
      h("td", { class: "num" }, formatSize(f.size)),
      h("td", { class: "when" }, formatDate(f.lastModified)),
      h(
        "td",
        {},
        h(
          "div",
          { class: "actions" },
          h("a", { class: "iconbtn", href: fileUrl(f.key), target: "_blank", rel: "noopener", title: "Buka", "aria-label": `Buka ${f.key}` }, icon("open")),
          h("a", { class: "iconbtn", href: fileUrl(f.key, true), title: "Unduh", "aria-label": `Unduh ${f.key}` }, icon("download")),
          del,
        ),
      ),
    );
    tr.dataset.name = f.key;
    return tr;
  };

  return {
    el,
    loading() {
      summary.textContent = "Memuat daftar berkas";
      body.replaceChildren(h("div", { class: "empty", "aria-busy": "true" }, h("p", {}, "Memuat berkas dari bucket.")));
    },
    failed(message, retry) {
      summary.textContent = "Daftar berkas tidak tersedia";
      search.hidden = true;
      const btn = h("button", { class: "btn btn-ghost", type: "button" }, icon("refresh"), "Muat ulang");
      btn.addEventListener("click", retry);
      body.replaceChildren(emptyState("alert", "Daftar berkas gagal dimuat", message, btn));
    },
    render(list) {
      summary.textContent = `${list.files.length} berkas, total ${formatSize(list.totalSize)}`;
      search.hidden = list.files.length === 0;
      if (list.files.length === 0) {
        rows = [];
        noMatch = null;
        body.replaceChildren(emptyState("cabinet", "Belum ada berkas", "Berkas yang Anda unggah akan tampil di sini."));
        return;
      }
      rows = list.files.map(row);
      noMatch = emptyState("search", "Tidak ada berkas yang cocok", "Periksa kembali kata kunci atau kosongkan kolom pencarian.");
      noMatch.hidden = true;
      body.replaceChildren(
        h(
          "table",
          {},
          h(
            "thead",
            {},
            h(
              "tr",
              {},
              h("th", { scope: "col" }, "Nama"),
              h("th", { scope: "col", class: "num col-size" }, "Ukuran"),
              h("th", { scope: "col", class: "col-when" }, "Terakhir diubah"),
              h("th", { scope: "col" }, h("span", { class: "sr-only" }, "Aksi")),
            ),
          ),
          h("tbody", {}, ...rows),
        ),
        noMatch,
      );
      applySearch();
    },
  };
}
