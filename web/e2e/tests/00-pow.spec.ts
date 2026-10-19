import { test, expect } from "@playwright/test";

// Requires a non-zero proof-of-work difficulty (CI sets 14). The cooperative
// solver must yield often enough that no single main-thread task exceeds 50ms.
test("widget PoW does not block the host page", async ({ page }) => {
  await page.goto("/demo/test");
  await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 30_000 });

  const difficulty = await page.evaluate(async () => {
    const key = (window as any).EveChat?.projectKey;
    if (!key) return 0;
    const res = await fetch(`/api/widget/v1/challenge?project_key=${encodeURIComponent(key)}`);
    return res.ok ? Number((await res.json()).difficulty) : 0;
  });
  expect(difficulty).toBeGreaterThan(0);

  await page.evaluate(() => {
    (window as any).__longTasks = [];
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) (window as any).__longTasks.push(entry.duration);
    }).observe({ entryTypes: ["longtask"] });
  });

  await page.evaluate(() => (window as any).ec("open"));
  await page.waitForFunction(() => Boolean((window as any).EveChat?.getState?.().ready), undefined, { timeout: 30_000 });
  await page.waitForTimeout(150);

  const maxTask = await page.evaluate(() => Math.max(0, ...((window as any).__longTasks || [])));
  expect(maxTask).toBeLessThan(50);
});
