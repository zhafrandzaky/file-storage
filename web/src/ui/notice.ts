import { h } from "../dom";
import { icon } from "../icons";

export interface Notices {
  el: HTMLElement;
  success(message: string): void;
  error(message: string): void;
  clear(): void;
}

export function createNotices(): Notices {
  const el = h("div", { class: "notices" });
  let timer: number | undefined;

  const show = (kind: "ok" | "err", message: string) => {
    window.clearTimeout(timer);
    const close = h("button", { class: "notice-close", type: "button", "aria-label": "Tutup pemberitahuan" }, icon("close"));
    const notice = h(
      "div",
      { class: `notice ${kind}`, role: kind === "ok" ? "status" : "alert" },
      icon(kind === "ok" ? "check" : "alert"),
      h("span", {}, message),
      close,
    );
    close.addEventListener("click", () => notice.remove());
    el.replaceChildren(notice);
    if (kind === "ok") timer = window.setTimeout(() => notice.remove(), 6000);
  };

  return {
    el,
    success: (m) => show("ok", m),
    error: (m) => show("err", m),
    clear: () => el.replaceChildren(),
  };
}
