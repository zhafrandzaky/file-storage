import { auth, errorMessage } from "../api";
import { h, input } from "../dom";
import { icon } from "../icons";
import type { User } from "../types";
import { field } from "../ui/dialog";

/** Sign-in screen. Resolves with the signed-in user. */
export function renderLogin(root: HTMLElement, notice?: string): Promise<User> {
  return new Promise((resolve) => {
    const email = input({ type: "email", name: "email", autocomplete: "username", required: true });
    const password = input({ type: "password", name: "password", autocomplete: "current-password", required: true });
    const error = h("div", { class: "login-error", role: "alert", hidden: !notice }, notice ? icon("alert") : null, notice ?? "");
    const submit = h("button", { class: "btn btn-primary btn-block", type: "submit" }, "Masuk");

    const form = h(
      "form",
      { class: "login-form", novalidate: true },
      field("Email", email),
      field("Kata sandi", password),
      error,
      submit,
    );

    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (!email.value.trim() || !password.value) {
        error.replaceChildren(icon("alert"), "Isi email dan kata sandi terlebih dahulu.");
        error.hidden = false;
        return;
      }
      submit.disabled = true;
      submit.textContent = "Memproses";
      try {
        resolve(await auth.login(email.value, password.value));
      } catch (err) {
        error.replaceChildren(icon("alert"), errorMessage(err));
        error.hidden = false;
        password.value = "";
        password.focus();
        submit.disabled = false;
        submit.textContent = "Masuk";
      }
    });

    root.replaceChildren(
      h(
        "main",
        { class: "login" },
        h(
          "div",
          { class: "login-panel" },
          h(
            "div",
            { class: "brand" },
            h("div", { class: "brand-mark" }, icon("storage")),
            h("div", {}, h("h1", {}, "File Storage"), h("p", {}, "Penyimpanan objek kompatibel S3")),
          ),
          h("h2", { class: "login-title" }, "Masuk ke akun Anda"),
          form,
        ),
        h("p", { class: "login-foot" }, "Akun dibuat oleh administrator. Hubungi administrator jika Anda belum memiliki akses."),
      ),
    );
    email.focus();
  });
}
