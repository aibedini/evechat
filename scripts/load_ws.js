// k6 load test: 1000 concurrent visitor websockets.
//   k6 run scripts/load_ws.js -e BASE=http://127.0.0.1:3000 -e PROJECT_KEY=pk_...
import ws from "k6/ws";
import http from "k6/http";
import { check, sleep } from "k6";
import { Trend } from "k6/metrics";

const BASE = __ENV.BASE || "http://127.0.0.1:3000";
const PROJECT_KEY = __ENV.PROJECT_KEY || "";

const messageLatency = new Trend("message_latency", true);

export const options = {
  scenarios: {
    chat: {
      executor: "ramping-vus",
      startVUs: 50,
      stages: [
        { duration: "30s", target: 500 },
        { duration: "60s", target: 1000 },
        { duration: "120s", target: 1000 },
        { duration: "20s", target: 0 },
      ],
    },
  },
  thresholds: {
    message_latency: ["p(95)<50"],
    http_req_failed: ["rate<0.01"],
  },
};

export function setup() {
  const sessions = [];
  for (let i = 0; i < 1000; i++) {
    const res = http.post(
      `${BASE}/api/widget/v1/session?project_key=${PROJECT_KEY}`,
      JSON.stringify({
        visitor: { name: `Load ${i}`, metadata: { load_test: "true" } },
        page: { url: `${BASE}/load`, title: "load" },
      }),
      { headers: { "Content-Type": "application/json" } }
    );
    if (res.status === 200) sessions.push(res.json("token"));
  }
  return { sessions };
}

export default function (data) {
  const token = data.sessions[__VU % data.sessions.length];
  const url = BASE.replace("http", "ws") + `/ws/visitor?token=${token}`;
  const started = Date.now();

  const res = ws.connect(url, {}, function (socket) {
    socket.on("open", () => {
      socket.setInterval(() => {
        socket.send(JSON.stringify({ type: "ping" }));
      }, 25000);
    });

    socket.on("message", (raw) => {
      const env = JSON.parse(raw);
      if (env.type === "message.received") {
        messageLatency.add(Date.now() - started);
      }
    });

    socket.setTimeout(() => {
      socket.send(
        JSON.stringify({
          type: "conversation.start",
          id: `${__VU}-${__ITER}-${Date.now()}`,
          data: { message: "سلام، این یک پیام تست بار است." },
        })
      );
    }, 200);

    socket.setTimeout(() => socket.close(), 30000);
  });

  check(res, { "ws connected": (r) => r && r.status === 101 });
  sleep(1);
}
