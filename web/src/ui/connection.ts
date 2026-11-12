import { h } from "../dom";
import type { ServiceInfo } from "../types";

export interface Connection {
  el: HTMLElement;
  set(online: boolean): void;
}

/** Header indicator: a status dot and a single label reflecting real storage reachability. */
export function createConnection(info: ServiceInfo): Connection {
  const label = h("span", {}, "Memeriksa");
  const el = h(
    "div",
    { class: "conn is-checking", role: "status" },
    h("span", { class: "conn-dot", "aria-hidden": "true" }),
    label,
  );
  el.title = info.endpoint
    ? `Bucket ${info.bucket} pada ${info.endpoint.replace(/^https?:\/\//, "")}`
    : `Bucket ${info.bucket}`;

  return {
    el,
    set(online) {
      el.classList.remove("is-checking");
      el.classList.toggle("is-down", !online);
      label.textContent = online ? "Terhubung" : "Terputus";
    },
  };
}
