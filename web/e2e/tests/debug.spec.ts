import { test } from "@playwright/test";
test("debug widget boot", async ({ page }) => {
  page.on("console", (m) => console.log("CONSOLE", m.type(), m.text()));
  page.on("pageerror", (e) => console.log("PAGEERROR", e.message));
  await page.goto("/demo/test");
  await page.waitForTimeout(4000);
  const state = await page.evaluate(() => ({
    hasEc: typeof (window as any).ec,
    hasUi: Boolean((window as any).EveChat?.ui),
    keys: Object.keys((window as any).EveChat || {}),
    loaded: (window as any).__evechatLoaded,
    host: Boolean(document.querySelector('[data-evechat="widget"]')),
  }));
  console.log("STATE", JSON.stringify(state));
});
