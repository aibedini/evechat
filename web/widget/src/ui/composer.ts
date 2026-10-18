import { t } from "../i18n";

export interface Composer {
  el: HTMLFormElement;
  input: HTMLTextAreaElement;
  value(): string;
  clear(): void;
  focus(): void;
  setEnabled(on: boolean): void;
}

const SEND_ICON =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>';

export function createComposer(
  locale: string,
  onSend: (text: string) => void,
  onTyping: (typing: boolean) => void
): Composer {
  const strings = t(locale);
  const el = document.createElement("form");
  el.className = "ec-composer";

  const input = document.createElement("textarea");
  input.className = "ec-input";
  input.rows = 1;
  input.placeholder = strings.placeholder;
  input.setAttribute("aria-label", strings.placeholder);

  const send = document.createElement("button");
  send.className = "ec-send";
  send.type = "submit";
  send.setAttribute("aria-label", strings.send);
  send.innerHTML = SEND_ICON;
  send.disabled = true;

  let typingTimer: any = 0;

  const autosize = () => {
    input.style.height = "auto";
    input.style.height = Math.min(input.scrollHeight, 96) + "px";
    send.disabled = input.value.trim().length === 0;
  };

  input.addEventListener("input", () => {
    autosize();
    onTyping(true);
    clearTimeout(typingTimer);
    typingTimer = setTimeout(() => onTyping(false), 1200);
  });

  input.addEventListener("keydown", (ev: KeyboardEvent) => {
    if (ev.key === "Enter" && !ev.shiftKey) {
      ev.preventDefault();
      submit();
    }
  });

  const submit = () => {
    const text = input.value.trim();
    if (!text) return;
    onTyping(false);
    onSend(text);
    input.value = "";
    autosize();
  };

  el.addEventListener("submit", (ev) => {
    ev.preventDefault();
    submit();
  });

  el.appendChild(input);
  el.appendChild(send);

  return {
    el,
    input,
    value: () => input.value,
    clear: () => {
      input.value = "";
      autosize();
    },
    focus: () => setTimeout(() => input.focus(), 30),
    setEnabled: (on: boolean) => {
      input.disabled = !on;
      send.disabled = !on || input.value.trim().length === 0;
    },
  };
}
