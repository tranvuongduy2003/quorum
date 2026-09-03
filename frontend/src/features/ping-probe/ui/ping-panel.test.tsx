import type { ReactNode } from "react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { rateLimitStore } from "../../../shared/lib/rate-limit-store"
import { PingPanel } from "./ping-panel"

function Wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: 0 }, mutations: { retry: 0 } },
  })

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

beforeEach(() => {
  rateLimitStore.reset()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("PingPanel", () => {
  it("shows the returned message and the response meta", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ message: "pong" }), {
            status: 200,
            headers: { "X-Request-Id": "req-ping" },
          })
      )
    )

    render(<PingPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Send ping" }))

    await waitFor(() => expect(screen.getByText("pong")).toBeTruthy())

    expect(screen.getByText("req-ping")).toBeTruthy()
    expect(screen.getByText("200")).toBeTruthy()
  })

  it("renders the rate-limited treatment and disables the button on a 429", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                code: "RATE_LIMITED",
                message: "the request budget is spent",
                request_id: "req-429",
              },
            }),
            {
              status: 429,
              headers: {
                "X-Request-Id": "req-429",
                "Retry-After": "7",
                "X-RateLimit-Limit": "5",
                "X-RateLimit-Remaining": "0",
              },
            }
          )
      )
    )

    render(<PingPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Send ping" }))

    await waitFor(() =>
      expect(screen.getByText("The request budget is spent")).toBeTruthy()
    )

    expect(screen.getByText("Retry in 7 seconds.")).toBeTruthy()
    expect(screen.getByRole("button", { name: "Send ping" }).hasAttribute("disabled")).toBe(
      true
    )
    expect(rateLimitStore.getSnapshot()?.status).toBe(429)
  })

  it("says the API is unreachable when fetch rejects", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch")
      })
    )

    render(<PingPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Send ping" }))

    await waitFor(() => expect(screen.getByText("Cannot reach the API")).toBeTruthy())

    expect(screen.getByRole("alert").textContent).toContain("http://localhost:8080")
  })
})
