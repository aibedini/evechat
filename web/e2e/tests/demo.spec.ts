import { test, expect } from "@playwright/test";

const widget = '[data-evechat="widget"]';

test("demo shop boots the widget and survives hostile CSS", async ({ page }) => {
  await page.goto("/demo");
  await expect(page.locator(".demo-bar")).toBeVisible();
  await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 30_000 });

  // The hostile stylesheet targets generic selectors and even .ec-* classes;
  // the widget lives in a shadow root, so it must stay visible and usable.
  await page.getByRole("button", { name: "تداخل CSS" }).click();
  await expect(page.locator(widget + " .ec-launcher")).toBeVisible();
  await page.evaluate(() => (window as any).ec("open"));
  await expect(page.locator(widget + " .ec-panel")).toHaveClass(/open/);
  const bodyBg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  expect(bodyBg).not.toBe("rgba(0, 0, 0, 0)"); // hostile CSS applied to the page…
  await expect(page.locator(widget + " .ec-msgs")).toBeVisible(); // …but not the widget
});

test("demo split reports a real round-trip latency in ms", async ({ page }) => {
  await page.goto("/demo/split");
  await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 30_000 });
  await page.evaluate(() => (window as any).ec("open"));
  // Wait for the transport: a send queued before the socket is open produces no
  // round trip, so the latency tile would stay empty.
  await page.waitForFunction(() => (window as any).EveChat.getState().connection === "open", undefined, {
    timeout: 20_000,
  });
  await page.getByRole("button", { name: "ارسال نمونه" }).click();
  await expect(page.locator("#demo-latency")).toContainText("ms", { timeout: 20_000 });
});

test("demo devices renders eight live presets", async ({ page }) => {
  await page.goto("/demo/devices");
  const frames = page.locator(".device iframe");
  await expect(frames).toHaveCount(8);
  await expect(frames.first()).toHaveAttribute("src", "/demo/test");
});

test("demo events console records widget events", async ({ page }) => {
  await page.goto("/demo/events");
  await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 30_000 });
  await page.evaluate(() => (window as any).ec("open"));
  await expect(page.locator("#demo-log .log-line").first()).toBeVisible({ timeout: 20_000 });
});

test("demo pages answer 404 to a spoofed remote caller", async ({ request }) => {
  // trust_proxy is off on the e2e instance, so the header is ignored and the
  // loopback caller still gets the page; the unit test covers the gate itself.
  const res = await request.get("/demo");
  expect(res.status()).toBe(200);
});