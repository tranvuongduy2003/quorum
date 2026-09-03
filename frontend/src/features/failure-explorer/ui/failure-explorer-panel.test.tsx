import type { ReactNode } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { FailureExplorerPanel } from "./failure-explorer-panel"

function Wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: 0 }, mutations: { retry: 0 } },
  })

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

function envelopeResponse(status: number, code: string, message: string, details?: unknown) {
  return new Response(
    JSON.stringify({
      error: { code, message, request_id: `req-${code}`, details },
    }),
    { status, headers: { "X-Request-Id": `req-${code}` } }
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("FailureExplorerPanel", () => {
  it("offers every documented failure", () => {
    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    for (const label of [
      "Not found",
      "Method not allowed",
      "Invalid JSON",
      "Validation failed",
      "Payload too large",
      "Server panic",
      "Request timeout",
    ]) {
      expect(screen.getByRole("button", { name: label })).toBeTruthy()
    }
  })

  it("warns about the timeout probe before it is clicked", () => {
    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    expect(screen.getByText(/up to fifteen seconds/)).toBeTruthy()
  })

  it("shows the envelope and the raw body for a 405", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        envelopeResponse(405, "METHOD_NOT_ALLOWED", "that method is not allowed")
      )
    )

    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Method not allowed" }))

    await waitFor(() => expect(screen.getByText("METHOD_NOT_ALLOWED")).toBeTruthy())

    expect(screen.getByText("405")).toBeTruthy()
    expect(screen.getByText("Show the raw response body")).toBeTruthy()
  })

  it("explains a 404 on a debug route as disabled rather than missing", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        envelopeResponse(404, "NOT_FOUND", "the requested resource does not exist")
      )
    )

    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Server panic" }))

    await waitFor(() => expect(screen.getByText("This route is disabled")).toBeTruthy())

    expect(screen.getByRole("alert").textContent).toContain("Debug routes are turned off")
  })

  it("calls a 404 on a real route not found rather than disabled", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        envelopeResponse(404, "NOT_FOUND", "the requested resource does not exist")
      )
    )

    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Not found" }))

    await waitFor(() => expect(screen.getByText("No such route")).toBeTruthy())
  })

  it("notes that a timeout on the slow route was the expected outcome", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        envelopeResponse(504, "REQUEST_TIMEOUT", "the request exceeded the time limit")
      )
    )

    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Request timeout" }))

    await waitFor(() =>
      expect(
        screen.getByText("The request exceeded the server time limit")
      ).toBeTruthy()
    )

    expect(screen.getByRole("alert").textContent).toContain("expected outcome")
  })

  it("shows elapsed time while a probe is in flight", async () => {
    let release = () => {}
    const pending = new Promise<void>((resolve) => {
      release = resolve
    })

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        await pending
        return envelopeResponse(504, "REQUEST_TIMEOUT", "the request exceeded the time limit")
      })
    )

    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Request timeout" }))

    await waitFor(() =>
      expect(screen.getByText(/Waiting on Request timeout/)).toBeTruthy()
    )

    release()

    await waitFor(() =>
      expect(screen.getByText("The request exceeded the server time limit")).toBeTruthy()
    )
  })

  it("handles a 429 on a probe with the rate-limited treatment", async () => {
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
            { status: 429, headers: { "X-Request-Id": "req-429", "Retry-After": "3" } }
          )
      )
    )

    render(<FailureExplorerPanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Not found" }))

    await waitFor(() =>
      expect(screen.getByText("The request budget is spent")).toBeTruthy()
    )

    expect(screen.getByRole("button", { name: "Not found" }).hasAttribute("disabled")).toBe(
      true
    )
  })
})
