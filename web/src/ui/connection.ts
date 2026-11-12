import { h } from "../dom";
import type { ServiceInfo } from "../types";

export interface Connection {
  el: HTMLElement;
  set(online: boolean): void;
}

/** Header indicator: which bucket the app writes to and whether it is reachable. */
export function createConnection(info: ServiceInfo): Connection {
  const status = h("strong", {});
  const detail = h("span", {});
  const el = h(
    "div",
    { class: "conn", role: "status" },
    h("span", { class: "conn-dot", "aria-hidden": "true" }),
    h("div", { class: "conn-text" }, status, detail),
  );

  const host = info.endpoint.replace(/^https?:\/\//, "");
  detail.textContent = info.bucket ? `Bucket ${info.bucket} pada ${host}` : "Konfigurasi penyimpanan tidak tersedia";

  return {
    el,
    set(online) {
      el.classList.toggle("is-down", !online);
      status.textContent = online ? "Terhubung" : "Penyimpanan tidak terhubung";
    },
  };
}
