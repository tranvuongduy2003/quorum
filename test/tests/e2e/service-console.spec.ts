import { expect, test, type Page } from "@playwright/test"
import { randomUUID } from "node:crypto"

const apiBaseURL = process.env.PLAYWRIGHT_API_BASE_URL ?? "http://127.0.0.1:8080"

const uniqueForwardedAddress = () =>
  `2001:db8:${randomUUID().replaceAll("-", "").slice(0, 24).replace(/(.{4})(?=.)/g, "$1:")}`

async function useOneRateLimitIdentity(page: Page, uniquePerRequest = false): Promise<void> {
  const identity = uniqueForwardedAddress()

  await page.route(`${apiBaseURL}/api/v1/**`, (route) =>
    route.fallback({
      headers: {
        ...route.request().headers(),
        "X-Forwarded-For": uniquePerRequest ? uniqueForwardedAddress() : identity,
      },
    })
  )
}

test("the service console exposes live status and performs diagnostic workflows", async ({ page }) => {
  await useOneRateLimitIdentity(page)
  await page.goto("/")

  await expect(page.getByRole("heading", { name: "Quorum service console" })).toBeVisible()
  await expect(page.getByText("alive", { exact: true })).toBeVisible()
  await expect(page.getByText("ready", { exact: true })).toBeVisible()
  await expect(page.getByText("postgres", { exact: true })).toBeVisible()
  await expect(page.getByText("redis", { exact: true })).toBeVisible()
  await expect(page.getByRole("link", { name: /API reference/ })).toBeVisible()

  await page.getByRole("button", { name: "Send ping" }).click()
  await expect(page.getByText("pong", { exact: true })).toBeVisible()

  await page.getByLabel("Message").fill("  hello  ")
  await page.getByRole("button", { name: "Send echo" }).click()
  await expect(page.getByText("The server echoed")).toContainText("hello")
  await expect(page.getByText("The server echoed")).toContainText("5 characters")

  await page.getByRole("button", { name: "Get database time" }).click()
  await expect(page.getByText("Clock skew:")).toBeVisible()
  await expect(page.getByText("0 / 3", { exact: true })).toBeVisible()
})

test("the failure explorer renders every documented server failure", async ({ page }) => {
  test.setTimeout(30000)
  await useOneRateLimitIdentity(page, true)
  await page.goto("/")

  for (const scenario of [
    { label: "Not found", status: 404, code: "NOT_FOUND" },
    { label: "Method not allowed", status: 405, code: "METHOD_NOT_ALLOWED" },
    { label: "Invalid JSON", status: 400, code: "INVALID_JSON" },
    { label: "Validation failed", status: 400, code: "VALIDATION_FAILED" },
    { label: "Payload too large", status: 413, code: "PAYLOAD_TOO_LARGE" },
    { label: "Server panic", status: 500, code: "INTERNAL" },
    { label: "Request timeout", status: 504, code: "REQUEST_TIMEOUT" },
  ]) {
    await page.getByRole("button", { name: scenario.label }).click()
    const alert = page.getByRole("alert")

    await expect(alert).toContainText(scenario.code, { timeout: 25000 })
    await expect(alert).toContainText(`HTTP ${scenario.status}`)
  }
})

test("the service console explains when its API cannot be reached", async ({ page }) => {
  await page.route(`${apiBaseURL}/**`, (route) => route.abort("failed"))
  await page.goto("/")

  await expect(page.getByText("unreachable", { exact: true })).toHaveCount(2)
  await expect(
    page.getByText("The API reference is not published by this server.")
  ).toBeVisible()
})

test("the service console hides the reference when its document is unavailable", async ({ page }) => {
  await page.route(`${apiBaseURL}/openapi.yaml`, (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "NOT_FOUND",
          message: "the requested resource does not exist",
          request_id: "playwright-docs-not-found",
        },
      }),
    })
  )
  await page.goto("/")

  await expect(page.getByText("ready", { exact: true })).toBeVisible()
  await expect(page.getByRole("link", { name: /API reference/ })).toHaveCount(0)
})
