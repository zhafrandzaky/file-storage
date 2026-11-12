import { errorMessage } from "../api";
import { h } from "../dom";
import { icon } from "../icons";

type Child = Node | string | null | false | undefined;

export interface DialogShell {
  dialog: HTMLDialogElement;
  body: HTMLElement;
  footer: HTMLElement;
  error: HTMLElement;
  close(): void;
  showError(err: unknown): void;
}

/** Modal frame with a title, body and footer. Removed from the DOM on close. */
export function createDialog(title: string, onClose: () => void = () => {}): DialogShell {
  const id = `dlg-${Math.random().toString(36).slice(2)}`;
  const body = h("div", { class: "dlg-body" });
  const error = h("div", { class: "dlg-error", role: "alert", hidden: true });
  const footer = h("div", { class: "dlg-foot" });
  const dialog = h(
    "dialog",
    { "aria-labelledby": id },
    h("div", { class: "dlg-head" }, h("h3", { id }, title)),
    body,
    error,
    footer,
  );
  dialog.addEventListener("close", () => {
    dialog.remove();
    onClose();
  });
  document.body.append(dialog);
  dialog.showModal();
  return {
    dialog,
    body,
    footer,
    error,
    close: () => dialog.close(),
    showError(err) {
      error.replaceChildren(icon("alert"), h("span", {}, errorMessage(err)));
      error.hidden = false;
    },
  };
}

export function field(label: string, control: HTMLElement, hint?: string): HTMLElement {
  return h("label", { class: "field" }, h("span", { class: "field-label" }, label), control, hint ? h("span", { class: "field-hint" }, hint) : null);
}

export function button(label: string, variant: "primary" | "ghost" | "danger", type: "button" | "submit" = "button"): HTMLButtonElement {
  return h("button", { class: `btn btn-${variant}`, type }, label);
}

interface FormDialogOptions {
  title: string;
  description?: Child[];
  fields?: HTMLElement[];
  submitLabel: string;
  danger?: boolean;
  /** Throw to keep the dialog open and show the error. */
  onSubmit: () => Promise<void>;
}

/** Dialog with a form; resolves true when onSubmit succeeded. */
export function formDialog(opts: FormDialogOptions): Promise<boolean> {
  return new Promise((resolve) => {
    let done = false;
    const shell = createDialog(opts.title, () => resolve(done));
    const cancel = button("Batal", "ghost");
    const submit = button(opts.submitLabel, opts.danger ? "danger" : "primary", "submit");
    const formId = `form-${Math.random().toString(36).slice(2)}`;
    const form = h(
      "form",
      { class: "dlg-form", id: formId, novalidate: true },
      opts.description ? h("p", { class: "dlg-text" }, ...opts.description) : null,
      ...(opts.fields ?? []),
    );
    shell.body.append(form);
    shell.footer.append(cancel, submit);
    // The submit button lives in the footer but belongs to the form.
    submit.setAttribute("form", formId);
    cancel.addEventListener("click", () => shell.close());

    const run = async (e: Event) => {
      e.preventDefault();
      submit.disabled = cancel.disabled = true;
      shell.error.hidden = true;
      try {
        await opts.onSubmit();
        done = true;
        shell.close();
      } catch (err) {
        shell.showError(err);
        submit.disabled = cancel.disabled = false;
      }
    };
    form.addEventListener("submit", run);
    const first = form.querySelector<HTMLElement>("input, select");
    (first ?? cancel).focus();
  });
}
