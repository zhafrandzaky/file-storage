import { h } from "../dom";

/** Asks the user to confirm deleting a document. Resolves true on confirm. */
export function confirmDelete(name: string): Promise<boolean> {
  const cancel = h("button", { class: "btn btn-ghost", value: "cancel" }, "Batal");
  const ok = h("button", { class: "btn btn-danger", value: "ok" }, "Hapus");
  const dialog = h(
    "dialog",
    { "aria-labelledby": "confirm-title" },
    h(
      "form",
      { method: "dialog" },
      h(
        "div",
        { class: "dlg-body" },
        h("h3", { id: "confirm-title" }, "Hapus dokumen?"),
        h("p", {}, h("b", {}, name), " akan dihapus permanen dari bucket."),
      ),
      h("div", { class: "dlg-foot" }, cancel, ok),
    ),
  );
  document.body.append(dialog);

  return new Promise((resolve) => {
    dialog.addEventListener("close", () => {
      resolve(dialog.returnValue === "ok");
      dialog.remove();
    });
    dialog.showModal();
    cancel.focus();
  });
}
