import { test, expect } from "@playwright/test";
import http, { type IncomingHttpHeaders } from "node:http";
import type { AddressInfo } from "node:net";

// The outbound test callback points at a receiver in the Playwright process.
// The EveChat instance must allow loopback for this test:
//   EVECHAT_SECURITY__OUTBOUND_ALLOW_HOSTS=127.0.0.1
const ADMIN = {
  email: process.env.EVECHAT_ADMIN_EMAIL || "admin@evechat.local",
  password: process.env.EVECHAT_ADMIN_PASSWORD || "Passw0rd123!",
};

async function signIn(page: import("@playwright/test").Page, baseURL: string): Promise<string> {
  const login = await page.request.post(baseURL + "/api/v1/auth/login", { data: ADMIN });
  expect(login.status()).toBe(200);
  const auth = await login.json();
  await page.addInitScript((data) => {
    localStorage.setItem("evechat:access", data.token);
    localStorage.setItem("evechat:refresh", data.refresh);
    localStorage.setItem("evechat:session", JSON.stringify({ agent: data.agent, project: data.project }));
  }, { token: auth.access_token, refresh: auth.refresh_token, agent: auth.agent, project: auth.project });
  return auth.access_token;
}

test("SMTP card is visible and reports its status", async ({ page, baseURL }) => {
  await signIn(page, baseURL!);
  await page.goto("/settings/integrations");
  const card = page.getByTestId("integration-smtp");
  await expect(card).toBeVisible();
  await expect(card.getByText("ایمیل", { exact: true })).toBeVisible();
  await expect(card.getByText("تنظیم نشده", { exact: true }).first()).toBeVisible();
});

test("channel outbound X-API-Key reaches the receiver and is masked in the inspector", async ({ page, baseURL }) => {
  const token = await signIn(page, baseURL!);
  const name = "E2E channel " + Date.now();
  const received: IncomingHttpHeaders[] = [];

  const receiver = http.createServer((req, res) => {
    received.push(req.headers);
    req.resume();
    req.on("end", () => {
      res.writeHead(200, { "content-type": "application/json" });
      res.end('{"ok":true}');
    });
  });
  await new Promise<void>((resolve) => receiver.listen(0, "127.0.0.1", () => resolve()));
  const port = (receiver.address() as AddressInfo).port;

  try {
    await page.goto("/settings/channels");
    await expect(page.getByRole("heading", { name: "کانال‌ها" })).toBeVisible();

    await page.getByRole("button", { name: /کانال جدید/ }).click();
    await page.getByTestId("channel-name").fill(name);
    await page.getByTestId("channel-outbound-url").fill(`http://127.0.0.1:${port}/hook`);
    await page.getByTestId("channel-ttl").fill("3600");
    await page.getByRole("button", { name: /افزودن هدر/ }).click();
    await page.getByTestId("channel-header-key-0").fill("X-API-Key");
    await page.getByTestId("channel-header-value-0").fill("e2e-secret");
    await page.getByTestId("channel-save").click();

    await expect(page.getByText(name).first()).toBeVisible();

    await page.getByRole("button", { name: "ارسال آزمایشی" }).first().click();
    await expect.poll(() => received.length, { timeout: 15_000 }).toBeGreaterThan(0);
    expect(received[0]["x-api-key"]).toBe("e2e-secret");

    await expect(page.getByText(/ارسال آزمایشی موفق بود/)).toBeVisible();

    const list = await page.request.get(baseURL + "/api/v1/channels", {
      headers: { Authorization: "Bearer " + token },
    });
    expect(list.status()).toBe(200);
    const channels = (await list.json()).data as Array<any>;
    const created = channels.find((c) => c.name === name);
    expect(created).toBeTruthy();
    const maskedKey = Object.keys(created.outbound_headers).find((k) => k.toLowerCase() === "x-api-key");
    expect(maskedKey).toBeTruthy();
    expect(created.outbound_headers[maskedKey!]).toBe("••••••");
    expect(created.session_ttl_seconds).toBe(3600);

    await page.getByRole("button", { name: "بازرس" }).first().click();
    const details = page.locator("details").first();
    await details.locator("summary").click();
    await expect(details).toContainText(/x-api-key/i);
    await expect(details).toContainText("••••••");

    const audit = await page.request.get(baseURL + "/api/v1/audit?limit=50", {
      headers: { Authorization: "Bearer " + token },
    });
    expect(await audit.text()).not.toContain("e2e-secret");
  } finally {
    await new Promise<void>((resolve) => {
      receiver.close(() => resolve());
      receiver.closeAllConnections();
    });
    const list = await page.request.get(baseURL + "/api/v1/channels", {
      headers: { Authorization: "Bearer " + token },
    });
    if (list.ok()) {
      const channels = (await list.json()).data as Array<any>;
      const created = channels.find((c) => c.name === name);
      if (created) {
        await page.request.delete(baseURL + "/api/v1/channels/" + created.id, {
          headers: { Authorization: "Bearer " + token },
        });
      }
    }
  }
});
