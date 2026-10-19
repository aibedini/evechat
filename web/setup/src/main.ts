import "./style.css";

interface Check {
  id: string;
  name: string;
  ok: boolean;
  fatal?: boolean;
  message: string;
  fix?: string;
}

interface Status {
  setup_required: boolean;
  version: string;
  instance_id: string;
  admin_exists: boolean;
  project_exists: boolean;
  checks: Check[];
}

interface SetupResult {
  admin_email: string;
  admin_password?: string;
  project_key: string;
  dashboard_url: string;
  embed_snippet: string;
}

class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

const app = document.getElementById("app") as HTMLElement;
const state = {
  step: 1,
  status: null as Status | null,
  checks: [] as Check[],
  admin: { name: "مدیر سیستم", email: "", password: "" },
  project: { name: "EVE", domains: [] as string[] },
  domainInput: "",
  integrations: {} as Record<string, Record<string, string>>,
  results: {} as Record<string, { ok: boolean; message: string }>,
  result: null as SetupResult | null,
  error: "",
  busy: false,
  setupToken: localStorage.getItem("evechat:setup-token") || "",
  tokenRequired: false,
};

// The wizard shows these labels instead of the raw doctor check ids/names.
const checkLabels: Record<string, string> = {
  "fs.data": "فضای ذخیره‌سازی",
  "db.open": "پایگاه داده",
  "net.port": "پورت",
  "config.keys": "کلیدهای امنیتی",
  "time.skew": "ساعت سرور",
  "net.outbound": "دسترسی اینترنت (اختیاری)",
  "assets.font": "فونت",
  "assets.dashboard": "فایل‌های پنل",
};

const stepTitles = [
  "خوش آمدید",
  "حساب مدیر",
  "اولین پروژه",
  "یکپارچه‌سازی‌ها",
  "پایان",
];

function esc(s: string): string {
  return String(s).replace(/[&<>"']/g, (c) =>
    c === "&" ? "&amp;" : c === "<" ? "&lt;" : c === ">" ? "&gt;" : c === '"' ? "&quot;" : "&#39;"
  );
}

async function api<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (state.setupToken) headers["X-Setup-Token"] = state.setupToken;
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let data: any = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }
  if (!res.ok) {
    throw new ApiError(data?.message || `خطای سرور (${res.status})`, res.status);
  }
  return data as T;
}

function header(): string {
  return `<div class="brand">
    <div class="brand-mark">⚡</div>
    <div><h1>EveChat</h1><p>راه‌اندازی اولیه ${state.status ? "— نسخه " + esc(state.status.version) : ""}</p></div>
  </div>
  <div class="progress">${stepTitles.map((_, i) => `<span class="${i + 1 <= state.step ? "on" : ""}"></span>`).join("")}</div>`;
}

function errorBox(): string {
  return state.error ? `<div class="error">${esc(state.error)}</div>` : "";
}

function checkRow(c: Check): string {
  const icon = c.ok ? '<span class="good">✓</span>' : `<span class="${c.fatal ? "bad" : "muted"}">${c.fatal ? "✗" : "!"}</span>`;
  const fix = !c.ok && c.fix ? `<div class="fix">راه‌حل: ${esc(c.fix)}</div>` : "";
  const label = checkLabels[c.id] || c.name;
  return `<div class="check"><span class="icon">${icon}</span><div><div>${esc(label)} — ${esc(c.message)}</div>${fix}</div></div>`;
}

function renderStep1(): string {
  if (state.tokenRequired) {
    return `<div class="card" data-testid="setup-token-gate">
      <h2>رمز نصب را وارد کنید</h2>
      <p class="sub">
        برای اینکه فقط صاحب سرور بتواند نصب را تمام کند، این رمز لازم است. آن را در فایل
        <code>data/setup-token.txt</code> کنار پایگاه داده، یا در لاگ سرور
        (برای داکر: <code>docker logs evechat</code>) پیدا می‌کنید.
      </p>
      <div class="field">
        <label for="setupToken">رمز نصب</label>
        <input id="setupToken" dir="ltr" autocomplete="off" spellcheck="false"
          placeholder="از فایل setup-token.txt یا لاگ سرور" value="${esc(state.setupToken)}" />
      </div>
      ${errorBox()}
      <div class="actions">
        <button class="btn btn-primary" id="checkToken">بررسی رمز</button>
      </div>
    </div>`;
  }

  const hardFail = state.checks.some((c) => !c.ok && c.fatal);
  const loaded = state.status !== null;
  return `<div class="card">
    <h2>خوش آمدید</h2>
    <p class="sub">چند دقیقه‌ی دیگر چت پشتیبانی‌تان آماده است. اول بررسی می‌کنیم سرور سالم است.</p>
    <div>${state.checks.length ? state.checks.map(checkRow).join("") : '<div class="muted">در حال بررسی…</div>'}</div>
    <div class="actions">
      <button class="btn btn-primary" id="start" ${hardFail || !loaded ? "disabled" : ""}>شروع</button>
      <button class="btn btn-ghost" id="recheck">بررسی مجدد</button>
      <span class="spacer"></span>
    </div>
    ${errorBox()}
  </div>`;
}

