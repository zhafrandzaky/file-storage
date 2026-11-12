import { ApiError, auth } from "../api";
import { h, input } from "../dom";
import { icon, type IconName } from "../icons";
import { roleLabel } from "../format";
import type { User } from "../types";
import type { Connection } from "../ui/connection";
import { field, formDialog } from "../ui/dialog";
import type { Notices } from "../ui/notice";

export type Route = "files" | "users" | "activity";

interface ShellOptions {
  user: User;
  connection: Connection;
  notices: Notices;
  onLogout(): void;
}

export interface Shell {
  main: HTMLElement;
  setRoute(route: Route): void;
}

const navItems: [Route, string, IconName][] = [
  ["files", "Berkas", "folder"],
  ["users", "Pengguna", "users"],
  ["activity", "Log aktivitas", "activity"],
];

function changePasswordDialog(notices: Notices) {
  const current = input({ type: "password", autocomplete: "current-password" });
  const next = input({ type: "password", autocomplete: "new-password" });
  const confirm = input({ type: "password", autocomplete: "new-password" });
  void formDialog({
    title: "Ganti kata sandi",
    submitLabel: "Simpan kata sandi",
    fields: [
      field("Kata sandi saat ini", current),
      field("Kata sandi baru", next, "Minimal 8 karakter."),
      field("Ulangi kata sandi baru", confirm),
    ],
    async onSubmit() {
      if (next.value !== confirm.value) throw new ApiError("Konfirmasi kata sandi baru tidak sama.", 0, "mismatch");
      await auth.changePassword(current.value, next.value);
    },
  }).then((ok) => ok && notices.success("Kata sandi berhasil diperbarui."));
}

/** Header, navigation and user menu around the active view. */
export function renderShell(root: HTMLElement, opts: ShellOptions): Shell {
  const { user } = opts;
  const main = h("main", { class: "wrap" });

  const menu = h("div", { class: "menu", role: "menu", hidden: true });
  const menuButton = h(
    "button",
    { class: "user-button", type: "button", "aria-haspopup": "menu", "aria-expanded": "false" },
    h("span", { class: "avatar", "aria-hidden": "true" }, user.name.trim().charAt(0).toUpperCase()),
    h("span", { class: "user-meta" }, h("strong", {}, user.name), h("span", {}, roleLabel(user.role))),
    icon("chevron"),
  );
  const toggleMenu = (open: boolean) => {
    menu.hidden = !open;
    menuButton.setAttribute("aria-expanded", String(open));
  };
  menuButton.addEventListener("click", (e) => {
    e.stopPropagation();
    toggleMenu(menu.hidden);
  });
  document.addEventListener("click", () => toggleMenu(false));
  document.addEventListener("keydown", (e) => e.key === "Escape" && toggleMenu(false));

  const menuItem = (label: string, iconName: IconName, action: () => void) => {
    const b = h("button", { class: "menu-item", type: "button", role: "menuitem" }, icon(iconName), label);
    b.addEventListener("click", () => {
      toggleMenu(false);
      action();
    });
    return b;
  };
  menu.append(
    h("div", { class: "menu-head" }, h("strong", {}, user.name), h("span", {}, user.email)),
    menuItem("Ganti kata sandi", "key", () => changePasswordDialog(opts.notices)),
    menuItem("Keluar", "logout", opts.onLogout),
  );

  const links = new Map<Route, HTMLAnchorElement>();
  const nav = user.role === "admin"
    ? h(
        "nav",
        { class: "nav", "aria-label": "Navigasi utama" },
        h(
          "div",
          { class: "wrap" },
          ...navItems.map(([route, label, iconName]) => {
            const a = h("a", { href: `#/${route}`, class: "nav-link" }, icon(iconName), label);
            links.set(route, a);
            return a;
          }),
        ),
      )
    : null;

  root.replaceChildren(
    h(
      "header",
      { class: "top" },
      h(
        "div",
        { class: "wrap top-bar" },
        h(
          "a",
          { class: "brand", href: "#/files" },
          h("div", { class: "brand-mark" }, icon("storage")),
          h("div", {}, h("h1", {}, "File Storage"), h("p", {}, "Penyimpanan objek kompatibel S3")),
        ),
        h("div", { class: "top-actions" }, opts.connection.el, h("div", { class: "user" }, menuButton, menu)),
      ),
      nav,
    ),
    main,
  );

  return {
    main,
    setRoute(route) {
      for (const [r, a] of links) {
        a.classList.toggle("is-active", r === route);
        if (r === route) a.setAttribute("aria-current", "page");
        else a.removeAttribute("aria-current");
      }
    },
  };
}
