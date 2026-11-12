// Custom stroke icons drawn on a 24px grid, 1.6px stroke.
const paths = {
  storage: '<rect x="4" y="3" width="16" height="18" rx="1.5"/><path d="M4 12h16M10 7.5h4M10 16.5h4"/>',
  upload: '<path d="M12 15V4M7.5 8.5 12 4l4.5 4.5"/><path d="M5 14v4.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V14"/>',
  uploadTray:
    '<path d="M3.5 7.5A1.5 1.5 0 0 1 5 6h4.5l2 2H19a1.5 1.5 0 0 1 1.5 1.5v8A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5z"/><path d="M12 11v5M9.75 13.75 12 16l2.25-2.25"/>',
  search: '<circle cx="11" cy="11" r="6"/><path d="m20 20-4.5-4.5"/>',
  open: '<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><circle cx="12" cy="12" r="2.75"/>',
  download: '<path d="M12 4v11M7.5 10.5 12 15l4.5-4.5M5 20h14"/>',
  trash:
    '<path d="M4.5 7h15M9.5 7V5h5v2M6.5 7l.8 12a1.5 1.5 0 0 0 1.5 1.4h6.4a1.5 1.5 0 0 0 1.5-1.4l.8-12M10 11v6M14 11v6"/>',
  link: '<path d="M10 14a4 4 0 0 0 5.66 0l3-3a4 4 0 0 0-5.66-5.66l-1 1"/><path d="M14 10a4 4 0 0 0-5.66 0l-3 3a4 4 0 0 0 5.66 5.66l1-1"/>',
  copy: '<rect x="8.5" y="8.5" width="11" height="11" rx="1.5"/><path d="M15.5 5.5v-1A1.5 1.5 0 0 0 14 3H6A1.5 1.5 0 0 0 4.5 4.5V13A1.5 1.5 0 0 0 6 14.5h1"/>',
  check: '<circle cx="12" cy="12" r="8.5"/><path d="m8.5 12.2 2.4 2.3 4.6-4.8"/>',
  tick: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  alert: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.75v5M12 16.2v.05"/>',
  close: '<path d="m7 7 10 10M17 7 7 17"/>',
  refresh: '<path d="M19.5 12a7.5 7.5 0 1 1-2.2-5.3"/><path d="M19.5 4.5v4h-4"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  user: '<circle cx="12" cy="8.5" r="3.5"/><path d="M5 19.5c1.2-3.2 3.8-5 7-5s5.8 1.8 7 5"/>',
  users:
    '<circle cx="9" cy="8.5" r="3.25"/><path d="M3 19c1-2.9 3.2-4.5 6-4.5s5 1.6 6 4.5"/><path d="M15.5 5.6a3.25 3.25 0 0 1 0 5.8M17.5 14.8c1.6.6 2.8 2 3.5 4.2"/>',
  activity: '<path d="M3 12h4l2.5-6 5 12 2.5-6H21"/>',
  folder: '<path d="M3.5 7.5A1.5 1.5 0 0 1 5 6h4.5l2 2H19a1.5 1.5 0 0 1 1.5 1.5v8A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5z"/>',
  key: '<circle cx="8" cy="15" r="3.5"/><path d="m10.5 12.5 8-8M15.5 7.5l2.5 2.5M13.5 9.5l2 2"/>',
  power: '<path d="M12 4v7"/><path d="M7.2 6.8a7 7 0 1 0 9.6 0"/>',
  shield: '<path d="M12 3.5 5 6v5.5c0 4.3 3 7.6 7 9 4-1.4 7-4.7 7-9V6z"/>',
  logout: '<path d="M14 4.5H6.5A1.5 1.5 0 0 0 5 6v12a1.5 1.5 0 0 0 1.5 1.5H14"/><path d="M10 12h10M16.5 8.5 20 12l-3.5 3.5"/>',
  chevron: '<path d="m7 10 5 5 5-5"/>',
} as const;

export type IconName = keyof typeof paths;

export function icon(name: IconName, className = "icon"): SVGSVGElement {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("class", className);
  svg.setAttribute("aria-hidden", "true");
  svg.innerHTML = paths[name];
  return svg;
}

/** File sheet with a folded corner; the extension label sits on top. */
export function fileSheet(): SVGSVGElement {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", "0 0 34 40");
  svg.setAttribute("aria-hidden", "true");
  svg.innerHTML =
    '<path d="M5 2.5h16l8.5 8.5v25a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 36V4A1.5 1.5 0 0 1 5 2.5Z" fill="#fff" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/><path d="M21 2.5V11h8.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>';
  return svg;
}
