import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import fs from "node:fs";

// The wizard needs an unconfigured instance, so this spec is opt-in: the
// regular behaviour job runs against a seeded instance and skips it. The
// dedicated CI job (and `make fresh`) sets EVECHAT_WIZARD=1.
test.skip(!process.env.EVECHAT_WIZARD, "needs a fresh, unconfigured instance (see make fresh)");
test.describe.configure({ retries: 0, mode: "serial" });

const baseURL = process.env.EVECHAT_BASE || "http://127.0.0.1:3411";
const tokenFile = process.env.EVECHAT_SETUP_TOKEN_FILE;
const setupToken =
  process.env.EVECHAT_SETUP_TOKEN ||
  (tokenFile && fs.existsSync(tokenFile) ? fs.readFileSync(tokenFile, "utf8").trim() : "");

fs.mkdirSync("../../docs/screenshots/wizard", { recursive: true });

const WRONG_TOKEN_ERROR =
  "رمز درست نیست. دوباره از فایل کپی کنید؛ فاصله‌ی اضافه‌ی اول یا آخر را نگیرید.";

async function expectNoHorizontalScroll(page: Page, label: string) {
  const overflow = await page.evaluate(() => {
    const doc = document.documentElement;
    return Math.max(doc.scrollWidth, document.body ? document.body.scrollWidth : 0) - window.innerWidth;
  });
  expect(overflow, `${label}: horizontal overflow`).toBeLessThanOrEqual(1);
}

async function expectTouchTargets(page: Page, label: string) {
  const small = await page.evaluate(() => {
    const nodes = Array.from(document.querySelectorAll("button, input, select, textarea, summary"));
    return nodes
      .filter((node) => {
        const el = node as HTMLElement;
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && rect.height < 44;
      })
      .map((node) => {
        const el = node as HTMLElement;
        const rect = el.getBoundingClientRect();
        return {
          tag: el.tagName,
          id: el.id,
          text: (el.textContent || "").trim().slice(0, 40),
          height: Math.round(rect.height),
        };
      });
  });
  expect(small, `${label}: touch targets under 44px`).toEqual([]);
}

async function newRemoteContext(browser: import("@playwright/test").Browser): Promise<BrowserContext> {
  // The peer is loopback, which is in server.trusted_proxies, so the server
  // believes this header and treats the request as coming from 203.0.113.9.
  return browser.newContext({
    viewport: { width: 375, height: 812 },
    extraHTTPHeaders: { "X-Forwarded-For": "203.0.113.9" },
  });
}

test("setup wizard: token gate, mobile/desktop layout and post-completion 404", async ({ browser }) => {
  test.skip(!setupToken, "EVECHAT_SETUP_TOKEN or EVECHAT_SETUP_TOKEN_FILE is required");

  // (a) Loopback: no token page, the wizard opens on step 1.
  const localContext = await browser.newContext({ viewport: { width: 375, height: 812 } });
  try {
    const local = await localContext.newPage();
    await local.goto("/setup");
    await expect(local.getByRole("heading", { name: "خوش آمدید" })).toBeVisible();
    await expect(local.getByTestId("setup-token-gate")).toHaveCount(0);
    await expect(local.getByRole("button", { name: "شروع" })).toBeEnabled();
    await expectNoHorizontalScroll(local, "loopback 375px");
    await expectTouchTargets(local, "loopback 375px");
  } finally {
    await localContext.close();
  }

  // (b) Non-loopback: the token page comes first.
  const remoteContext = await newRemoteContext(browser);
  try {
    const page = await remoteContext.newPage();
    await page.goto("/setup");
    await expect(page.getByTestId("setup-token-gate")).toBeVisible();
    await expect(page.getByRole("heading", { name: "رمز نصب را وارد کنید" })).toBeVisible();
    await expect(page.getByText("data/setup-token.txt")).toBeVisible();
    await expectNoHorizontalScroll(page, "token gate 375px");
    await expectTouchTargets(page, "token gate 375px");
    await page.screenshot({ path: "../../docs/screenshots/wizard/00-token-375.png", fullPage: true });

    // Wrong token -> the specific Persian error, still on the gate.
    await page.locator("#setupToken").fill("definitely-not-the-token");
    await page.getByRole("button", { name: "بررسی رمز" }).click();
    await expect(page.getByText(WRONG_TOKEN_ERROR)).toBeVisible();
    await expect(page.getByTestId("setup-token-gate")).toBeVisible();

    // Correct token -> step 1.
    await page.locator("#setupToken").fill(setupToken);
    await page.getByRole("button", { name: "بررسی رمز" }).click();
    await expect(page.getByRole("heading", { name: "خوش آمدید" })).toBeVisible();
    const start = page.getByRole("button", { name: "شروع" });
    await expect(start).toBeEnabled();
    await expectTouchTargets(page, "welcome 375px");

    // 1440px: the same screens must fit without horizontal scrolling.
    await page.setViewportSize({ width: 1440, height: 900 });
    await expectNoHorizontalScroll(page, "welcome 1440px");
    await expect(start).toBeVisible();
    await page.screenshot({ path: "../../docs/screenshots/wizard/01-welcome-1440.png", fullPage: true });

    await page.setViewportSize({ width: 375, height: 812 });
    await expectNoHorizontalScroll(page, "welcome 375px again");

    // Step 2 — admin account.
    await start.click();
    await expect(page.getByRole("heading", { name: "حساب مدیر" })).toBeVisible();
    await page.locator("#adminName").fill("مدیر ویزارد");
    await page.locator("#adminEmail").fill("wizard@evechat.local");
    await page.locator("#adminPassword").fill("WizardPass123!");
    await expectTouchTargets(page, "admin 375px");
    await expectNoHorizontalScroll(page, "admin 375px");
    await page.screenshot({ path: "../../docs/screenshots/wizard/02-admin-375.png", fullPage: true });
    await page.getByRole("button", { name: "ادامه" }).click();

    // Step 3 — first project.
    await expect(page.getByRole("heading", { name: "اولین پروژه" })).toBeVisible();
    await page.locator("#projectName").fill("Wizard CI");
    await expectNoHorizontalScroll(page, "project 375px");
    await page.screenshot({ path: "../../docs/screenshots/wizard/03-project-375.png", fullPage: true });
    await page.getByRole("button", { name: "ادامه" }).click();

    // Step 4 — optional integrations.
    await expect(page.getByRole("heading", { name: "یکپارچه‌سازی‌ها" })).toBeVisible();
    await expectNoHorizontalScroll(page, "integrations 375px");
    await page.screenshot({ path: "../../docs/screenshots/wizard/04-integrations-375.png", fullPage: true });
    await page.getByRole("button", { name: "رد کردن" }).click();

    // Step 5 — done.
    await expect(page.getByRole("heading", { name: "آماده است!" })).toBeVisible();
    await expectNoHorizontalScroll(page, "done 375px");
    await page.screenshot({ path: "../../docs/screenshots/wizard/05-done-375.png", fullPage: true });

    // (d) After completion the wizard is gone.
    const setupResponse = await remoteContext.request.get(`${baseURL}/setup`);
    expect(setupResponse.status()).toBe(404);
    const apiResponse = await remoteContext.request.get(`${baseURL}/api/setup/status`);
    expect(apiResponse.status()).toBe(410);
  } finally {
    await remoteContext.close();
  }
});
