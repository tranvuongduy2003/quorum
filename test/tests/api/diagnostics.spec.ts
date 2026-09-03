import { expect, test } from "@playwright/test"
import { apiBaseURL, ErrorEnvelope, requestHeaders } from "./support.js"

test("diagnostic API contracts span routing, validation, PostgreSQL, and middleware", async ({ request }) => {
  const requestID = `playwright-diagnostics-${Date.now()}`
  const headers = requestHeaders(requestID)

  const ping = await request.get(`${apiBaseURL}/api/v1/ping`, { headers })

  expect(ping.status()).toBe(200)
  expect(await ping.json()).toEqual({ message: "pong" })
  expect(ping.headers()["x-request-id"]).toBe(requestID)
  expect(ping.headers()["x-ratelimit-limit"]).toBe("3")
  expect(ping.headers()["x-ratelimit-remaining"]).toBe("2")

  const echo = await request.post(`${apiBaseURL}/api/v1/echo`, {
    headers,
    data: { message: "  hello 👋  " },
  })

  expect(echo.status()).toBe(200)
  expect(await echo.json()).toEqual({ message: "hello 👋", length: 7 })

  const databaseTime = await request.get(`${apiBaseURL}/api/v1/db/time`, { headers })

  expect(databaseTime.status()).toBe(200)
  expect(Number.isNaN(Date.parse(((await databaseTime.json()) as { database_time: string }).database_time))).toBe(false)

  const invalidMessage = await request.post(`${apiBaseURL}/api/v1/echo`, {
    headers: requestHeaders(),
    data: { message: "" },
  })

  expect(invalidMessage.status()).toBe(400)
  const invalidBody = (await invalidMessage.json()) as ErrorEnvelope

  expect(invalidBody.error.code).toBe("VALIDATION_FAILED")
  expect(invalidBody.error.details).toEqual(
    expect.arrayContaining([expect.objectContaining({ field: "message", code: "REQUIRED" })])
  )
  expect(invalidBody.error.request_id).toBe(invalidMessage.headers()["x-request-id"])
})

test("echo accepts the rune boundary and rejects each public body failure", async ({ request }) => {
  const maximum = String.fromCodePoint(0x1f389).repeat(280)
  const accepted = await request.post(`${apiBaseURL}/api/v1/echo`, {
    headers: requestHeaders(),
    data: { message: maximum },
  })

  expect(accepted.status()).toBe(200)
  expect(await accepted.json()).toEqual({ message: maximum, length: 280 })

  for (const value of ["", "   ", "a".repeat(281)]) {
    const rejected = await request.post(`${apiBaseURL}/api/v1/echo`, {
      headers: requestHeaders(),
      data: { message: value },
    })
    const body = (await rejected.json()) as ErrorEnvelope

    expect(rejected.status()).toBe(400)
    expect(body.error.code).toBe("VALIDATION_FAILED")
    expect(body.error.details).toEqual([
      expect.objectContaining({ field: "message", code: value.length === 281 ? "TOO_LONG" : "REQUIRED" }),
    ])
  }

  const malformed = await request.post(`${apiBaseURL}/api/v1/echo`, {
    headers: { ...requestHeaders(), "Content-Type": "application/json" },
    data: '{"message":',
  })

  expect(malformed.status()).toBe(400)
  expect(((await malformed.json()) as ErrorEnvelope).error.code).toBe("INVALID_JSON")

  const oversized = await request.post(`${apiBaseURL}/api/v1/echo`, {
    headers: { ...requestHeaders(), "Content-Type": "application/json" },
    data: JSON.stringify({ message: "a".repeat(65537) }),
  })

  expect(oversized.status()).toBe(413)
  expect(((await oversized.json()) as ErrorEnvelope).error.code).toBe("PAYLOAD_TOO_LARGE")
})
