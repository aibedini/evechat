import type { Chat, ChatState } from "../store";
import { t } from "../i18n";
import { createLauncher, type Launcher } from "./launcher";
import { createComposer, type Composer } from "./composer";
import { renderCSAT, renderMessages, renderTyping } from "./messages";
import { styles, injectFont, type Palette } from "./styles";

export interface UIOptions {
  palette: Partial<Palette>;
  position: string;
  locale: string;
  apiUrl?: string;
  font?: "vazirmatn" | "inherit" | "none";
  dark?: boolean;
}

/** Builds the launcher and the panel inside a closed shadow root. */
export class WidgetUI {
  private host: HTMLElement;
  private root: ShadowRoot;
  private launcher: Launcher;
  private panel: HTMLElement;
  private messagesEl: HTMLElement;
  private bannerEl: HTMLElement;
  private typingEl: HTMLElement;
  private csatEl: HTMLElement;
  private statusEl: HTMLElement;
  private titleEl: HTMLElement;
  private composer: Composer;
  private unsubscribe: () => void = () => undefined;
  private viewportCleanup: () => void = () => undefined;
  private locked = false;
  private prevOverflow = "";

  constructor(private chat: Chat, private opts: UIOptions) {
    this.host = document.createElement("div");
    this.host.setAttribute("data-evechat", "widget");
    this.host.setAttribute("lang", "fa");
    this.host.setAttribute("dir", "rtl");
    this.root = this.host.attachShadow({ mode: "open" });

    const style = document.createElement("style");
    style.textContent = styles(opts.palette, opts.position, !!opts.dark);
    this.root.appendChild(style);

    const wrap = document.createElement("div");
    wrap.className = "ec-root";
    this.root.appendChild(wrap);

    this.launcher = createLauncher(() => this.chat.toggle());
    wrap.appendChild(this.launcher.el);

    this.panel = document.createElement("section");
    this.panel.className = "ec-panel";
    this.panel.setAttribute("role", "dialog");
    this.panel.setAttribute("aria-label", t(opts.locale).title);
    wrap.appendChild(this.panel);

    this.buildHeader();
    this.bannerEl = document.createElement("div");
    this.bannerEl.className = "ec-banner";
    this.panel.appendChild(this.bannerEl);

    this.messagesEl = document.createElement("div");
    this.messagesEl.className = "ec-msgs";
    this.messagesEl.setAttribute("role", "log");
    this.messagesEl.setAttribute("aria-live", "polite");
    this.messagesEl.setAttribute("aria-relevant", "additions text");
    this.panel.appendChild(this.messagesEl);

    this.typingEl = document.createElement("div");
    this.typingEl.className = "ec-typing";
    this.panel.appendChild(this.typingEl);

    this.csatEl = document.createElement("div");
    this.csatEl.className = "ec-csat";
    this.panel.appendChild(this.csatEl);

    this.composer = createComposer(
      opts.locale,
      (text) => {
        this.chat.send(text);
      },
      (typing) => this.chat.sendTyping(typing)
    );
    this.panel.appendChild(this.composer.el);
    this.composer.input.value = this.chat.draft();
    this.composer.input.addEventListener("input", () => this.chat.setDraft(this.composer.input.value));

    this.root.addEventListener("keydown", (ev: Event) => {
      const ke = ev as KeyboardEvent;
      if (ke.key === "Escape") this.chat.setOpen(false);
    });
    document.addEventListener("keydown", (ev: KeyboardEvent) => {
      if (ev.key === "Escape" && this.chat.state.open) this.chat.setOpen(false);
    });

    this.mount();
    this.setupViewport();
    this.unsubscribe = this.chat.subscribe((s) => this.update(s));
  }

  /**
   * Mobile keyboards do not resize the layout viewport: window.innerHeight
   * stays put and the composer ends up behind the keyboard. visualViewport
   * reports the actually visible box, so track it and expose the keyboard
   * height (and the visible height) to the CSS as --ec-kb / --ec-vv.
   */
  private setupViewport(): void {
    const vv = (window as any).visualViewport as VisualViewport | undefined;
    const apply = () => {
      const height = vv ? vv.height : window.innerHeight;
      const offsetTop = vv ? vv.offsetTop : 0;
      const kb = Math.max(0, Math.round(window.innerHeight - height - offsetTop));
      this.host.style.setProperty("--ec-vh", window.innerHeight + "px");
      this.host.style.setProperty("--ec-vv", Math.round(height) + "px");
      this.host.style.setProperty("--ec-kb", kb + "px");
      this.panel.classList.toggle("kb", kb > 80);
    };
    apply();
    if (vv) {
      vv.addEventListener("resize", apply);
      vv.addEventListener("scroll", apply);
    }
    window.addEventListener("resize", apply);
    window.addEventListener("orientationchange", apply);
    this.viewportCleanup = () => {
      if (vv) {
        vv.removeEventListener("resize", apply);
        vv.removeEventListener("scroll", apply);
      }
      window.removeEventListener("resize", apply);
      window.removeEventListener("orientationchange", apply);
    };
  }

