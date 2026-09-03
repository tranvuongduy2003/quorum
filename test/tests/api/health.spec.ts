import { expect, test } from "@playwright/test"

const apiBaseURL = process.env.PLAYWRIGHT_API_BASE_URL ?? "http://127.0.0.1:8080"

interface ReadinessBody {
  status: string
  dependencies: Array<{ name: string; status: string }>
  checked_at: string
}

test("health endpoints report a live and ready integrated service", async ({ request }) => {
  const liveness = await request.get(`${apiBaseURL}/healthz`)

  expect(liveness.status()).toBe(200)
  expect(await liveness.json()).toEqual({ status: "ok" })
  expect(liveness.headers()["x-request-id"]).toBeTruthy()

  const readiness = await request.get(`${apiBaseURL}/readyz`)

  expect(readiness.status()).toBe(200)
  const body = (await readiness.json()) as ReadinessBody

  expect(body.status).toBe("ready")
  expect(body.dependencies).toEqual(
    expect.arrayContaining([
      { name: "postgres", status: "up" },
      { name: "redis", status: "up" },
    ])
  )
  expect(Number.isNaN(Date.parse(body.checked_at))).toBe(false)
  expect(readiness.headers()["x-request-id"]).toBeTruthy()
})
