import { errorMessage, fileUrl, filesApi } from "../api";
import { debounce, h } from "../dom";
import { extensionOf, formatDateTime, formatSize, kindLabels } from "../format";
import { fileSheet, icon, type IconName } from "../icons";
import type { FileKind, FileListResponse, ServiceInfo, StoredFile, User } from "../types";
import type { Connection } from "../ui/connection";
import { formDialog } from "../ui/dialog";
import type { Notices } from "../ui/notice";
import { openShareDialog } from "../ui/shareDialog";
import { createUploader } from "../ui/uploader";

interface FilesViewOptions {
  user: User;
  info: ServiceInfo;
  notices: Notices;
  connection: Connection;
}

const kinds: FileKind[] = ["document", "image", "spreadsheet", "archive", "other"];

function emptyState(iconName: IconName, heading: string, text: string, ...extra: Node[]) {
  return h("div", { class: "empty" }, icon(iconName), h("strong", {}, heading), h("p", {}, text), ...extra);
}

function iconLink(href: string, label: string, iconName: IconName, newTab = false) {
  return h(
    "a",
    { class: "iconbtn", href, title: label, "aria-label": label, target: newTab ? "_blank" : undefined, rel: newTab ? "noopener" : undefined },
    icon(iconName),
  );
}

function iconButton(label: string, iconName: IconName, extraClass = "") {
  return h("button", { class: `iconbtn ${extraClass}`.trim(), type: "button", title: label, "aria-label": label }, icon(iconName));
}

