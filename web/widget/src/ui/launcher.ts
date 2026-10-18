export interface Launcher {
  el: HTMLButtonElement;
  setUnread(n: number): void;
}

const CHAT_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/></svg>';

const CLOSE_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>';

export function createLauncher(onClick: () => void): Launcher {
  const el = document.createElement("button");
  el.className = "ec-launcher";
  el.type = "button";
  el.setAttribute("aria-label", "EveChat");
  el.innerHTML = CHAT_ICON;

  const badge = document.createElement("span");
  badge.className = "ec-badge";
  el.appendChild(badge);

  el.addEventListener("click", onClick);

  return {
    el,
    setUnread(n: number) {
      if (n > 0) {
        badge.textContent = n > 9 ? "9+" : String(n);
        badge.classList.add("on");
      } else {
        badge.classList.remove("on");
      }
    },
  };
}

