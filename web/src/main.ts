import "./styles.css";
import { auth, errorMessage, getHealth, getInfo, setUnauthorizedHandler } from "./api";
import { h } from "./dom";
import { icon } from "./icons";
import type { ServiceInfo, User } from "./types";
import { createConnection } from "./ui/connection";
import { createNotices } from "./ui/notice";
import { renderActivity } from "./views/activity";
import { renderFiles } from "./views/files";
import { renderLogin } from "./views/login";
import { renderShell, type Route } from "./views/shell";
import { renderUsers } from "./views/users";

const root = document.getElementById("app")!;
let teardown: () => void = () => {};

function routeFromHash(user: User): Route {
  const route = location.hash.replace(/^#\//, "");
  if (user.role === "admin" && (route === "users" || route === "activity")) return route;
  return "files";
}

async function start(loginNotice?: string) {
  teardown();
  teardown = () => {};
  let user: User;
  try {
    user = loginNotice ? await renderLogin(root, loginNotice) : await auth.me();
  } catch {
    user = await renderLogin(root);
  }
  await signedIn(user);
}

async function signedIn(user: User) {
  let info: ServiceInfo;
  try {
    info = await getInfo();
  } catch (err) {
    root.replaceChildren(
      h("main", { class: "wrap" }, h("div", { class: "empty" }, icon("alert"), h("strong", {}, "Aplikasi tidak dapat dimuat"), h("p", {}, errorMessage(err)))),
    );
    return;
  }

  const notices = createNotices();
  const connection = createConnection(info);
  const shell = renderShell(root, {
    user,
    connection,
    notices,
    async onLogout() {
      try {
        await auth.logout();
      } finally {
        history.replaceState(null, "", location.pathname);
        void start();
      }
    },
  });

  const show = () => {
    const route = routeFromHash(user);
    if (location.hash !== `#/${route}`) history.replaceState(null, "", `#/${route}`);
    shell.setRoute(route);
    notices.clear();
    if (route === "users") renderUsers(shell.main, user, notices);
    else if (route === "activity") renderActivity(shell.main, notices);
    else renderFiles(shell.main, { user, info, notices, connection });
  };
  window.addEventListener("hashchange", show);

  const checkHealth = async () => connection.set(await getHealth());
  void checkHealth();
  const timer = window.setInterval(checkHealth, 15000);

  teardown = () => {
    window.removeEventListener("hashchange", show);
    window.clearInterval(timer);
    document.querySelectorAll("dialog").forEach((d) => d.remove());
  };
  show();
}

setUnauthorizedHandler(() => void start("Sesi Anda telah berakhir. Silakan masuk kembali."));
void start();
