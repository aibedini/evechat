import { defineConfig } from "@playwright/test";

// The suite runs against a live instance (see docs/E2E.md). CI starts
// \`evechat serve\` on a temp data dir and points EVECHAT_BASE at it.
export default defineConfig({
  testDir: "./tests",
  timeout: 45_000,
  expect: { timeout: 8_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [["list"]],
  use: {
    baseURL: process.env.EVECHAT_BASE || "http://127.0.0.1:3411",
    headless: true,
    trace: "retain-on-failure",
  },
});