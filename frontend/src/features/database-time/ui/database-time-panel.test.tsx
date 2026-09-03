import type { ReactNode } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { DatabaseTimePanel } from "./database-time-panel"

function Wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: 0 }, mutations: { retry: 0 } },
  })

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("DatabaseTimePanel", () => {
  it("shows the raw timestamp and the clock skew on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(JSON.stringify({ database_time: "2026-08-27T18:20:31Z" }), {
            status: 200,
            headers: { "X-Request-Id": "req-db" },
          })
      )
    )

    render(<DatabaseTimePanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Get database time" }))

    await waitFor(() => expect(screen.getByText("2026-08-27T18:20:31Z")).toBeTruthy())

    expect(screen.getByText(/Clock skew:/)).toBeTruthy()
    expect(screen.getByText("req-db")).toBeTruthy()
  })

  it("explains a 503 without inventing a reason and shows the request id", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({
              error: {
                code: "SERVICE_UNAVAILABLE",
                message: "a dependency is unavailable",
                request_id: "req-db-503",
              },
            }),
            { status: 503, headers: { "X-Request-Id": "req-db-503" } }
          )
      )
    )

    render(<DatabaseTimePanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Get database time" }))

    await waitFor(() =>
      expect(screen.getByText("A dependency is not answering")).toBeTruthy()
    )

    const alert = screen.getByRole("alert")
    expect(alert.textContent).toContain("server logs")
    expect(alert.textContent).toContain("service status panel")
    expect(screen.getAllByText("req-db-503").length).toBeGreaterThan(0)
  })

  it("reports an unreachable API distinctly from a 503", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("Failed to fetch")
      })
    )

    render(<DatabaseTimePanel />, { wrapper: Wrapper })

    fireEvent.click(screen.getByRole("button", { name: "Get database time" }))

    await waitFor(() => expect(screen.getByText("Cannot reach the API")).toBeTruthy())

    expect(screen.queryByText("A dependency is not answering")).toBeNull()
  })
})
