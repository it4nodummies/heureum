import { defineConfig } from "@playwright/test";

// Origin of the backend webServer below. Specs that call the REST API directly
// (page.request, e.g. e2e/epics.spec.ts) read it from E2E_API_URL; set here so
// a config copy that moves the ports keeps the specs in step.
if (!process.env.E2E_API_URL) process.env.E2E_API_URL = "http://localhost:8080";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  retries: process.env.CI ? 2 : 0,
  // Every spec runs against ONE shared backend with ONE SQLite file
  // (scripts/e2e-backend.sh). Playwright's default is half the CPU count, which
  // puts two spec files in write contention on that single database — the
  // flakiness recorded in STATE.md since R13, and the real cause of PR #24's red
  // e2e job (adding one spec file redistributed the load across two workers).
  // Round 24 gives each worker its own database and earns the parallelism back;
  // until then this must stay 1.
  workers: 1,
  use: {
    baseURL: "http://localhost:3000",
    trace: "on-first-retry",
  },
  webServer: [
    {
      command: "bash ../scripts/e2e-backend.sh",
      url: "http://localhost:8080/healthz",
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: "npm run dev",
      url: "http://localhost:3000/login",
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: { NEXT_PUBLIC_API_URL: "http://localhost:8080" },
    },
  ],
});
