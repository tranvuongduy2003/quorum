import { defineConfig } from "@playwright/test"

const apiBaseURL = process.env.PLAYWRIGHT_API_BASE_URL ?? "http://127.0.0.1:8080"
const appBaseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:5173"
const startServers = process.env.PLAYWRIGHT_NO_WEB_SERVER !== "true"
const appOrigin = new URL(appBaseURL).origin

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: appBaseURL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: startServers
    ? [
        {
          command: "go run ./cmd/api",
          cwd: "../backend",
          url: `${apiBaseURL}/readyz`,
          timeout: 120000,
          reuseExistingServer: !process.env.CI,
          env: {
            ...process.env,
            BACKEND_PORT: "8080",
            CORS_ALLOWED_ORIGINS: appOrigin,
            DEBUG_ROUTES_ENABLED: "true",
            DOCS_ENABLED: "true",
            ENVIRONMENT: "development",
            LOG_FORMAT: "text",
            LOG_LEVEL: "warn",
            POSTGRES_DB: "app",
            POSTGRES_HOST: "localhost",
            POSTGRES_MAX_CONNS: "10",
            POSTGRES_MIN_CONNS: "2",
            POSTGRES_PASSWORD: "app",
            POSTGRES_PORT: "5432",
            POSTGRES_SSLMODE: "disable",
            POSTGRES_USER: "app",
            RATE_LIMIT_ENABLED: "true",
            RATE_LIMIT_FAIL_MODE: "closed",
            RATE_LIMIT_REQUESTS: "3",
            RATE_LIMIT_WINDOW: "1m",
            REDIS_DB: "0",
            REDIS_HOST: "localhost",
            REDIS_PORT: "6379",
            REQUEST_TIMEOUT: "15s",
            SERVER_IDLE_TIMEOUT: "60s",
            SERVER_READ_TIMEOUT: "10s",
            SERVER_WRITE_TIMEOUT: "35s",
            SHUTDOWN_TIMEOUT: "10s",
            STARTUP_PROBE_TIMEOUT: "5s",
            TRUST_PROXY_HEADERS: "true",
          },
        },
        {
          command: "npm run dev -- --host 127.0.0.1 --port 5173 --strictPort",
          cwd: "../frontend",
          url: appBaseURL,
          timeout: 120000,
          reuseExistingServer: !process.env.CI,
          env: {
            ...process.env,
            VITE_API_BASE_URL: apiBaseURL,
          },
        },
      ]
    : undefined,
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
})
