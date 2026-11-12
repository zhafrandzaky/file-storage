import { auditApi, errorMessage } from "../api";
import { h } from "../dom";
import { actionLabels, formatDateTime } from "../format";
import { icon } from "../icons";
import type { AuditEntry } from "../types";
import type { Notices } from "../ui/notice";

const LIMIT = 200;

export function renderActivity(main: HTMLElement, notices: Notices) {
  const summary = h("p", {});
  const body = h("div", { class: "panel-body" });
  const refresh = h("button", { class: "btn btn-ghost", type: "button" }, icon("refresh"), "Muat ulang");

  main.replaceChildren(
    notices.el,
    h(
      "section",
      { class: "panel", "aria-labelledby": "activity-title" },
      h("div", { class: "panel-head" }, h("div", {}, h("h2", { id: "activity-title" }, "Log aktivitas"), summary), refresh),
      body,
    ),
  );

  const row = (e: AuditEntry) =>
    h(
      "tr",
      { class: e.action === "auth.login_failed" ? "is-warning" : "" },
      h("td", { class: "when" }, formatDateTime(e.createdAt)),
      h("td", {}, e.actorEmail || "Sistem"),
      h("td", {}, h("span", { class: "action" }, actionLabels[e.action] ?? e.action)),
      h("td", { class: "target" }, e.target || "", e.detail ? h("small", {}, e.detail) : null),
      h("td", { class: "ip" }, e.ip),
    );

  async function load() {
    refresh.disabled = true;
    try {
      const { entries } = await auditApi.list(LIMIT);
      summary.textContent = entries.length < LIMIT ? `${entries.length} aktivitas tercatat` : `${LIMIT} aktivitas terbaru`;
      body.replaceChildren(
        entries.length === 0
          ? h("div", { class: "empty" }, icon("activity"), h("strong", {}, "Belum ada aktivitas"), h("p", {}, "Aktivitas masuk, unggah, dan perubahan pengguna akan tercatat di sini."))
          : h(
              "table",
              {},
              h(
                "thead",
                {},
                h(
                  "tr",
                  {},
                  h("th", { scope: "col", class: "when" }, "Waktu"),
                  h("th", { scope: "col" }, "Pengguna"),
                  h("th", { scope: "col" }, "Aktivitas"),
                  h("th", { scope: "col" }, "Objek"),
                  h("th", { scope: "col", class: "ip" }, "Alamat IP"),
                ),
              ),
              h("tbody", {}, ...entries.map(row)),
            ),
      );
    } catch (err) {
      summary.textContent = "Log aktivitas tidak tersedia";
      body.replaceChildren(h("div", { class: "empty" }, icon("alert"), h("strong", {}, "Log aktivitas gagal dimuat"), h("p", {}, errorMessage(err))));
    } finally {
      refresh.disabled = false;
    }
  }
  refresh.addEventListener("click", () => void load());

  body.replaceChildren(h("div", { class: "empty", "aria-busy": "true" }, h("p", {}, "Memuat log aktivitas.")));
  void load();
}
