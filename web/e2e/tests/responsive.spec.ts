import { test, expect, type Browser } from "@playwright/test";

// 8 devices x 2 orientations x 2 themes = 32 combinations, 8 assertions each.
const DEVICES = [
  { name: "galaxy-s8", w: 360, h: 740 },
  { name: "iphone-se", w: 375, h: 667 },
  { name: "iphone-12", w: 390, h: 844 },
  { name: "pixel-5", w: 393, h: 851 },
  { name: "ipad-mini", w: 768, h: 1024 },
  { name: "ipad-pro", w: 1024, h: 1366 },
  { name: "laptop", w: 1280, h: 800 },
  { name: "desktop", w: 1920, h: 1080 },
];
const ORIENTATIONS = ["portrait", "landscape"] as const;
const THEMES = ["light", "dark"] as const;
const widget = '[data-evechat="widget"]';

async function boot(browser: Browser, width: number, height: number, theme: "light" | "dark") {
  const context = await browser.newContext({
    viewport: { width, height },
    colorScheme: theme,
    deviceScaleFactor: 1,
  });
  const page = await context.newPage();
  await page.goto("/demo/test");
  await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
  await page.evaluate(() => (window as any).ec("open"));
  return { context, page };
}

for (const device of DEVICES) {
  for (const orientation of ORIENTATIONS) {
    for (const theme of THEMES) {
      const width = orientation === "portrait" ? device.w : device.h;
      const height = orientation === "portrait" ? device.h : device.w;
      test(`${device.name} ${orientation} ${theme}`, async ({ browser }) => {
        const { context, page } = await boot(browser, width, height, theme);
        const panel = page.locator(widget + " .ec-panel");
        const launcher = page.locator(widget + " .ec-launcher");

        // 1. the launcher is visible and inside the viewport
        await expect(launcher).toBeVisible();
        const lbox = await launcher.boundingBox();
        expect(lbox).not.toBeNull();
        expect(lbox!.x + lbox!.width).toBeLessThanOrEqual(width + 1);

        // 2. the panel opens
        await expect(panel).toHaveClass(/open/);

        // 3. the panel fits the viewport
        const box = await panel.boundingBox();
        expect(box).not.toBeNull();
        expect(box!.x).toBeGreaterThanOrEqual(-1);
        expect(box!.y).toBeGreaterThanOrEqual(-1);
        expect(box!.width).toBeLessThanOrEqual(width + 1);
        expect(box!.height).toBeLessThanOrEqual(height + 1);

        // 4. the composer is visible inside the panel
        const input = page.locator(widget + " .ec-input");
        await expect(input).toBeVisible();
        const ibox = await input.boundingBox();
        expect(ibox!.y + ibox!.height).toBeLessThanOrEqual(height + 1);

        // 5. the page itself does not scroll horizontally
        const overflow = await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth
        );
        expect(overflow).toBeLessThanOrEqual(1);

        // 6. sending a message renders it
        const text = "پیام " + device.name + " " + orientation + " " + theme;
        await page.evaluate((t) => (window as any).ec("send", t), text);
        await expect(page.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(text);

        // 7. the theme reaches the panel background
        const bg = await panel.evaluate((el) => getComputedStyle(el).backgroundColor);
        if (theme === "dark") {
          expect(bg).toBe("rgb(15, 23, 42)");
        } else {
          expect(bg).toBe("rgb(255, 255, 255)");
        }

        // 8. closing removes the panel
        await page.evaluate(() => (window as any).ec("close"));
        await expect(panel).not.toHaveClass(/open/);

        await context.close();
      });
    }
  }
}

test("dashboard: the sidebar becomes a drawer under md", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  const login = await context.request.post(baseURL + "/api/v1/auth/login", {
    data: {
      email: process.env.EVECHAT_ADMIN_EMAIL || "admin@evechat.local",
      password: process.env.EVECHAT_ADMIN_PASSWORD || "Passw0rd123!",
    },
  });
  if (login.status() !== 200) throw new Error("login failed: " + login.status());
  const auth = await login.json();
  await page.addInitScript((data) => {
    localStorage.setItem("evechat:access", data.token);
    localStorage.setItem("evechat:refresh", data.refresh);
    localStorage.setItem("evechat:session", JSON.stringify({ agent: data.agent, project: data.project }));
  }, { token: auth.access_token, refresh: auth.refresh_token, agent: auth.agent, project: auth.project });
  await page.goto("/inbox");
  const burger = page.getByRole("button", { name: "منو" });
  await expect(burger).toBeVisible();
  const shell = page.locator(".sidebar-shell");
  await expect(shell).toHaveAttribute("data-open", "false");
  await burger.click();
  await expect(shell).toHaveAttribute("data-open", "true");
  await context.close();
});