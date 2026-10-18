import { test, expect, type BrowserContext, type Page } from "@playwright/test";

// Seven scenarios from the bugfix sprint, one per shipped defect:
//   1-2  BUG-1  resume: history and read state survive a reload
//   3    BUG-1  a resolved thread never swallows the next visitor message
//   4    BUG-3  a message on a pending (unassigned) conversation reaches the agent
//   5    BUG-3  typing: widget -> server -> panel, with a server-side TTL
//   6    BUG-2  claiming a pending conversation activates and owns it
//   7    BUG-0  the widget still works when storage is blocked
const widget = '[data-evechat="widget"]';
const ADMIN = {
  email: process.env.EVECHAT_ADMIN_EMAIL || "admin@evechat.local",
  password: process.env.EVECHAT_ADMIN_PASSWORD || "Passw0rd123!",
};

async function visitorPage(context: BrowserContext): Promise<Page> {
  const page = await context.newPage();
  await page.goto("/demo/test");
  await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
  await page.evaluate(() => (window as any).ec("open"));
  return page;
}

async function agentPage(context: BrowserContext, baseURL: string): Promise<Page> {
  const login = await context.request.post(baseURL + "/api/v1/auth/login", { data: ADMIN });
  if (login.status() !== 200) {
    throw new Error("login failed: " + login.status() + " " + (await login.text()));
  }
  const auth = await login.json();
  const page = await context.newPage();
  await page.addInitScript((data) => {
    localStorage.setItem("evechat:access", data.token);
    localStorage.setItem("evechat:refresh", data.refresh);
    localStorage.setItem("evechat:session", JSON.stringify({ agent: data.agent, project: data.project }));
  }, { token: auth.access_token, refresh: auth.refresh_token, agent: auth.agent, project: auth.project });
  await page.goto("/inbox");
  await expect(page.getByText("صندوق گفتگو").first()).toBeVisible({ timeout: 20_000 });
  return page;
}

async function agentToken(context: BrowserContext, baseURL: string): Promise<string> {
  const login = await context.request.post(baseURL + "/api/v1/auth/login", { data: ADMIN });
  if (login.status() !== 200) {
    throw new Error("login failed: " + login.status() + " " + (await login.text()));
  }
  return (await login.json()).access_token;
}

