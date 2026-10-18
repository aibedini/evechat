import { resolveConfig } from "./config";
import { Chat } from "./store";
import { WidgetUI } from "./ui/panel";

declare global {
  interface Window {
    EveChat: any;
    ec: any;
    __evechatLoaded?: boolean;
  }
}

function globMatch(pattern: string, path: string): boolean {
  if (!pattern) return true;
  const parts = pattern.split("*");
  let pos = 0;
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (!part) continue;
    const idx = path.indexOf(part, pos);
    if (idx < 0) return false;
    if (i === 0 && idx !== 0) return false;
    pos = idx + part.length;
  }
  return true;
}

function setupProactive(chat: Chat, ui: WidgetUI): void {
  const triggers = chat.widgetConfig && chat.widgetConfig.proactive;
  if (!triggers || !triggers.length) return;
  const seen = "evechat:proactive:" + (chat.widgetConfig ? chat.widgetConfig.project_id : "x");
  try {
    if (window.sessionStorage.getItem(seen)) return;
    window.sessionStorage.setItem(seen, "1");
  } catch {
    /* sessionStorage may be blocked; triggers simply fire once per load */
  }
  for (const trig of triggers) {
    if (!trig || typeof trig.after_seconds !== "number") continue;
    if (!globMatch(trig.page_pattern || "*", location.pathname + location.search)) continue;
    setTimeout(() => {
      if (!chat.state.open) chat.setOpen(true);
    }, Math.max(1, trig.after_seconds) * 1000);
    break;
  }
}

function bootstrap(): void {
  if (window.__evechatLoaded) return;
  const cfg = resolveConfig();
  if (!cfg) {
    // eslint-disable-next-line no-console
    console.warn("[EveChat] projectKey is missing; widget not started.");
    return;
  }
  window.__evechatLoaded = true;

  const chat = new Chat(cfg);
  const api = window.EveChat || (window.EveChat = {});
  api.open = () => chat.setOpen(true);
  api.close = () => chat.setOpen(false);
  api.toggle = () => chat.toggle();
  api.identify = (visitor: any) => chat.identify(visitor);
  api.setVariable = (key: string, value: any) => chat.setVariable(key, value);
  api.send = (text: string) => chat.send(text);
  api.on = (event: string, cb: (data: any) => void) => chat.on(event as any, cb);
  api.getState = () => chat.state;
  api.destroy = () => {
    chat.destroy();
  };

  // Queued-command compatibility: ec('open') before the bundle loaded.
  const queued: any[] = (window.ec && window.ec.q) || [];
  const run = (cmd: string, args: any[]) => {
    switch (cmd) {
      case "open":
        chat.setOpen(true);
        break;
      case "close":
        chat.setOpen(false);
        break;
      case "toggle":
        chat.toggle();
        break;
      case "identify":
        chat.identify(args[0]);
        break;
      case "setVariable":
        chat.setVariable(String(args[0]), args[1]);
        break;
      case "on":
        chat.on(args[0], args[1]);
        break;
      case "send":
        chat.send(String(args[0] || ""));
        break;
      default:
        break;
    }
  };
  const ec: any = function (cmd: string, ...args: any[]) {
    run(cmd, args);
  };
  ec.q = [];
  window.ec = ec;
  queued.forEach((args) => {
    if (Array.isArray(args) && args.length) run(String(args[0]), args.slice(1));
  });

  chat
    .init()
    .then(() => {
      const wc = chat.widgetConfig;
      const palette = {
        primary: (wc && wc.colors && wc.colors.primary) || cfg.colors?.primary || undefined,
        launcher: (wc && wc.colors && (wc.colors.launcher || wc.colors.primary)) || undefined,
        text: (wc && wc.colors && wc.colors.text) || undefined,
        background: (wc && wc.colors && wc.colors.background) || undefined,
      };
      const ui = new WidgetUI(chat, {
        palette,
        position: (wc && wc.position) || cfg.position || "bottom-left",
        locale: (wc && wc.locale) || cfg.locale || "fa",
      });
      api.ui = ui;
      setupProactive(chat, ui);
    })
    .catch((err) => {
      // eslint-disable-next-line no-console
      console.warn("[EveChat] init failed", err);
    });
}

if (typeof window !== "undefined") {
  if (document.readyState === "complete" || document.readyState === "interactive") {
    bootstrap();
  } else {
    document.addEventListener("DOMContentLoaded", bootstrap);
  }
}
