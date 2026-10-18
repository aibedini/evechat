// Guards the "no runtime dependency on an external host" rule: scans the built
// bundles for absolute URLs and fails on anything that is not an explicit
// allow-listed documentation/example link or a user-provided apiUrl.
import { readFileSync, existsSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const ALLOW = [
  "https://github.com/aibedini/evechat",
  "https://t.me/BotFather",
  "https://chat.example.com",
  "http://localhost",
  "http://127.0.0.1",
  "http://www.w3.org",
  "https://opensource.org",
  "https://raw.githubusercontent.com",
  "https://api.anthropic.com",
  "https://api.telegram.org",
  "https://reactjs.org/docs/error-decoder.html",
  "https://react.dev/errors/",
  "https://eve.ir/",
  "http://eve.ir/",
];

const roots = ["embed/dashboard", "embed/widget", "embed/setup"];
const files = [];

function walk(dir) {
  if (!existsSync(dir)) return;
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) walk(p);
    else if (/\.(js|css|html)$/.test(entry)) files.push(p);
  }
}

roots.forEach(walk);
if (files.length === 0) {
  console.error("no built bundles found; run the web builds first");
  process.exit(1);
}

const urlRe = /https?:\/\/[^\s"'<>()\\]+/g;
let violations = 0;
for (const file of files) {
  const text = readFileSync(file, "utf8");
  for (const raw of text.match(urlRe) || []) {
    const url = raw.replace(/[.,;)]+$/, "");
    if (ALLOW.some((a) => url.startsWith(a))) continue;
    // URLs assembled at runtime from window.EveChat.apiUrl are relative here.
    if (url.includes("example.com") || url.includes("example.org")) continue;
    console.error(`✗ external URL in ${file}: ${url}`);
    violations++;
  }
}
if (violations > 0) {
  console.error(`\n${violations} unexpected external URL(s); ship everything in the binary`);
  process.exit(1);
}
console.log(`✓ ${files.length} bundle files contain no unexpected external URLs`);
