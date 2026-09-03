import type { ReactNode } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { ServiceStatusPanel } from "./service-status-panel"

function Wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: 0, refetchInterval: false, gcTime: 0 } },
  })

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

interface RouteAnswer {
  status: number
  body: unknown
}

function stubRoutes(answers: Record<string, RouteAnswer | "network-failure">) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => {
      const path = new URL(input).pathname
      const answer = answers[path]

      if (answer === undefined || answer === "network-failure") {
        throw new TypeError("Failed to fetch")
      }

      return new Response(JSON.stringify(answer.body), {
        status: answer.status,
        headers: { "X-Request-Id": `req-for-${path}` },
      })
    })
  )
}

const READY_BODY = {
  status: "ready",
  dependencies: [
    { name: "postgres", status: "up" },
    { name: "redis", status: "up" },
  ],
  checked_at: "2026-08-27T18:20:31Z",
}

const NOT_READY_BODY = {
  status: "not_ready",
  dependencies: [
    { name: "postgres", status: "down" },
    { name: "redis", status: "up" },
  ],
  checked_at: "2026-08-27T18:20:31Z",
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("ServiceStatusPanel", () => {
  it("renders ready with both dependencies up", async () => {
    stubRoutes({
      "/healthz": { status: 200, body: { status: "ok" } },
      "/readyz": { status: 200, body: READY_BODY },
    })

    render(<ServiceStatusPanel />, { wrapper: Wrapper })

    await waitFor(() => expect(screen.getByText("ready")).toBeTruthy())

    expect(screen.getByText("alive")).toBeTruthy()
    expect(screen.getByText("postgres")).toBeTruthy()
    expect(screen.getByText("redis")).toBeTruthy()
    expect(screen.getAllByText("up")).toHaveLength(2)
    expect(screen.queryByText("unreachable")).toBeNull()
  })

  it("treats a 503 from readyz as data and names the failing dependency", async () => {
    stubRoutes({
      "/healthz": { status: 200, body: { status: "ok" } },
      "/readyz": { status: 503, body: NOT_READY_BODY },
    })

    render(<ServiceStatusPanel />, { wrapper: Wrapper })

    await waitFor(() => expect(screen.getByText("not ready")).toBeTruthy())

    expect(screen.queryByText("unreachable")).toBeNull()
    expect(screen.getByText("down")).toBeTruthy()
    expect(screen.getByText(/postgres is down/i)).toBeTruthy()
  })

  it("shows liveness alive and readiness not ready at the same time", async () => {
    stubRoutes({
      "/healthz": { status: 200, body: { status: "ok" } },
      "/readyz": { status: 503, body: NOT_READY_BODY },
    })

    render(<ServiceStatusPanel />, { wrapper: Wrapper })

    await waitFor(() => expect(screen.getByText("not ready")).toBeTruthy())

    expect(screen.getByText("alive")).toBeTruthy()
  })

  it("renders unreachable when fetch rejects", async () => {
    stubRoutes({})

    render(<ServiceStatusPanel />, { wrapper: Wrapper })

    await waitFor(() => expect(screen.getAllByText("unreachable")).toHaveLength(2))

    expect(screen.getAllByText(/backend may not be running/i).length).toBeGreaterThan(0)
  })

  it("says so rather than rendering an empty box when no dependency is reported", async () => {
    stubRoutes({
      "/healthz": { status: 200, body: { status: "ok" } },
      "/readyz": {
        status: 200,
        body: { ...READY_BODY, dependencies: [] },
      },
    })

    render(<ServiceStatusPanel />, { wrapper: Wrapper })

    await waitFor(() => expect(screen.getByText("ready")).toBeTruthy())

    expect(screen.getByText("No dependencies reported.")).toBeTruthy()
  })
})