function strength(pw: string): number {
  let score = 0;
  if (pw.length >= 10) score++;
  if (pw.length >= 14) score++;
  if (/[A-Z]/.test(pw)) score++;
  if (/[0-9]/.test(pw)) score++;
  if (/[^A-Za-z0-9]/.test(pw)) score++;
  return Math.min(score, 5);
}

function renderStep2(): string {
  return `<div class="card">
    <h2>حساب مدیر</h2>
    <p class="sub">این حساب به همه‌چیز دسترسی دارد. رمز را جای امنی نگه دارید.</p>
    <div class="field"><label>نام</label><input id="adminName" value="${esc(state.admin.name)}" /></div>
    <div class="field"><label>ایمیل</label><input id="adminEmail" dir="ltr" type="email" value="${esc(state.admin.email)}" placeholder="admin@example.com" /></div>
    <div class="field"><label>رمز عبور (حداقل ۸ کاراکتر)</label>
      <input id="adminPassword" dir="ltr" type="password" value="${esc(state.admin.password)}" />
      <div class="strength"><i id="strengthBar"></i></div>
      <button class="btn-link" id="genPassword" style="margin-top:6px">تولید رمز قوی</button>
    </div>
    ${errorBox()}
    <div class="actions"><button class="btn btn-primary" id="next">ادامه</button></div>
  </div>`;
}

function renderStep3(): string {
  return `<div class="card">
    <h2>اولین پروژه</h2>
    <p class="sub">پروژه یعنی یک سایت یا سرویس شما. «دامنه‌های مجاز»: فقط از این سایت‌ها ویجت کار می‌کند. اگر خالی بگذارید از هر سایتی کار می‌کند؛ این فقط برای آزمایش است.</p>
    <div class="field"><label>نام پروژه</label><input id="projectName" value="${esc(state.project.name)}" /></div>
    <div class="field"><label>دامنه‌های مجاز (Enter برای افزودن)</label>
      <input id="domainInput" dir="ltr" placeholder="eve.ir" value="${esc(state.domainInput)}" />
      <div class="chips">${state.project.domains
        .map((d, i) => `<span class="chip">${esc(d)}<button data-remove="${i}">×</button></span>`)
        .join("")}</div>
      ${state.project.domains.length === 0 ? '<div class="muted" style="font-size:11px;margin-top:6px">خالی = حالت توسعه (ویجت روی همه دامنه‌ها)</div>' : ""}
    </div>
    <div class="preview">
      <header>${esc(state.project.name || "EVE")}</header>
      <div class="body">سلام 👋 چطور می‌توانیم کمک کنیم؟</div>
    </div>
    ${errorBox()}
    <div class="actions">
      <button class="btn btn-ghost" id="back">بازگشت</button>
      <button class="btn btn-primary" id="next">ادامه</button>
    </div>
  </div>`;
}

