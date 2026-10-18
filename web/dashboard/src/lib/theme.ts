export type Theme = "light" | "dark" | "auto";

const KEY = "evechat:theme";

export function getTheme(): Theme {
  const saved = localStorage.getItem(KEY);
  return saved === "light" || saved === "dark" ? saved : "auto";
}

export function resolvedTheme(): "light" | "dark" {
  const t = getTheme();
  if (t !== "auto") return t;
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function applyTheme(theme: Theme): void {
  if (theme === "auto") localStorage.removeItem(KEY);
  else localStorage.setItem(KEY, theme);
  document.documentElement.dataset.theme = resolvedTheme();
}

export function toggleTheme(): "light" | "dark" {
  const next = resolvedTheme() === "dark" ? "light" : "dark";
  applyTheme(next);
  return next;
}