export function renderFiles(main: HTMLElement, opts: FilesViewOptions) {
  const { user, info, notices } = opts;
  let kind: FileKind | "" = "";
  let query = "";

  const summary = h("p", {});
  const search = h("input", { class: "input", type: "search", placeholder: "Cari berdasarkan nama", autocomplete: "off" });
  const tabs = h("div", { class: "tabs", role: "tablist", "aria-label": "Jenis berkas" });
  const body = h("div", { class: "panel-body" });

  const panel = h(
    "section",
    { class: "panel", "aria-labelledby": "files-title" },
    h(
      "div",
      { class: "panel-head" },
      h("div", {}, h("h2", { id: "files-title" }, "Berkas"), summary),
      h("label", { class: "search" }, h("span", { class: "sr-only" }, "Cari berkas"), icon("search"), search),
    ),
    tabs,
    body,
  );

  const renderTabs = (data: FileListResponse) => {
    const entries: [FileKind | "", string, number][] = [
      ["", "Semua", data.summary.count],
      ...kinds.map((k): [FileKind, string, number] => [k, kindLabels[k], data.summary.byKind[k] ?? 0]),
    ];
    tabs.replaceChildren(
      ...entries
        .filter(([k, , n]) => k === "" || k === kind || n > 0)
        .map(([k, label, n]) => {
          const tab = h(
            "button",
            { class: "tab", type: "button", role: "tab", "aria-selected": String(k === kind) },
            label,
            h("span", { class: "tab-count" }, String(n)),
          );
          tab.addEventListener("click", () => {
            kind = k;
            void load();
          });
          return tab;
        }),
    );
  };

  const confirmDelete = (file: StoredFile) =>
    formDialog({
      title: "Hapus berkas?",
      description: [h("b", {}, file.name), ` akan dihapus secara permanen dari bucket ${info.bucket}. Tindakan ini tidak dapat dibatalkan.`],
      submitLabel: "Hapus berkas",
      danger: true,
      onSubmit: () => filesApi.remove(file.id),
    }).then((ok) => {
      if (!ok) return;
      notices.success(`${file.name} berhasil dihapus.`);
      void load();
    });

  const row = (f: StoredFile) => {
    const ext = extensionOf(f.name);
    const own = f.ownerId === user.id;
    const actions: Node[] = [iconLink(fileUrl(f.id), `Buka ${f.name}`, "open", true), iconLink(fileUrl(f.id, true), `Unduh ${f.name}`, "download")];
    if (info.shareEnabled) {
      const share = iconButton(`Bagikan ${f.name}`, "link");
      share.addEventListener("click", () => openShareDialog(f, info.shareMaxHours, () => {}));
      actions.push(share);
    }
    if (user.role === "admin" || own) {
      const del = iconButton(`Hapus ${f.name}`, "trash", "danger");
      del.addEventListener("click", () => void confirmDelete(f));
      actions.push(del);
    } else {
      // Keeps the action columns aligned across rows.
      actions.push(h("span", { class: "iconbtn-spacer", "aria-hidden": "true" }));
    }
    return h(
      "tr",
      {},
      h(
        "td",
        {},
        h(
          "div",
          { class: "name" },
          h("span", { class: `kind kind-${f.kind}`, "aria-hidden": "true" }, fileSheet(), h("b", {}, ext || "file")),
          h(
            "div",
            {},
            h("a", { href: fileUrl(f.id), target: "_blank", rel: "noopener" }, f.name),
            h("small", {}, `${formatSize(f.size)}, ${formatDateTime(f.createdAt)}`),
          ),
        ),
      ),
      h("td", { class: "owner" }, own ? "Anda" : f.ownerName),
      h("td", { class: "num" }, formatSize(f.size)),
      h("td", { class: "when" }, formatDateTime(f.createdAt)),
      h("td", {}, h("div", { class: "actions" }, ...actions)),
    );
  };

  const renderList = (data: FileListResponse) => {
    const filtered = kind !== "" || query.trim() !== "";
    summary.textContent = `${data.summary.count} berkas, total ${formatSize(data.summary.totalSize)}`;
    renderTabs(data);
    if (data.files.length === 0) {
      body.replaceChildren(
        filtered
          ? emptyState("search", "Tidak ada berkas yang cocok", "Periksa kembali kata kunci atau pilih jenis berkas lain.")
          : emptyState("storage", "Belum ada berkas", "Berkas yang Anda unggah akan tampil di sini."),
      );
      return;
    }
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
            h("th", { scope: "col", class: "owner" }, "Pemilik"),
            h("th", { scope: "col", class: "num" }, "Ukuran"),
            h("th", { scope: "col", class: "when" }, "Diunggah"),
            h("th", { scope: "col" }, h("span", { class: "sr-only" }, "Aksi")),
          ),
        ),
        h("tbody", {}, ...data.files.map(row)),
      ),
    );
  };

  let seq = 0;
  async function load() {
    const mine = ++seq;
    try {
      const data = await filesApi.list(kind, query);
      if (mine !== seq) return;
      renderList(data);
      opts.connection.set(true);
    } catch (err) {
      if (mine !== seq) return;
      summary.textContent = "Daftar berkas tidak tersedia";
      const retry = h("button", { class: "btn btn-ghost", type: "button" }, icon("refresh"), "Muat ulang");
      retry.addEventListener("click", () => void load());
      body.replaceChildren(emptyState("alert", "Daftar berkas gagal dimuat", errorMessage(err), retry));
    }
  }

  search.addEventListener(
    "input",
    debounce(() => {
      query = search.value;
      void load();
    }, 250),
  );

  const uploader = createUploader({
    info,
    onUploaded(stored) {
      notices.success(stored.length === 1 ? `${stored[0].name} berhasil diunggah.` : `${stored.length} berkas berhasil diunggah.`);
      void load();
    },
    onError: (msg) => notices.error(msg),
  });

  body.replaceChildren(h("div", { class: "empty", "aria-busy": "true" }, h("p", {}, "Memuat berkas dari bucket.")));
  main.replaceChildren(notices.el, uploader, panel, h("footer", {}, `Seluruh berkas disimpan sebagai objek di bucket ${info.bucket} pada RustFS.`));
  void load();
}
