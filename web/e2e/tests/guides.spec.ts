import { test, expect } from "@playwright/test";

const ADMIN = {
  email: process.env.EVECHAT_ADMIN_EMAIL || "admin@evechat.local",
  password: process.env.EVECHAT_ADMIN_PASSWORD || "Passw0rd123!",
};

async function signIn(page: import("@playwright/test").Page, baseURL: string): Promise<void> {
  const login = await page.request.post(baseURL + "/api/v1/auth/login", { data: ADMIN });
  expect(login.status()).toBe(200);
  const auth = await login.json();
  await page.addInitScript((data) => {
    localStorage.setItem("evechat:access", data.token);
    localStorage.setItem("evechat:refresh", data.refresh);
    localStorage.setItem("evechat:session", JSON.stringify({ agent: data.agent, project: data.project }));
  }, { token: auth.access_token, refresh: auth.refresh_token, agent: auth.agent, project: auth.project });
}

// Every sidebar destination must explain itself inside the panel: a one-line
// intro plus a three-step "what do I do here?" guide, not a docs page.
const routes = [
  "/",
  "/inbox",
  "/visitors",
  "/tickets",
  "/knowledge-base",
  "/settings/widget",
  "/settings/routing",
  "/settings/team",
  "/settings/integrations",
  "/settings/webhooks",
  "/settings/channels",
  "/settings/security",
  "/settings/activity",
  "/settings/diagnostics",
];

test("every menu page has an in-panel intro and guide", async ({ page, baseURL }) => {
  await signIn(page, baseURL!);
  for (const route of routes) {
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await expect(page.locator(".page-intro")).toBeVisible();
    const guide = page.getByTestId("page-guide");
    await expect(guide, `missing guide on ${route}`).toBeVisible();
    await expect(guide.locator("li"), `wrong step count on ${route}`).toHaveCount(3);
  }
});

test("the guide stays reachable on a phone", async ({ page, baseURL }) => {
  await signIn(page, baseURL!);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/settings/channels");
  const guide = page.getByTestId("page-guide");
  await expect(guide).toBeVisible();
  await expect(guide.locator("li").first()).toBeHidden();
  await guide.getByRole("button", { name: "نمایش راهنما" }).click();
  await expect(guide.locator("li")).toHaveCount(3);
});
