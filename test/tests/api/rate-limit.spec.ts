import { expect, test } from "@playwright/test"
import { apiBaseURL, ErrorEnvelope, requestHeaders, uniqueForwardedAddress } from "./support.js"

test("rate limiting shares a fixed budget by forwarded client identity", async ({ request }) => {
  const client = uniqueForwardedAddress()
  const headers = { "X-Forwarded-For": client }

  for (const remaining of ["2", "1", "0"]) {
    const response = await request.get(`${apiBaseURL}/api/v1/ping`, { headers })

    expect(response.status()).toBe(200)
    expect(response.headers()["x-ratelimit-limit"]).toBe("3")
    expect(response.headers()["x-ratelimit-remaining"]).toBe(remaining)
  }

  const rejected = await request.get(`${apiBaseURL}/api/v1/ping`, { headers })

  expect(rejected.status()).toBe(429)
  expect(rejected.headers()["x-ratelimit-remaining"]).toBe("0")
  expect(Number(rejected.headers()["retry-after"])).toBeGreaterThan(0)
  expect(((await rejected.json()) as ErrorEnvelope).error.code).toBe("RATE_LIMITED")
})

test("rate limiting is isolated to versioned API clients", async ({ request }) => {
  const headers = requestHeaders()

  for (let index = 0; index < 4; index += 1) {
    await request.get(`${apiBaseURL}/api/v1/ping`, { headers })
  }

  const liveness = await request.get(`${apiBaseURL}/healthz`, { headers })
  const documentation = await request.get(`${apiBaseURL}/openapi.yaml`, { headers })

  expect(liveness.status()).toBe(200)
  expect(liveness.headers()["x-ratelimit-limit"]).toBeUndefined()
  expect(documentation.status()).toBe(200)
  expect(documentation.headers()["x-ratelimit-limit"]).toBeUndefined()
})
