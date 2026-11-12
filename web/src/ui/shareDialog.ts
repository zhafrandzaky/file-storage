import { filesApi } from "../api";
import { h, select } from "../dom";
import { formatDateTime, formatDuration } from "../format";
import { icon } from "../icons";
import type { StoredFile } from "../types";
import { button, createDialog, field } from "./dialog";

const presets = [1, 24, 72, 168];

/** Creates a time-limited download link served directly by object storage. */
export function openShareDialog(file: StoredFile, maxHours: number, onShared: () => void) {
  const shell = createDialog("Bagikan berkas");
  const options = presets.filter((p) => p <= maxHours).map((p): [string, string] => [String(p), formatDuration(p)]);
  const duration = select(options, options.some(([v]) => v === "24") ? "24" : options[0][0]);

  const create = button("Buat tautan", "primary");
  const close = button("Tutup", "ghost");
  const result = h("div", { class: "share-result", hidden: true });

  shell.body.append(
    h("p", { class: "dlg-text" }, "Siapa pun yang memiliki tautan dapat mengunduh ", h("b", {}, file.name), " tanpa masuk, hingga masa berlakunya habis."),
    h("div", { class: "dlg-form" }, field("Masa berlaku", duration)),
    result,
  );
  shell.footer.append(close, create);
  close.addEventListener("click", () => shell.close());

  create.addEventListener("click", async () => {
    create.disabled = true;
    shell.error.hidden = true;
    try {
      const link = await filesApi.share(file.id, Number(duration.value));
      const url = h("input", { class: "input", readonly: true, value: link.url, "aria-label": "Tautan berbagi" });
      const copyLabel = h("span", {}, "Salin");
      const copy = h("button", { class: "btn btn-ghost", type: "button" }, icon("copy"), copyLabel);
      copy.addEventListener("click", async () => {
        url.select();
        try {
          await navigator.clipboard.writeText(link.url);
        } catch {
          document.execCommand("copy");
        }
        copyLabel.textContent = "Disalin";
        window.setTimeout(() => (copyLabel.textContent = "Salin"), 2000);
      });
      result.replaceChildren(
        h("div", { class: "share-row" }, url, copy),
        h("span", { class: "field-hint" }, `Berlaku hingga ${formatDateTime(link.expiresAt)}.`),
      );
      result.hidden = false;
      create.textContent = "Buat tautan baru";
      url.focus();
      url.select();
      onShared();
    } catch (err) {
      shell.showError(err);
    } finally {
      create.disabled = false;
    }
  });
  duration.focus();
}
