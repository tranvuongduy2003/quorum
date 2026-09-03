import { expect, test } from "@playwright/test"
import { apiBaseURL, ErrorEnvelope, requestHeaders } from "./support.js"

test("debug panic is recovered into a safe internal-error envelope", async ({ request }) => {
  const response = await request.get(`${apiBaseURL}/api/v1/debug/panic`, {
    headers: requestHeaders(),
  })
  const body = (await response.json()) as ErrorEnvelope

  expect(response.status()).toBe(500)
  expect(body.error.code).toBe("INTERNAL")
  expect(body.error.message).toBe("an unexpected error occurred")
  expect(JSON.stringify(body)).not.toMatch(/panic|stack|\.go:/i)
})

test("debug slow returns immediately at zero, applies its default, and validates its range", async ({ request }) => {
  const immediate = await request.get(`${apiBaseURL}/api/v1/debug/slow?ms=0`, {
    headers: requestHeaders(),
  })
  const defaulted = await request.get(`${apiBaseURL}/api/v1/debug/slow`, {
    headers: requestHeaders(),
  })

  expect(immediate.status()).toBe(200)
  expect(await immediate.json()).toEqual({ message: "slept" })
  expect(defaulted.status()).toBe(200)
  expect(await defaulted.json()).toEqual({ message: "slept" })

  for (const value of ["-1", "not-a-number", "30001"]) {
    const response = await request.get(`${apiBaseURL}/api/v1/debug/slow?ms=${value}`, {
      headers: requestHeaders(),
    })
    const body = (await response.json()) as ErrorEnvelope

    expect(response.status()).toBe(400)
    expect(body.error.code).toBe("VALIDATION_FAILED")
    expect(body.error.details).toEqual([
      expect.objectContaining({ field: "ms", code: "OUT_OF_RANGE" }),
    ])
  }
})

test("debug slow converts a request deadline into the timeout contract", async ({ request }) => {
  test.setTimeout(25000)

  const response = await request.get(`${apiBaseURL}/api/v1/debug/slow?ms=16000`, {
    headers: requestHeaders(),
    timeout: 22000,
  })
  const body = (await response.json()) as ErrorEnvelope

  expect(response.status()).toBe(504)
  expect(body.error.code).toBe("REQUEST_TIMEOUT")
  expect(body.error.message).toBe("the request took too long to complete")
})
