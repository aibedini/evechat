// Headless dashboard smoke check (system Chrome/Edge, no bundled browser).
// Usage: node check.mjs <email> <password>
import { chromium } from "playwright-core";

const email = process.argv[2] || "admin@evechat.local";
const password = process.argv[3] || "";
const base = process.argv[4] || "http://127.0.0.1:3000";

const browser = await chromium.launch({ channel: "chrome", headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const errors = [];
page.on("console", (msg) => {
  if (msg.type() === "error") errors.push(msg.text());
});
page.on("pageerror", (err) => errors.push("pageerror: " + err.message));

try {
  await page.goto(base + "/login", { waitUntil: "networkidle", timeout: 30000 });
  await page.fill("#email", email);
  await page.fill("#password", password);
  await page.click('button[type="submit"]');
  await page.waitForURL("**/inbox", { timeout: 20000 });
  await page.waitForTimeout(2500);
  await page.screenshot({ path: "inbox.png", fullPage: false });
  console.log("inbox heading:", await page.locator("h1").first().textContent().catch(() => "-"));

  // open the first conversation if any
  const first = page.locator('button:has-text("در انتظار")').first();
  if (await first.count()) {
    await first.click().catch(() => undefined);
    await page.waitForTimeout(1200);
  }
  await page.screenshot({ path: "inbox-detail.png" });

  for (const path of ["/", "/visitors", "/tickets", "/knowledge-base", "/settings/widget", "/settings/routing", "/settings/team", "/settings/integrations", "/settings/webhooks"]) {
    await page.goto(base + path, { waitUntil: "networkidle", timeout: 30000 });
    await page.waitForTimeout(700);
    const body = (await page.locator("body").innerText()).slice(0, 200).replace(/\n+/g, " | ");
    console.log(path, "=>", body.slice(0, 110));
    if (path === "/settings/widget") await page.screenshot({ path: "settings-widget.png" });
  }

  console.log("\nconsole errors:", errors.length ? errors.slice(0, 10) : "none");
} finally {
  await browser.close();
}