function integrationCard(key: string, title: string, fields: [string, string, string][]): string {
  const values = state.integrations[key] || {};
  const result = state.results[key];
  return `<details class="accordion" ${result ? "open" : ""}>
    <summary>${title} ${result ? (result.ok ? '<span class="good">✓</span>' : '<span class="bad">✗</span>') : ""}</summary>
    <div class="body">
      ${fields
        .map(
          ([name, label, type]) =>
            `<div class="field"><label>${label}</label><input data-int="${key}" data-field="${name}" type="${type}" dir="ltr" value="${esc(
              values[name] || ""
            )}" /></div>`
        )
        .join("")}
      <div class="actions">
        <button class="btn btn-ghost" data-test="${key}">تست اتصال</button>
        ${result ? `<span class="${result.ok ? "good" : "bad"}" style="font-size:12px">${esc(result.message)}</span>` : ""}
      </div>
    </div>
  </details>`;
}

function renderStep4(): string {
  return `<div class="card">
    <h2>یکپارچه‌سازی‌ها</h2>
    <p class="sub">همه اختیاری‌اند و بعداً هم از تنظیمات می‌شود اضافه کرد.</p>
    ${integrationCard("telegram", "تلگرام", [
      ["token", "توکن ربات (BotFather)", "password"],
      ["secret", "کلید وب‌هوک (اختیاری)", "text"],
    ])}
    ${integrationCard("eve", "EVE", [
      ["base_url", "آدرس EVE", "text"],
      ["api_key", "کلید API", "password"],
    ])}
    ${integrationCard("gmweb", "GMweb (پیامک)", [
      ["base_url", "آدرس درگاه", "text"],
      ["api_key", "کلید API", "password"],
    ])}
    ${integrationCard("ai", "دستیار هوش مصنوعی", [
      ["api_key", "کلید Anthropic", "password"],
      ["model", "مدل", "text"],
    ])}
    ${integrationCard("smtp", "ایمیل (SMTP)", [
      ["host", "Host", "text"],
      ["port", "Port", "number"],
      ["username", "Username", "text"],
      ["password", "Password", "password"],
      ["from_email", "From", "text"],
    ])}
    ${errorBox()}
    <div class="actions">
      <button class="btn btn-ghost" id="back">بازگشت</button>
      <button class="btn btn-primary" id="next">ادامه</button>
      <button class="btn-link" id="skip">رد کردن</button>
    </div>
  </div>`;
}

function renderStep5(): string {
  const r = state.result;
  if (!r) return `<div class="card"><h2>پایان</h2><p class="sub">در حال نهایی‌سازی…</p></div>`;
  return `<div class="card">
    <h2>آماده است!</h2>
    <p class="sub">کد زیر را قبل از <code>&lt;/body&gt;</code> سایت‌تان بچسبانید. اطلاعات ورود را هم جای امنی ذخیره کنید.</p>
    <div class="success-box">
      <div class="cred"><span>پنل</span><code>${esc(r.dashboard_url)}</code></div>
      <div class="cred"><span>ایمیل</span><code>${esc(r.admin_email)}</code></div>
      ${r.admin_password ? `<div class="cred"><span>رمز عبور</span><code>${esc(r.admin_password)}</code></div>` : ""}
      <div class="cred"><span>کلید پروژه</span><code>${esc(r.project_key)}</code></div>
    </div>
    <h2 style="font-size:15px">کد جاسازی ویجت</h2>
    <pre id="snippet">${esc(r.embed_snippet)}</pre>
    <div class="actions">
      <button class="btn btn-ghost" id="copySnippet">کپی کد</button>
      <button class="btn btn-ghost" id="downloadSummary">دانلود خلاصه</button>
      <span class="spacer"></span>
      <button class="btn btn-primary" id="enter">ورود به پنل</button>
    </div>
    <p class="muted" style="font-size:11px;margin-top:10px">برای امنیت، فایل <code>setup-token.txt</code> را پس از نصب حذف کنید.</p>
  </div>`;
}

function render(): void {
  const body =
    state.step === 1
      ? renderStep1()
      : state.step === 2
      ? renderStep2()
      : state.step === 3
      ? renderStep3()
      : state.step === 4
      ? renderStep4()
      : renderStep5();
  app.innerHTML = `<div class="shell">${header()}${body}</div>`;
  bind();
}

function bind(): void {
  const on = (id: string, event: string, handler: (ev: Event) => void) => {
    const node = document.getElementById(id);
    if (node) node.addEventListener(event, handler);
  };

  if (state.step === 1) {
    const tokenInput = document.getElementById("setupToken") as HTMLInputElement | null;
    tokenInput?.addEventListener("input", () => {
      state.setupToken = tokenInput.value.trim();
      localStorage.setItem("evechat:setup-token", state.setupToken);
      state.error = "";
    });
    if (state.tokenRequired) {
      const submitToken = async () => {
        state.setupToken = (tokenInput?.value || "").trim();
        localStorage.setItem("evechat:setup-token", state.setupToken);
        if (!state.setupToken) {
          state.error = "رمز نصب را وارد کنید.";
          render();
          return;
        }
        await loadStatus();
      };
      on("checkToken", "click", submitToken);
      tokenInput?.addEventListener("keydown", (ev) => {
        if ((ev as KeyboardEvent).key === "Enter") {
          ev.preventDefault();
          void submitToken();
        }
      });
      return;
    }
    on("start", "click", () => {
      state.step = 2;
      state.error = "";
      render();
    });
    on("recheck", "click", async () => {
      state.error = "";
      await loadStatus(true);
    });
    return;
  }

  if (state.step === 2) {
    const pw = document.getElementById("adminPassword") as HTMLInputElement;
    const bar = document.getElementById("strengthBar") as HTMLElement;
    const paint = () => {
      const s = strength(pw.value);
      bar.style.width = (s / 5) * 100 + "%";
      bar.style.background = s >= 4 ? "#10b981" : s >= 3 ? "#f59e0b" : "#ef4444";
    };
    pw.addEventListener("input", paint);
    paint();
    on("genPassword", "click", () => {
      const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$";
      const bytes = new Uint8Array(16);
      crypto.getRandomValues(bytes);
      pw.value = Array.from(bytes, (b) => alphabet[b % alphabet.length]).join("");
      paint();
      navigator.clipboard?.writeText(pw.value).catch(() => undefined);
    });
    on("next", "click", async () => {
      state.admin.name = (document.getElementById("adminName") as HTMLInputElement).value.trim();
      state.admin.email = (document.getElementById("adminEmail") as HTMLInputElement).value.trim();
      state.admin.password = pw.value;
      state.error = "";
      if (!state.admin.email.includes("@")) {
        state.error = "ایمیل معتبر نیست";
        render();
        return;
      }
      if (state.admin.password.length < 8) {
        state.error = "رمز عبور باید حداقل ۸ کاراکتر باشد";
        render();
        return;
      }
      state.busy = true;
      try {
        await api("/api/setup/admin", "POST", state.admin);
        state.step = 3;
      } catch (err) {
        state.error = (err as Error).message;
      } finally {
        state.busy = false;
        render();
      }
    });
    return;
  }

  if (state.step === 3) {
    const domainInput = document.getElementById("domainInput") as HTMLInputElement;
    domainInput?.addEventListener("keydown", (ev) => {
      if (ev.key !== "Enter") return;
      ev.preventDefault();
      const value = domainInput.value.trim().toLowerCase();
      if (value && !state.project.domains.includes(value)) state.project.domains.push(value);
      domainInput.value = "";
      state.project.name = (document.getElementById("projectName") as HTMLInputElement).value;
      render();
    });
    app.querySelectorAll("[data-remove]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const idx = Number((btn as HTMLElement).dataset.remove);
        state.project.domains.splice(idx, 1);
        state.project.name = (document.getElementById("projectName") as HTMLInputElement).value;
        render();
      });
    });
    on("back", "click", () => {
      state.step = 2;
      render();
    });
    on("next", "click", async () => {
      state.project.name = (document.getElementById("projectName") as HTMLInputElement).value.trim();
      state.error = "";
      state.busy = true;
      try {
        await api("/api/setup/project", "POST", { name: state.project.name, domains: state.project.domains });
        state.step = 4;
      } catch (err) {
        state.error = (err as Error).message;
      } finally {
        state.busy = false;
        render();
      }
    });
    return;
  }

  if (state.step === 4) {
    app.querySelectorAll("[data-int]").forEach((input) => {
      input.addEventListener("input", () => {
        const el = input as HTMLInputElement;
        const kind = el.dataset.int as string;
        const field = el.dataset.field as string;
        state.integrations[kind] = state.integrations[kind] || {};
        state.integrations[kind][field] = el.value;
      });
    });
    app.querySelectorAll("[data-test]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const kind = (btn as HTMLElement).dataset.test as string;
        const payload = state.integrations[kind] || {};
        try {
          const res = await api<{ ok: boolean; message: string }>(`/api/setup/test/${kind}`, "POST", payload);
          state.results[kind] = res;
        } catch (err) {
          state.results[kind] = { ok: false, message: (err as Error).message };
        }
        render();
      });
    });
    on("back", "click", () => {
      state.step = 3;
      render();
    });
    on("skip", "click", () => completeSetup());
    on("next", "click", async () => {
      state.error = "";
      state.busy = true;
      try {
        const payload: Record<string, unknown> = {};
        for (const [kind, values] of Object.entries(state.integrations)) {
          if (Object.values(values).some((v) => v.trim() !== "")) payload[kind] = values;
        }
        if (Object.keys(payload).length > 0) {
          const res = await api<{ results: Record<string, { ok: boolean; message: string }> }>(
            "/api/setup/integrations",
            "POST",
            payload
          );
          state.results = { ...state.results, ...res.results };
          const failed = Object.values(res.results).filter((r) => !r.ok);
          if (failed.length > 0) {
            state.error = "ذخیره شد ولی تست برخی سرویس‌ها ناموفق بود — بعداً در تنظیمات اصلاح کنید.";
          }
        }
        await completeSetup();
      } catch (err) {
        state.error = (err as Error).message;
      } finally {
        state.busy = false;
        render();
      }
    });
    return;
  }

  on("copySnippet", "click", () => {
    navigator.clipboard?.writeText(state.result?.embed_snippet || "").catch(() => undefined);
    const btn = document.getElementById("copySnippet");
    if (btn) btn.textContent = "کپی شد ✓";
  });
  on("downloadSummary", "click", () => {
    const r = state.result;
    if (!r) return;
    const text = `EveChat setup\n\nDashboard: ${r.dashboard_url}\nEmail: ${r.admin_email}\nPassword: ${
      r.admin_password || "(not shown)"
    }\nProject key: ${r.project_key}\n\n${r.embed_snippet}\n`;
    const blob = new Blob([text], { type: "text/plain;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "evechat-setup.txt";
    a.click();
    URL.revokeObjectURL(a.href);
  });
  on("enter", "click", () => {
    const access = localStorage.getItem("evechat:access");
    location.assign(access ? "/inbox" : "/login");
  });
}

async function completeSetup(): Promise<void> {
  try {
    const res = await api<{
      ok: boolean;
      access_token: string;
      refresh_token: string;
      redirect: string;
    }>("/api/setup/complete", "POST", {});
    localStorage.setItem("evechat:access", res.access_token);
    localStorage.setItem("evechat:refresh", res.refresh_token);
    const status = await api<{ admin_email?: string }>("/api/v1/auth/me").catch(() => null);
    state.result = {
      admin_email: status?.admin_email || state.admin.email,
      admin_password: state.admin.password,
      project_key: (await lastProjectKey()) || "",
      dashboard_url: location.origin,
      embed_snippet: (await lastSnippet()) || "",
    };
    state.step = 5;
  } catch (err) {
    state.error = (err as Error).message;
  }
  render();
}

async function lastProjectKey(): Promise<string> {
  try {
    const data = await api<{ project: { api_key: string } }>("/api/v1/settings/project");
    return data.project.api_key;
  } catch {
    return "";
  }
}

async function lastSnippet(): Promise<string> {
  try {
    const data = await api<{ embed_code: string }>("/api/v1/settings/project");
    return data.embed_code;
  } catch {
    return "";
  }
}

async function loadStatus(rerunChecks = false): Promise<void> {
  try {
    if (rerunChecks) {
      const report = await api<{ checks: Array<{ id: string; name: string; result: Check; level: number }> }>(
        "/api/setup/check",
        "POST",
        {}
      );
      state.checks = report.checks.map((c) => ({
        id: c.id,
        name: c.name,
        ok: c.result.ok,
        message: c.result.message,
        fix: c.result.fix,
        fatal: c.level === 0,
      }));
      render();
      return;
    }
    const status = await api<Status>("/api/setup/status");
    state.status = status;
    state.tokenRequired = false;
    state.error = "";
    state.checks = status.checks || [];
    if (!status.setup_required) {
      location.assign("/inbox");
      return;
    }
    state.step = status.admin_exists ? (status.project_exists ? 4 : 3) : 1;
    render();
    if (state.step === 1 && state.checks.length === 0) {
      await loadStatus(true);
    }
  } catch (err) {
    const code = err instanceof ApiError ? err.status : 0;
    if (code === 401) {
      state.tokenRequired = true;
      state.step = 1;
      state.error = state.setupToken
        ? "رمز درست نیست. دوباره از فایل کپی کنید؛ فاصله‌ی اضافه‌ی اول یا آخر را نگیرید."
        : "";
    } else {
      state.error = (err as Error).message;
    }
    render();
  }
}

loadStatus();
