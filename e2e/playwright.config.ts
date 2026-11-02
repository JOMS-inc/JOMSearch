import { defineConfig, devices } from "@playwright/test";

// The app must already be running. BASE_URL picks which one to test:
//   local/CI:  http://localhost:8080 (default)
//   prod:      BASE_URL=https://jomsearch.dk npm run test:readonly
export default defineConfig({
  testDir: "./tests",
  timeout: 30_000,
  forbidOnly: !!process.env.CI, // a leftover test.only must not silently skip the rest in CI
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: process.env.BASE_URL ?? "http://localhost:8080",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
