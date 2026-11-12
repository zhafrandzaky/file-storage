type Attrs = Record<string, string | boolean | undefined>;
type Child = Node | string | null | undefined | false;

/** Minimal element builder. Text children are always set as text, never HTML. */
export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Attrs = {},
  ...children: Child[]
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  for (const [name, value] of Object.entries(attrs)) {
    if (value === undefined || value === false) continue;
    el.setAttribute(name, value === true ? "" : value);
  }
  for (const child of children) {
    if (child) el.append(child);
  }
  return el;
}

export function input(attrs: Attrs): HTMLInputElement {
  return h("input", { class: "input", ...attrs });
}

export function select(options: [value: string, label: string][], value: string): HTMLSelectElement {
  const el = h("select", { class: "input" }, ...options.map(([v, label]) => h("option", { value: v }, label)));
  el.value = value;
  return el;
}

/** Debounces calls to fn by ms. */
export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number): (...args: A) => void {
  let t: number | undefined;
  return (...args) => {
    window.clearTimeout(t);
    t = window.setTimeout(() => fn(...args), ms);
  };
}