  /** Full-screen mobile panel locks the page scroll behind it. */
  private updateScrollLock(open: boolean): void {
    const mobile =
      typeof window.matchMedia === "function" && window.matchMedia("(max-width:480px)").matches;
    if (open && mobile && !this.locked) {
      this.prevOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      this.locked = true;
    } else if ((!open || !mobile) && this.locked) {
      document.body.style.overflow = this.prevOverflow;
      this.locked = false;
    }
  }

  private buildHeader(): void {
    const head = document.createElement("header");
    head.className = "ec-head";

    const avatar = document.createElement("div");
    avatar.className = "ec-avatar";
    avatar.textContent = "EC";
    head.appendChild(avatar);

    const text = document.createElement("div");
    text.className = "ec-head-text";
    this.titleEl = document.createElement("div");
    this.titleEl.className = "ec-title";
    this.titleEl.textContent = t(this.opts.locale).title;
    this.statusEl = document.createElement("div");
    this.statusEl.className = "ec-status";
    text.appendChild(this.titleEl);
    text.appendChild(this.statusEl);
    head.appendChild(text);

    const close = document.createElement("button");
    close.className = "ec-close";
    close.type = "button";
    close.setAttribute("aria-label", t(this.opts.locale).close);
    close.innerHTML = "&times;";
    close.addEventListener("click", () => this.chat.setOpen(false));
    head.appendChild(close);

    this.panel.appendChild(head);
  }

  private mount(): void {
    const attach = () => document.body.appendChild(this.host);
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", attach);
    } else {
      attach();
    }
  }

  private update(state: ChatState): void {
    const strings = t(this.opts.locale);
    if (state.open && !this.panel.dataset.font) {
      this.panel.dataset.font = "1";
      if (this.opts.font !== "none" && this.opts.font !== "inherit" && this.opts.apiUrl) {
        injectFont(this.opts.apiUrl);
      }
    }
    this.panel.classList.toggle("open", state.open);
    this.updateScrollLock(state.open);
    this.launcher.el.setAttribute("aria-expanded", state.open ? "true" : "false");
    this.launcher.setUnread(state.unread);
    this.launcher.el.setAttribute(
      "aria-label",
      state.unread > 0 ? `باز کردن گفتگوی پشتیبانی (${state.unread} پیام خوانده‌نشده)` : "باز کردن گفتگوی پشتیبانی"
    );

    const online = this.chat.widgetConfig ? this.chat.widgetConfig.online : true;
    const sessionError = !state.ready && state.error === "session";
    this.statusEl.textContent = "";
    if (!state.ready) {
      this.statusEl.appendChild(
        document.createTextNode(sessionError ? strings.sessionFailed : strings.preparing)
      );
    } else {
      this.statusEl.appendChild(
        document.createTextNode(
          state.agentName ? state.agentName : online ? strings.title : state.connection === "open" ? strings.title : strings.connecting
        )
      );
    }

    const degraded = state.connection === "degraded" || state.connection === "reconnecting";
    const offline = state.connection === "offline";
    const bannerText = sessionError
      ? strings.sessionFailed + " — " + strings.retry
      : degraded
        ? strings.degraded
        : offline
          ? strings.offline
          : "";
    this.bannerEl.textContent = bannerText;
    this.bannerEl.classList.toggle("on", !!bannerText);
    this.bannerEl.onclick = sessionError ? () => this.chat.retrySession() : null;
    this.bannerEl.style.cursor = sessionError ? "pointer" : "";

    renderMessages(this.messagesEl, state.messages, this.opts.locale, (id) => this.chat.retry(id));
    renderTyping(this.typingEl, state.typing, this.opts.locale);

    if (state.csatPending) {
      if (!this.csatEl.classList.contains("on")) {
        this.csatEl.classList.add("on");
        renderCSAT(this.csatEl, this.opts.locale, (score, comment) => {
          this.chat.submitCSAT(score, comment);
          this.csatEl.classList.remove("on");
          this.csatEl.innerHTML = "";
          const thanks = document.createElement("div");
          thanks.className = "ec-csat-title";
          thanks.textContent = strings.csatThanks;
          this.csatEl.classList.add("on");
          this.csatEl.appendChild(thanks);
          setTimeout(() => this.csatEl.classList.remove("on"), 4000);
        });
      }
    } else if (this.csatEl.classList.contains("on") && !this.csatEl.dataset.thanks) {
      this.csatEl.classList.remove("on");
    }

    if (state.open && !this.panel.dataset.focused) {
      this.panel.dataset.focused = "1";
      this.composer.focus();
    }
    if (!state.open) delete this.panel.dataset.focused;
  }

  destroy(): void {
    this.unsubscribe();
    this.viewportCleanup();
    if (this.locked) document.body.style.overflow = this.prevOverflow;
    this.host.remove();
  }
}
