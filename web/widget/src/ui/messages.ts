import { t } from "../i18n";
import type { Message } from "../types";

function timeOf(msg: Message): string {
  const raw = msg.created_at;
  if (!raw) return "";
  const d = typeof raw === "number" ? new Date(raw) : new Date(String(raw));
  if (isNaN(d.getTime())) return "";
  return d.toLocaleTimeString("fa-IR", { hour: "2-digit", minute: "2-digit" });
}

function statusIcon(msg: Message): string {
  switch (msg.status) {
    case "pending":
      return "⏳";
    case "sent":
      return "✓";
    case "read":
      return "✓✓";
    case "failed":
      return "⚠";
    default:
      return "";
  }
}

/**
 * Renders the thread. Above 200 messages only the newest 200 are painted: the
 * DOM stays small and scrolling stays smooth on low-end phones.
 */
export function renderMessages(container: HTMLElement, messages: Message[], locale: string, onRetry: (id: string) => void): void {
  const strings = t(locale);
  const nearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 60;
  const windowed = messages.length > 200 ? messages.slice(messages.length - 200) : messages;
  container.innerHTML = "";
  if (!windowed.length) {
    const empty = document.createElement("div");
    empty.className = "ec-msg system";
    empty.textContent = strings.greeting;
    container.appendChild(empty);
    return;
  }
  for (const msg of windowed) {
    if (msg.sender_type === "system" && !msg.content) continue;
    const el = document.createElement("div");
    el.className = "ec-msg " + (msg.sender_type || "bot");
    if (msg.status === "failed") el.classList.add("failed");
    const body = document.createElement("div");
    body.textContent = msg.content;
    el.appendChild(body);
    if (msg.sender_type === "visitor") {
      const meta = document.createElement("div");
      meta.className = "ec-meta";
      const tm = document.createElement("span");
      tm.textContent = timeOf(msg);
      meta.appendChild(tm);
      const st = document.createElement("span");
      st.textContent = statusIcon(msg);
      st.title =
        msg.status === "failed" ? strings.failed : msg.status === "read" ? strings.read : msg.status === "pending" ? strings.pending : strings.sent;
      meta.appendChild(st);
      el.appendChild(meta);
      if (msg.status === "failed") {
        el.addEventListener("click", () => onRetry(msg.client_id || msg.id));
      }
    }
    container.appendChild(el);
  }
  if (nearBottom) container.scrollTop = container.scrollHeight;
}

export function renderTyping(container: HTMLElement, typing: boolean, locale: string): void {
  if (!typing) {
    container.classList.remove("on");
    container.textContent = "";
    return;
  }
  const dots = document.createElement("span");
  dots.className = "ec-dots";
  dots.innerHTML = "<span></span><span></span><span></span>";
  container.textContent = t(locale).typing + " ";
  container.appendChild(dots);
  container.classList.add("on");
}

export function renderCSAT(container: HTMLElement, locale: string, onSubmit: (score: number, comment: string) => void): void {
  container.innerHTML = "";
  const strings = t(locale);
  const title = document.createElement("div");
  title.className = "ec-csat-title";
  title.textContent = strings.csatTitle;
  container.appendChild(title);

  const stars = document.createElement("div");
  stars.className = "ec-stars";
  const buttons: HTMLButtonElement[] = [];
  for (let i = 1; i <= 5; i++) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "ec-star";
    b.textContent = "★";
    b.addEventListener("click", () => {
      buttons.forEach((x, idx) => x.classList.toggle("on", idx < i));
      comment.dataset.score = String(i);
    });
    buttons.push(b);
    stars.appendChild(b);
  }
  container.appendChild(stars);

  const comment = document.createElement("textarea");
  comment.placeholder = strings.csatComment;
  container.appendChild(comment);

  const submit = document.createElement("button");
  submit.type = "button";
  submit.className = "ec-submit";
  submit.textContent = strings.csatSubmit;
  submit.addEventListener("click", () => {
    const score = parseInt(comment.dataset.score || "0", 10);
    if (score >= 1) onSubmit(score, comment.value.trim());
  });
  container.appendChild(submit);
}

