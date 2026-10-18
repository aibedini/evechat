// Regenerates docs/screenshots/*.png with the system Chrome (playwright-core).
//   node scripts/screenshots.mjs http://127.0.0.1:3000 admin@evechat.local <password>
import { mkdirSync } from "node:fs";

// playwright-core is a dev-only dependency; look in the repo tooling folder as
// a fallback so this script works without a root package.json.
let chromium;
for (const candidate of ["playwright-core", "../tools/ui-check/node_modules/playwright-core/index.js"]) {
  try {
    const mod = await import(candidate);
    chromium = mod.chromium || (mod.default && mod.default.chromium);
    if (chromium) break;
  } catch {
    /* try the next candidate */
  }
}
if (!chromium) {
  console.error("playwright-core not found. Install it with: npm --prefix tools/ui-check i playwright-core");
  process.exit(1);
}
const base = process.argv[2] || "http://127.0.0.1:3000";
const email = process.argv[3] || "admin@evechat.local";
const password = process.argv[4] || process.env.EVECHAT_DEMO_PASSWORD || "";
const outDir = "docs/screenshots";
mkdirSync(outDir, { recursive: true });

const browser = await chromium.launch({ channel: "chrome", headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });
const page = await context.newPage();

async function shot(name, path, opts = {}) {
  await page.goto(base + path, { waitUntil: "networkidle", timeout: 45000 });
  await page.waitForTimeout(900);
  if (opts.dark) await page.evaluate(() => (document.documentElement.dataset.theme = "dark"));
  if (opts.light) await page.evaluate(() => (document.documentElement.dataset.theme = "light"));
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${outDir}/${name}.png`, fullPage: false });
  console.log("✓", `${outDir}/${name}.png`);
}

await page.goto(base + "/login", { waitUntil: "networkidle" });
await page.screenshot({ path: `${outDir}/login.png` });
console.log("✓", `${outDir}/login.png`);
await page.fill("#email", email);
await page.fill("#password", password);
await page.click('button[type="submit"]');
await page.waitForURL("**/inbox", { timeout: 30000 });
await page.waitForTimeout(1800);

const first = page.getByText(/./).first();
await page.screenshot({ path: `${outDir}/inbox.png` });
console.log("✓", `${outDir}/inbox.png`);

for (const [name, path, opts] of [
  ["dashboard", "/", { light: true }],
  ["dashboard-dark", "/", { dark: true }],
  ["tickets", "/tickets", { light: true }],
  ["visitors", "/visitors", { light: true }],
  ["knowledge-base", "/knowledge-base", { light: true }],
  ["settings-widget", "/settings/widget", { light: true }],
  ["settings-routing", "/settings/routing", { light: true }],
  ["settings-team", "/settings/team", { light: true }],
  ["settings-security", "/settings/security", { light: true }],
  ["settings-health", "/settings/diagnostics", { light: true }],
  ["settings-activity", "/settings/activity", { light: true }],
]) {
  await shot(name, path, opts);
}

// Widget: embed it on a blank page and open the panel.
const projectKey = process.env.EVECHAT_PROJECT_KEY || "";
if (projectKey) {
  await page.setContent(`<!doctype html><html lang="fa" dir="rtl"><body style="font-family:Vazirmatn,sans-serif;padding:40px">
    <h1>نمونه سایت</h1><p>ویجت پشتیبانی در گوشه صفحه است.</p>
    <script>window.EveChat={projectKey:${JSON.stringify(projectKey)},apiUrl:${JSON.stringify(base)}};</script>
    <script src="${base}/widget.js"></script></body></html>`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1500);
  await page.evaluate(() => window.EveChat && window.EveChat.open && window.EveChat.open());
  await page.waitForTimeout(1200);
  await page.screenshot({ path: `${outDir}/widget.png` });
  console.log("✓", `${outDir}/widget.png`);
}

await browser.close();
console.log("done");
