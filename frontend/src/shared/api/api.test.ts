import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { API_BASE_URL, ApiRequestError, apiGet, request, unwrap } from "./api"
import { isValidRequestId } from "../lib/request-id"
import { rateLimitStore } from "../lib/rate-limit-store"

interface StubResponseOptions {
  status?: number
  body?: string
  headers?: Record<string, string>
}

function stubResponse({ status = 200, body = "{}", headers = {} }: StubResponseOptions) {
  return new Response(body, { status, headers })
}

beforeEach(() => {
  rateLimitStore.reset()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("request", () => {
  it("parses a 2xx body and reports status, request id and duration", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({
          body: JSON.stringify({ message: "pong" }),
          headers: { "X-Request-Id": "abc-123" },
        })
      )
    )

    const result = await request<{ message: string }>("/api/v1/ping")

    expect(result.ok).toBe(true)
    if (!result.ok) return

    expect(result.data.message).toBe("pong")
    expect(result.meta.status).toBe(200)
    expect(result.meta.requestId).toBe("abc-123")
    expect(result.meta.durationMs).toBeGreaterThanOrEqual(0)
  })

  it("parses an error envelope on a non-2xx and keeps the code intact", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({
          status: 400,
          body: JSON.stringify({
            error: {
              code: "VALIDATION_FAILED",
              message: "the request contains one or more invalid values",
              request_id: "req-9",
              details: [{ field: "message", code: "REQUIRED", message: "required" }],
            },
          }),
          headers: { "X-Request-Id": "req-9" },
        })
      )
    )

    const result = await request("/api/v1/echo", { method: "POST", body: {} })

    expect(result.ok).toBe(false)
    if (result.ok || result.reason !== "http") return

    expect(result.error.code).toBe("VALIDATION_FAILED")
    expect(result.error.details?.[0]?.field).toBe("message")
    expect(result.meta.status).toBe(400)
  })

  it("synthesises an INTERNAL envelope when the error body is unparseable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({
          status: 503,
          body: "<html>gateway</html>",
          headers: { "X-Request-Id": "req-html" },
        })
      )
    )

    const result = await request("/api/v1/db/time")

    expect(result.ok).toBe(false)
    if (result.ok || result.reason !== "http") return

    expect(result.error.code).toBe("INTERNAL")
    expect(result.error.request_id).toBe("req-html")
    expect(result.meta.status).toBe(503)
  })

  it("synthesises an INTERNAL envelope when the error body is empty", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => stubResponse({ status: 500, body: "" }))
    )

    const result = await request("/api/v1/ping")

    expect(result.ok).toBe(false)
    if (result.ok || result.reason !== "http") return

    expect(result.error.code).toBe("INTERNAL")
  })

  it("reports a rejected fetch as a network failure, distinct from any status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch")
      })
    )

    const result = await request("/healthz")

    expect(result.ok).toBe(false)
    if (result.ok || result.reason !== "network") {
      throw new Error("expected a network failure")
    }

    expect(result.message).toContain(API_BASE_URL)
    expect(isValidRequestId(result.requestId)).toBe(true)
  })

  it("parses the rate-limit headers to numbers", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({
          headers: {
            "X-RateLimit-Limit": "60",
            "X-RateLimit-Remaining": "0",
            "X-RateLimit-Reset": "17",
          },
        })
      )
    )

    const result = await request("/api/v1/ping")

    expect(result.ok).toBe(true)
    if (!result.ok) return

    expect(result.meta.rateLimit).toMatchObject({ limit: 60, remaining: 0, reset: 17 })
    expect(result.meta.rateLimitHeadersPresent).toBe(true)
  })

  it("leaves absent rate-limit headers undefined rather than zero", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => stubResponse({})))

    const result = await request("/api/v1/ping")

    expect(result.ok).toBe(true)
    if (!result.ok) return

    expect(result.meta.rateLimit.limit).toBeUndefined()
    expect(result.meta.rateLimit.remaining).toBeUndefined()
    expect(result.meta.rateLimit.reset).toBeUndefined()
    expect(result.meta.rateLimitHeadersPresent).toBe(false)
  })

  it("publishes rate-limit observations for versioned routes only", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({
          headers: { "X-RateLimit-Limit": "5", "X-RateLimit-Remaining": "4" },
        })
      )
    )

    await request("/healthz")
    expect(rateLimitStore.getSnapshot()).toBeNull()

    await request("/api/v1/ping")
    expect(rateLimitStore.getSnapshot()?.headers.remaining).toBe(4)
  })

  it("sends an X-Request-Id the server will accept, on every request", async () => {
    const fetchMock = vi.fn(async (_input: string, _init?: RequestInit) => stubResponse({}))
    vi.stubGlobal("fetch", fetchMock)

    await request("/healthz")
    await request("/api/v1/ping")

    expect(fetchMock).toHaveBeenCalledTimes(2)

    for (const [, init] of fetchMock.mock.calls) {
      const headers = (init?.headers ?? {}) as Record<string, string>
      expect(isValidRequestId(headers["X-Request-Id"] ?? "")).toBe(true)
    }
  })

  it("treats a listed accept status as data rather than as an error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({ status: 503, body: JSON.stringify({ status: "not_ready" }) })
      )
    )

    const result = await request<{ status: string }>("/readyz", { acceptStatuses: [503] })

    expect(result.ok).toBe(true)
    if (!result.ok) return

    expect(result.data.status).toBe("not_ready")
    expect(result.meta.status).toBe(503)
  })

  it("sends a raw body untouched so a malformed payload reaches the server", async () => {
    const fetchMock = vi.fn(async (_input: string, _init?: RequestInit) => stubResponse({}))
    vi.stubGlobal("fetch", fetchMock)

    await request("/api/v1/echo", { method: "POST", rawBody: "{not json" })

    const init = fetchMock.mock.calls[0]?.[1]
    const headers = (init?.headers ?? {}) as Record<string, string>
    expect(init?.body).toBe("{not json")
    expect(headers["Content-Type"]).toBe("application/json")
  })
})

describe("unwrap", () => {
  it("throws one narrowable error type carrying the envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        stubResponse({
          status: 429,
          body: JSON.stringify({
            error: { code: "RATE_LIMITED", message: "budget spent", request_id: "req-1" },
          }),
          headers: { "X-Request-Id": "req-1", "Retry-After": "12" },
        })
      )
    )

    await expect(apiGet("/api/v1/ping")).rejects.toBeInstanceOf(ApiRequestError)

    const result = await request("/api/v1/ping")

    try {
      unwrap(result)
      throw new Error("unwrap should have thrown")
    } catch (error) {
      if (!(error instanceof ApiRequestError)) throw error
      expect(error.status).toBe(429)
      expect(error.envelope?.code).toBe("RATE_LIMITED")
      expect(error.requestId).toBe("req-1")
      expect(error.rateLimit.retryAfter).toBe(12)
    }
  })
})
