import { ApiError, errorMessage, usersApi } from "../api";
import { h, input, select } from "../dom";
import { formatDate, roleLabel } from "../format";
import { icon, type IconName } from "../icons";
import type { Role, User } from "../types";
import { field, formDialog } from "../ui/dialog";
import type { Notices } from "../ui/notice";

const roleOptions: [string, string][] = [
  ["member", "Anggota: unggah, unduh, hapus berkas sendiri"],
  ["admin", "Administrator: akses penuh"],
];

function textButton(label: string, iconName: IconName, extraClass = "") {
  return h("button", { class: `btn btn-small ${extraClass}`.trim(), type: "button" }, icon(iconName), label);
}

export function renderUsers(main: HTMLElement, me: User, notices: Notices) {
  const summary = h("p", {});
  const body = h("div", { class: "panel-body" });
  const add = h("button", { class: "btn btn-primary", type: "button" }, icon("plus"), "Tambah pengguna");

  main.replaceChildren(
    notices.el,
    h(
      "section",
      { class: "panel", "aria-labelledby": "users-title" },
      h("div", { class: "panel-head" }, h("div", {}, h("h2", { id: "users-title" }, "Pengguna"), summary), add),
      body,
    ),
  );

  const createDialog = () => {
    const name = input({ autocomplete: "off" });
    const email = input({ type: "email", autocomplete: "off" });
    const password = input({ type: "password", autocomplete: "new-password" });
    const role = select(roleOptions, "member");
    void formDialog({
      title: "Tambah pengguna",
      submitLabel: "Tambah pengguna",
      fields: [field("Nama lengkap", name), field("Email", email), field("Kata sandi awal", password, "Minimal 8 karakter. Sampaikan kepada pengguna secara langsung."), field("Peran", role)],
      onSubmit: async () => {
        await usersApi.create({ name: name.value, email: email.value, password: password.value, role: role.value as Role });
      },
    }).then((ok) => {
      if (!ok) return;
      notices.success(`${name.value.trim()} berhasil ditambahkan.`);
      void load();
    });
  };
  add.addEventListener("click", createDialog);

  const resetDialog = (u: User) => {
    const password = input({ type: "password", autocomplete: "new-password" });
    const confirm = input({ type: "password", autocomplete: "new-password" });
    void formDialog({
      title: "Atur ulang kata sandi",
      description: ["Kata sandi baru untuk ", h("b", {}, u.name), ". Seluruh sesi aktif pengguna ini akan diakhiri."],
      submitLabel: "Simpan kata sandi",
      fields: [field("Kata sandi baru", password, "Minimal 8 karakter."), field("Ulangi kata sandi baru", confirm)],
      onSubmit: async () => {
        if (password.value !== confirm.value) throw new ApiError("Konfirmasi kata sandi baru tidak sama.", 0, "mismatch");
        await usersApi.resetPassword(u.id, password.value);
      },
    }).then((ok) => ok && notices.success(`Kata sandi ${u.name} berhasil diatur ulang.`));
  };

  const update = async (u: User, patch: { role?: Role; active?: boolean }, message: string) => {
    try {
      await usersApi.update(u.id, patch);
      notices.success(message);
    } catch (err) {
      notices.error(errorMessage(err));
    }
    void load();
  };

  const row = (u: User) => {
    const self = u.id === me.id;
    const actions: Node[] = [];
    if (!self) {
      const role = textButton(u.role === "admin" ? "Jadikan anggota" : "Jadikan admin", "shield");
      role.addEventListener("click", () =>
        update(u, { role: u.role === "admin" ? "member" : "admin" }, `Peran ${u.name} diubah menjadi ${roleLabel(u.role === "admin" ? "member" : "admin").toLowerCase()}.`),
      );
      const toggle = textButton(u.active ? "Nonaktifkan" : "Aktifkan", "power", u.active ? "btn-small-danger" : "");
      toggle.addEventListener("click", () => update(u, { active: !u.active }, `${u.name} ${u.active ? "dinonaktifkan" : "diaktifkan kembali"}.`));
      const reset = textButton("Atur ulang sandi", "key");
      reset.addEventListener("click", () => resetDialog(u));
      actions.push(role, toggle, reset);
    }
    return h(
      "tr",
      { class: u.active ? "" : "is-muted" },
      h(
        "td",
        {},
        h(
          "div",
          { class: "person" },
          h("span", { class: "avatar", "aria-hidden": "true" }, u.name.trim().charAt(0).toUpperCase()),
          h("div", {}, h("strong", {}, u.name, self ? h("span", { class: "tag" }, "Anda") : null), h("small", {}, u.email)),
        ),
      ),
      h("td", {}, h("span", { class: `badge badge-${u.role}` }, roleLabel(u.role))),
      h("td", {}, h("span", { class: `status ${u.active ? "is-on" : "is-off"}` }, u.active ? "Aktif" : "Nonaktif")),
      h("td", { class: "when" }, formatDate(u.createdAt)),
      h("td", {}, h("div", { class: "actions" }, ...actions)),
    );
  };

  async function load() {
    try {
      const { users } = await usersApi.list();
      const admins = users.filter((u) => u.role === "admin" && u.active).length;
      summary.textContent = `${users.length} pengguna, ${admins} administrator aktif`;
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
              h("th", { scope: "col" }, "Pengguna"),
              h("th", { scope: "col" }, "Peran"),
              h("th", { scope: "col" }, "Status"),
              h("th", { scope: "col", class: "when" }, "Terdaftar"),
              h("th", { scope: "col" }, h("span", { class: "sr-only" }, "Aksi")),
            ),
          ),
          h("tbody", {}, ...users.map(row)),
        ),
      );
    } catch (err) {
      summary.textContent = "Daftar pengguna tidak tersedia";
      body.replaceChildren(h("div", { class: "empty" }, icon("alert"), h("strong", {}, "Daftar pengguna gagal dimuat"), h("p", {}, errorMessage(err))));
    }
  }

  body.replaceChildren(h("div", { class: "empty", "aria-busy": "true" }, h("p", {}, "Memuat daftar pengguna.")));
  void load();
}
