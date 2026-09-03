import { describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { ApiRequestError, type ApiErrorBody } from "../../../shared/api/api"
import { ApiErrorView } from "./api-error-view"

function httpError(status: number, error: ApiErrorBody, retryAfter?: number) {
  return new ApiRequestError({
    ok: false,
    reason: "http",
    error,
    meta: {
      status,
      requestId: error.request_id,
      durationMs: 4,
      rateLimit: { retryAfter },
      rateLimitHeadersPresent: retryAfter !== undefined,
    },
  })
}

function networkError() {
  return new ApiRequestError({
    ok: false,
    reason: "network",
    message: "cannot reach the API",
    requestId: "client-generated-id",
    durationMs: 2,
  })
}

describe("ApiErrorView", () => {
  it("names the base URL when the API is unreachable", () => {
    render(<ApiErrorView error={networkError()} />)

    expect(screen.getByRole("alert").textContent).toContain("Cannot reach the API")
    expect(screen.getByRole("alert").textContent).toContain("http://localhost:8080")
  })

  it("renders validation details field by field", () => {
    render(
      <ApiErrorView
        error={httpError(400, {
          code: "VALIDATION_FAILED",
          message: "the request contains one or more invalid values",
          request_id: "req-validation",
          details: [
            { field: "message", code: "TOO_LONG", message: "at most 280 characters" },
          ],
        })}
      />
    )

    expect(screen.getByText("message")).toBeTruthy()
    expect(screen.getByText("at most 280 characters")).toBeTruthy()
  })

  it("explains a 404 on a debug route as disabled rather than missing", () => {
    render(
      <ApiErrorView
        error={httpError(404, {
          code: "NOT_FOUND",
          message: "the requested resource does not exist",
          request_id: "req-404",
        })}
        options={{ debugRoute: true }}
      />
    )

    expect(screen.getByRole("alert").textContent).toContain("This route is disabled")
    expect(screen.getByRole("alert").textContent).toContain("Debug routes are turned off")
  })

  it("points a 503 at the server logs and the request id", () => {
    render(
      <ApiErrorView
        error={httpError(503, {
          code: "SERVICE_UNAVAILABLE",
          message: "a dependency is unavailable",
          request_id: "req-503",
        })}
      />
    )

    const alert = screen.getByRole("alert")
    expect(alert.textContent).toContain("A dependency is not answering")
    expect(alert.textContent).toContain("server logs")
    expect(screen.getByText("req-503")).toBeTruthy()
  })

  it("does not speculate about the cause of a 500", () => {
    render(
      <ApiErrorView
        error={httpError(500, {
          code: "INTERNAL",
          message: "an unexpected error occurred",
          request_id: "req-500",
        })}
      />
    )

    expect(screen.getByRole("alert").textContent).toContain("an unexpected error occurred")
    expect(screen.getByText("req-500")).toBeTruthy()
  })

  it("counts down the retry window on a 429", () => {
    render(
      <ApiErrorView
        error={httpError(
          429,
          { code: "RATE_LIMITED", message: "budget spent", request_id: "req-429" },
          9
        )}
        retrySeconds={9}
      />
    )

    expect(screen.getByRole("alert").textContent).toContain("The request budget is spent")
    expect(screen.getByText("Retry in 9 seconds.")).toBeTruthy()
  })

  it("renders an unrecognised code without crashing", () => {
    render(
      <ApiErrorView
        error={httpError(418, {
          code: "TEAPOT_BREWING",
          message: "the server is a teapot",
          request_id: "req-418",
        })}
      />
    )

    const alert = screen.getByRole("alert")
    expect(alert.textContent).toContain("TEAPOT_BREWING")
    expect(alert.textContent).toContain("the server is a teapot")
    expect(alert.textContent).toContain("HTTP 418")
  })

  it("always renders a copyable request id", () => {
    render(
      <ApiErrorView
        error={httpError(405, {
          code: "METHOD_NOT_ALLOWED",
          message: "that method is not allowed",
          request_id: "req-405",
        })}
      />
    )

    expect(screen.getByRole("button", { name: "Copy request id req-405" })).toBeTruthy()
  })
})
