import "./styles.css";
import { ApiError, deleteFile, getHealth, getInfo, listFiles } from "./api";
import { h } from "./dom";
import { icon } from "./icons";
import type { ServiceInfo } from "./types";
import { confirmDelete } from "./ui/confirmDialog";
import { createConnection } from "./ui/connection";
import { createFileTable } from "./ui/fileTable";
import { createNotices } from "./ui/notice";
import { createUploader } from "./ui/uploader";

const message = (err: unknown) => (err instanceof ApiError ? err.message : "Terjadi kesalahan yang tidak terduga.");

async function bootstrap(root: HTMLElement) {
  let info: ServiceInfo = { bucket: "", endpoint: "", maxUploadBytes: 64 * 1024 ** 2 };
  const notices = createNotices();
  try {
    info = await getInfo();
  } catch (err) {
    notices.error(message(err));
  }

  const connection = createConnection(info);

  const table = createFileTable({
    async onDelete(key) {
      if (!(await confirmDelete(key, info.bucket))) return;
      try {
        await deleteFile(key);
        notices.success(`${key} berhasil dihapus.`);
      } catch (err) {
        notices.error(message(err));
      }
      await refresh();
    },
  });

  const uploader = createUploader({
    maxBytes: info.maxUploadBytes,
    async onUploaded(keys) {
      notices.success(keys.length === 1 ? `${keys[0]} berhasil diunggah.` : `${keys.length} berkas berhasil diunggah.`);
      await refresh();
    },
    onError: (msg) => notices.error(msg),
  });

  async function refresh() {
    try {
      table.render(await listFiles());
      connection.set(true);
    } catch (err) {
      table.failed(message(err), () => {
        table.loading();
        void refresh();
      });
      connection.set(await getHealth());
    }
  }

  root.replaceChildren(
    h(
      "header",
      { class: "top" },
      h(
        "div",
        { class: "wrap" },
        h(
          "div",
          { class: "brand" },
          h("div", { class: "brand-mark" }, icon("storage")),
          h("div", {}, h("h1", {}, "File Storage"), h("p", {}, "Penyimpanan objek kompatibel S3")),
        ),
        connection.el,
      ),
    ),
    h(
      "main",
      { class: "wrap" },
      notices.el,
      uploader,
      table.el,
      info.bucket ? h("footer", {}, `Seluruh berkas disimpan sebagai objek di bucket ${info.bucket} pada RustFS.`) : null,
    ),
  );

  table.loading();
  await refresh();
}

void bootstrap(document.getElementById("app")!);
