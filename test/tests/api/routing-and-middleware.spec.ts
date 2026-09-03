import { expect, test } from "@playwright/test"
import { apiBaseURL, ErrorEnvelope, requestHeaders } from "./support.js"

test("unknown paths, unsupported methods, and unversioned paths use the standard safe envelope", async ({ request }) => {
  const cases = [
    { method: "GET", path: "/no-such-route", status: 404, code: "NOT_FOUND" },
    { method: "POST", path: "/healthz", status: 405, code: "METHOD_NOT_ALLOWED" },
    { method: "GET", path: "/ping", status: 404, code: "NOT_FOUND" },
  ] as const

  for (const item of cases) {
    const response = await request.fetch(`${apiBaseURL}${item.path}`, {
      method: item.method,
      headers: requestHeaders(),
    })
    const body = (await response.json()) as ErrorEnvelope

    expect(response.status()).toBe(item.status)
    expect(response.headers()["content-type"]).toContain("application/json")
    expect(response.headers()["x-request-id"]).toBe(body.error.request_id)
    expect(body.error).toEqual({
      code: item.code,
      message: expect.any(String),
      request_id: response.headers()["x-request-id"],
    })
  }
})

test("request identifiers are propagated when safe and replaced when unsafe", async ({ request }) => {
  const safeID = "playwright.request_id-123"
  const propagated = await request.get(`${apiBaseURL}/healthz`, {
    headers: requestHeaders(safeID),
  })
  const invalid = await request.get(`${apiBaseURL}/healthz`, {
    headers: requestHeaders("bad request id"),
  })
  const oversized = await request.get(`${apiBaseURL}/healthz`, {
    headers: requestHeaders("a".repeat(129)),
  })

  expect(propagated.headers()["x-request-id"]).toBe(safeID)
  expect(invalid.headers()["x-request-id"]).toMatch(/^[A-Za-z0-9._-]+$/)
  expect(invalid.headers()["x-request-id"]).not.toBe("bad request id")
  expect(oversized.headers()["x-request-id"]).not.toBe("a".repeat(129))
})

test("CORS allows the configured origin, rejects another origin, and terminates valid preflight", async ({ request }) => {
  const appOrigin = new URL(process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:5173").origin
  const allowed = await request.get(`${apiBaseURL}/healthz`, {
    headers: { ...requestHeaders(), Origin: appOrigin },
  })
  const rejected = await request.get(`${apiBaseURL}/healthz`, {
    headers: { ...requestHeaders(), Origin: "https://untrusted.example" },
  })
  const preflight = await request.fetch(`${apiBaseURL}/api/v1/ping`, {
    method: "OPTIONS",
    headers: {
      ...requestHeaders(),
      Origin: appOrigin,
      "Access-Control-Request-Method": "GET",
    },
  })

  expect(allowed.headers()["access-control-allow-origin"]).toBe(appOrigin)
  expect(allowed.headers()["access-control-expose-headers"]).toContain("X-Request-Id")
  expect(allowed.headers()["vary"]).toContain("Origin")
  expect(rejected.headers()["access-control-allow-origin"]).toBeUndefined()
  expect(preflight.status()).toBe(204)
  expect(preflight.headers()["access-control-allow-methods"]).toContain("GET")
  expect(preflight.headers()["access-control-allow-headers"]).toContain("X-Request-Id")
  expect(preflight.headers()["access-control-max-age"]).toBe("600")
  expect(preflight.headers()["x-ratelimit-limit"]).toBeUndefined()
})