test.describe("behaviour", () => {
  test("1. resume keeps the conversation history", async ({ page }) => {
    await page.goto("/demo/test");
    await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
    await page.evaluate(() => (window as any).ec("open"));
    const text = "پیام ماندگار " + Date.now();
    await page.evaluate((t) => (window as any).ec("send", t), text);
    await expect(page.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(text);

    await page.reload();
    await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
    await page.evaluate(() => (window as any).ec("open"));
    await expect(page.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(text, { timeout: 15_000 });
  });

  test("2. resume does not resurrect read messages as unread", async ({ page }) => {
    await page.goto("/demo/test");
    await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
    await page.evaluate(() => (window as any).ec("open"));
    await page.evaluate(() => (window as any).ec("close"));
    await page.reload();
    await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
    await page.waitForTimeout(1500);
    const badge = page.locator(widget + " .ec-badge");
    const shown = await badge.evaluate((el) => el.classList.contains("on")).catch(() => false);
    const value = shown ? await badge.textContent() : "";
    expect(shown && value && value !== "0").toBeFalsy();
  });

  test("3. a resolved thread does not swallow the next message", async ({ page }) => {
    await page.goto("/demo/test");
    await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
    await page.evaluate(() => (window as any).ec("open"));
    const text = "قبل از resolve " + Date.now();
    await page.evaluate((t) => (window as any).ec("send", t), text);
    await expect(page.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(text);

    const result = await page.evaluate(async ({ admin }) => {
      const auth = await fetch("/api/v1/auth/login", {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(admin),
      }).then((r) => r.json());
      const headers = { Authorization: "Bearer " + auth.access_token };
      const list = await fetch("/api/v1/conversations?status=pending", { headers }).then((r) => r.json());
      const conv = list.data[0];
      const res = await fetch("/api/v1/conversations/" + conv.id + "/resolve", { method: "POST", headers, body: "{}" });
      return { convId: conv.id, status: res.status };
    }, { admin: ADMIN });
    expect(result.status).toBe(200);

    await page.waitForTimeout(800);
    const after = "بعد از resolve " + Date.now();
    await page.evaluate((t) => (window as any).ec("send", t), after);
    await page.waitForTimeout(1500);

    const newest = await page.evaluate(async ({ admin }) => {
      const auth = await fetch("/api/v1/auth/login", {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(admin),
      }).then((r) => r.json());
      const list = await fetch("/api/v1/conversations?status=all&limit=5", {
        headers: { Authorization: "Bearer " + auth.access_token },
      }).then((r) => r.json());
      return list.data[0].id;
    }, { admin: ADMIN });
    expect(newest).not.toBe(result.convId);
  });

  test("4. a pending conversation delivers messages to the agent", async ({ browser, baseURL }) => {
    const visitorCtx = await browser.newContext();
    const agentCtx = await browser.newContext();
    const visitor = await visitorPage(visitorCtx);
    const first = "پیام اول " + Date.now();
    await visitor.evaluate((t) => (window as any).ec("send", t), first);
    await expect(visitor.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(first);

    const agent = await agentPage(agentCtx, baseURL!);
    const row = agent.getByText("کاربر تست").first();
    await expect(row).toBeVisible({ timeout: 20_000 });
    await row.click();

    const live = "پیام زنده " + Date.now();
    await visitor.evaluate((t) => (window as any).ec("send", t), live);
    await expect(agent.getByText(live).first()).toBeVisible({ timeout: 20_000 });

    await visitorCtx.close();
    await agentCtx.close();
  });

  test("5. typing reaches the panel and expires server-side", async ({ browser, baseURL }) => {
    const visitorCtx = await browser.newContext();
    const agentCtx = await browser.newContext();
    const visitor = await visitorPage(visitorCtx);
    const first = "شروع گفتگو " + Date.now();
    await visitor.evaluate((t) => (window as any).ec("send", t), first);
    await expect(visitor.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(first);

    const agent = await agentPage(agentCtx, baseURL!);
    const row = agent.getByText("کاربر تست").first();
    await expect(row).toBeVisible({ timeout: 20_000 });
    await row.click();

    const token = await agentToken(agentCtx, baseURL!);
    const list = await agentCtx.request.get(baseURL + "/api/v1/conversations?status=pending", {
      headers: { Authorization: "Bearer " + token },
    });
    const convId = (await list.json()).data[0].id as string;
    const visitorToken = await visitor.evaluate(() => {
      const key = (window as any).EveChat.projectKey;
      return localStorage.getItem("evechat:token:" + key) || "";
    });
    expect(visitorToken).not.toBe("");

    await visitor.evaluate(async ({ token, convId }) => {
      const ws = new WebSocket(location.origin.replace(/^http/, "ws") + "/ws/visitor?token=" + encodeURIComponent(token));
      await new Promise<void>((resolve) => ws.addEventListener("open", () => resolve()));
      ws.send(JSON.stringify({ type: "typing.start", data: { conversation_id: convId } }));
      (window as any).__typingSocket = ws;
    }, { token: visitorToken, convId });

    await expect(agent.getByText("در حال تایپ…").first()).toBeVisible({ timeout: 15_000 });
    // No typing.stop frame: only the 5s server TTL can clear it.
    await expect(agent.getByText("در حال تایپ…").first()).toBeHidden({ timeout: 20_000 });

    await visitorCtx.close();
    await agentCtx.close();
  });

  test("6. claiming a pending conversation activates and owns it", async ({ browser, baseURL }) => {
    const visitorCtx = await browser.newContext();
    const visitor = await visitorPage(visitorCtx);
    const token = await agentToken(visitorCtx, baseURL!);
    const me = await (await visitorCtx.request.get(baseURL + "/api/v1/auth/me", {
      headers: { Authorization: "Bearer " + token },
    })).json();
    const listPending = async () => {
      const res = await visitorCtx.request.get(baseURL + "/api/v1/conversations?status=pending&limit=50", {
        headers: { Authorization: "Bearer " + token },
      });
      if (res.status() !== 200) console.log("listPending status", res.status(), await res.text());
      return ((await res.json()).data || []) as Array<{ id: string; status: string }>;
    };

    // Identify our conversation by set difference: the suite shares one instance
    // and every demo visitor is called "کاربر تست".
    const before = new Set((await listPending()).map((c) => c.id));
    const first = "برای برداشتن " + Date.now();
    await visitor.evaluate((t) => (window as any).ec("send", t), first);
    await expect(visitor.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(first);

    const created = (await listPending()).find((c) => !before.has(c.id));
    expect(created).toBeTruthy();
    const convId = created!.id;

    const claim = await visitorCtx.request.post(baseURL + "/api/v1/conversations/" + convId + "/claim", {
      headers: { Authorization: "Bearer " + token },
      data: {},
    });
    expect(claim.status()).toBe(200);

    const detail = await (await visitorCtx.request.get(baseURL + "/api/v1/conversations/" + convId, {
      headers: { Authorization: "Bearer " + token },
    })).json();
    expect(detail.conversation.status).toBe("active");
    expect(detail.conversation.assigned_agent_id).toBe(me.agent.id);

    // A second claim must lose: the conversation is no longer pending.
    const again = await visitorCtx.request.post(baseURL + "/api/v1/conversations/" + convId + "/claim", {
      headers: { Authorization: "Bearer " + token },
      data: {},
    });
    expect(again.status()).toBeGreaterThanOrEqual(400);

    await visitorCtx.close();
  });

  test("7. the widget survives blocked localStorage and IndexedDB", async ({ browser }) => {
    const ctx = await browser.newContext();
    await ctx.addInitScript(() => {
      const boom = () => {
        throw new DOMException("blocked", "SecurityError");
      };
      Object.defineProperty(window, "localStorage", { get: boom });
      Object.defineProperty(window, "sessionStorage", { get: boom });
      Object.defineProperty(window, "indexedDB", { get: () => undefined });
    });
    const page = await ctx.newPage();
    await page.goto("/demo/test");
    await page.waitForFunction(() => Boolean((window as any).EveChat?.ui), undefined, { timeout: 20_000 });
    await page.evaluate(() => (window as any).ec("open"));
    const text = "بدون حافظه " + Date.now();
    await page.evaluate((t) => (window as any).ec("send", t), text);
    await expect(page.locator(widget + " .ec-msgs .ec-msg.visitor").last()).toContainText(text, { timeout: 15_000 });
    await ctx.close();
  });
});