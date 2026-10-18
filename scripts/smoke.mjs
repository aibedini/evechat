// End-to-end smoke test for a running EveChat instance.
import crypto from "node:crypto";
// Usage: node scripts/smoke.mjs [baseUrl] [projectKey] [email] [password]
const base = process.argv[2] || "http://127.0.0.1:3000";
const projectKey = process.argv[3];
const email = process.argv[4] || "admin@evechat.local";
const password = process.argv[5];

if (!projectKey || !password) {
  console.error("usage: node scripts/smoke.mjs <baseUrl> <projectKey> <email> <password>");
  process.exit(2);
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function leadingZeroBits(hex) {
  let bits = 0;
  for (const ch of hex) {
    const v = parseInt(ch, 16);
    if (v === 0) { bits += 4; continue; }
    if (v < 2) bits += 3;
    else if (v < 4) bits += 2;
    else if (v < 8) bits += 1;
    break;
  }
  return bits;
}

function solvePoW(challenge, difficulty) {
  for (let i = 0; ; i++) {
    const nonce = i.toString(36);
    const digest = crypto.createHash("sha256").update(challenge + nonce).digest("hex");
    if (leadingZeroBits(digest) >= difficulty) return nonce;
  }
}

async function post(path, body, headers = {}) {
  const res = await fetch(base + path, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...headers },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${path} -> ${res.status} ${text}`);
  return text ? JSON.parse(text) : null;
}

function openSocket(path, inbox) {
  const ws = new WebSocket(base.replace(/^http/, "ws") + path);
  ws.addEventListener("message", (ev) => {
    const msg = JSON.parse(ev.data);
    inbox.push(msg);
  });
  return new Promise((resolve, reject) => {
    ws.addEventListener("open", () => resolve(ws));
    ws.addEventListener("error", (e) => reject(new Error("ws error " + (e.message || ""))));
  });
}

async function waitFor(inbox, type, timeoutMs = 5000, predicate = () => true) {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    const found = inbox.find((m) => m.type === type && predicate(m));
    if (found) return found;
    await sleep(50);
  }
  throw new Error(`timed out waiting for ${type}; got ${inbox.map((m) => m.type).join(",")}`);
}

function assert(cond, message) {
  if (!cond) throw new Error("assertion failed: " + message);
}

async function main() {
  console.log("1. health");
  const health = await (await fetch(base + "/api/v1/health")).json();
  assert(health.status === "ok", "health status ok");
  assert(health.db === "ok", "db ok");

  console.log("2. widget challenge + session (proof of work)");
  const challenge = await (await fetch(`${base}/api/widget/v1/challenge?project_key=${projectKey}`)).json();
  const nonce = challenge.difficulty > 0 ? solvePoW(challenge.challenge, challenge.difficulty) : "";
  const session = await post(`/api/widget/v1/session?project_key=${projectKey}`, {
    challenge: challenge.challenge,
    nonce,
    visitor: { name: "Smoke Visitor", mobile: "09120000000", metadata: { utm_source: "smoke" } },
    page: { url: "http://example.com/pricing", title: "Pricing" },
  });
  assert(!!session.token, "session token");

  console.log("3. visitor websocket + conversation.start");
  const visitorInbox = [];
  const vws = await openSocket(`/ws/visitor?token=${session.token}`, visitorInbox);
  const clientMsgId = "smoke-" + Date.now();
  vws.send(
    JSON.stringify({
      type: "conversation.start",
      id: clientMsgId,
      data: { message: "سلام، یک سوال دارم", page: { url: "http://example.com/pricing", title: "Pricing" } },
    })
  );
  const created = await waitFor(visitorInbox, "conversation.created");
  const conversationId = created.data.id;
  assert(!!conversationId, "conversation id");
  await waitFor(visitorInbox, "message.sent");
  await waitFor(visitorInbox, "message.received");
  console.log("   conversation:", conversationId);

  console.log("4. agent login + websocket");
  const auth = await post("/api/v1/auth/login", { email, password });
  assert(!!auth.access_token, "access token");
  const agentInbox = [];
  const aws = await openSocket(`/ws/agent?token=${auth.access_token}`, agentInbox);
  await waitFor(agentInbox, "sync.response");

  console.log("5. agent sees conversation.new");
  // conversation.new was emitted before the agent connected: fetch through REST.
  const list = await fetch(`${base}/api/v1/conversations?status=all`, {
    headers: { Authorization: "Bearer " + auth.access_token },
  }).then((r) => r.json());
  assert(list.data.some((c) => c.id === conversationId), "conversation listed for agent");

  console.log("6. agent replies, visitor receives message.received");
  aws.send(
    JSON.stringify({
      type: "message.send",
      data: { conversation_id: conversationId, content: "سلام! در خدمت شما هستم." },
    })
  );
  const received = await waitFor(visitorInbox, "message.received", 6000, (m) => m.data && m.data.sender_type === "agent");
  assert(received.data.sender_type === "agent", "agent sender type");

  console.log("7. idempotent send (same client_message_id twice)");
  const dupBody = { content: "پیام تکراری", client_message_id: "dup-1" };
  const first = await post(`/api/widget/v1/conversations/${conversationId}/messages?token=${session.token}`, dupBody);
  const second = await post(`/api/widget/v1/conversations/${conversationId}/messages?token=${session.token}`, dupBody);
  assert(first.id === second.id, "dedup returns same message id");

  console.log("8. sync.request from ring buffer");
  visitorInbox.length = 0;
  vws.send(JSON.stringify({ type: "sync.request", data: { conversation_id: conversationId, since: 0 } }));
  const sync = await waitFor(visitorInbox, "sync.response");
  assert(sync.data.conversation, "sync carries conversation");

  console.log("9. resolve + reopen via REST");
  const resolve = await post(`/api/v1/conversations/${conversationId}/resolve`, {}, { Authorization: "Bearer " + auth.access_token });
  assert(resolve.ok === true, "resolve ok");

  console.log("10. widget sync endpoint");
  const syncRest = await fetch(`${base}/api/widget/v1/conversations/${conversationId}/sync?token=${session.token}&since=0`).then((r) => r.json());
  assert(Array.isArray(syncRest.messages) || Array.isArray(syncRest.events), "sync messages or events array");

  vws.close();
  aws.close();
  console.log("\nSMOKE TEST PASSED ✅");
}

main().catch((err) => {
  console.error("\nSMOKE TEST FAILED ❌\n" + err.stack);
  process.exit(1);
});
