#!/usr/bin/env node
// Guards the TASK 43 UX-writing rules:
//   1. Persian text only lives in web/dashboard/src/lib/copy.fa.ts.
//   2. Every sidebar page has a pages.<key>.intro of at most 220 characters.
//   3. Every `empty:` block has a `cta` (an empty state always has a next step).
//
// Files that are still being migrated are listed in PENDING; the list shrinks
// to empty with TASK 43b/43c. Do not add new files to it.
import fs from "node:fs";
import path from "node:path";

const root = path.resolve("web/dashboard/src");
const copyFile = path.join(root, "lib/copy.fa.ts");
const persian = /[\u0600-\u06FF]/;
const maxIntro = 220;

const navKeys = [
  "inbox",
  "dashboard",
  "customers",
  "tickets",
  "knowledgeBase",
  "widget",
  "routing",
  "team",
  "integrations",
  "channels",
  "webhooks",
  "security",
  "activity",
  "health",
];

const PENDING = new Set([
  "components/ChatPanel.tsx",
  "components/ConversationList.tsx",
  "components/Sidebar.tsx",
  "components/VisitorPanel.tsx",
  "pages/Dashboard.tsx",
  "pages/Forgot.tsx",
  "pages/Inbox.tsx",
  "pages/KnowledgeBase.tsx",
  "pages/Login.tsx",
  "pages/Reset.tsx",
  "pages/Tickets.tsx",
  "pages/Visitors.tsx",
  "pages/Wallboard.tsx",
  "pages/settings/Activity.tsx",
  "pages/settings/Channels.tsx",
  "pages/settings/Diagnostics.tsx",
  "pages/settings/Integrations.tsx",
  "pages/settings/Routing.tsx",
  "pages/settings/Security.tsx",
  "pages/settings/Team.tsx",
  "pages/settings/Webhooks.tsx",
  "pages/settings/Widget.tsx",
]);

const errors = [];

function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full);
    else if (/\.(ts|tsx)$/.test(entry.name)) inspect(full);
  }
}

function inspect(file) {
  if (file === copyFile) return;
  if (/\.(test|spec)\.(ts|tsx)$/.test(file)) return;
  const rel = path.relative(root, file).split(path.sep).join("/");
  if (PENDING.has(rel)) return;
  const lines = fs.readFileSync(file, "utf8").split(/\r?\n/);
  lines.forEach((line, index) => {
    if (persian.test(line)) {
      errors.push(`${rel}:${index + 1}: Persian text outside copy.fa.ts: ${line.trim()}`);
    }
  });
}

walk(root);

if (!fs.existsSync(copyFile)) {
  errors.push("web/dashboard/src/lib/copy.fa.ts is missing");
} else {
  const source = fs.readFileSync(copyFile, "utf8");
  for (const section of ["errors", "toasts", "glossary"]) {
    if (!new RegExp(`^  ${section}: \\{`, "m").test(source)) {
      errors.push(`copy.${section} is missing`);
    }
  }

  const pages = new Map();
  const pageRe = /^    ([A-Za-z0-9_]+): \{\r?\n([\s\S]*?)^    \},?$/gm;
  let match;
  while ((match = pageRe.exec(source))) pages.set(match[1], match[2]);

  for (const key of navKeys) {
    const block = pages.get(key);
    if (!block) {
      errors.push(`copy.pages.${key} is missing (sidebar route has no intro)`);
      continue;
    }
    const intro = block.match(/intro:\s*"((?:[^"\\]|\\.)*)"/);
    if (!intro) {
      errors.push(`copy.pages.${key}.intro is missing`);
    } else {
      const length = [...intro[1]].length;
      if (length > maxIntro) {
        errors.push(`copy.pages.${key}.intro is ${length} characters (max ${maxIntro})`);
      }
    }
    if (/empty:\s*\{/.test(block) && !/cta:/.test(block)) {
      errors.push(`copy.pages.${key}.empty has no cta`);
    }
  }
}

if (errors.length > 0) {
  console.error(`check-copy: ${errors.length} problem(s)`);
  for (const error of errors) console.error(`  - ${error}`);
  process.exit(1);
}

const pending = PENDING.size > 0 ? ` (${PENDING.size} file(s) still pending)` : "";
console.log(`check-copy: ok${pending}`);
