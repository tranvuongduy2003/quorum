import type { ReactNode } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { EchoPanel } from "./echo-panel"

function Wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: 0 }, mutations: { retry: 0 } },
  })

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "X-Request-Id": "req-echo", ...headers },
  })
}

function typeMessage(value: string) {
  fireEvent.change(screen.getByLabelText("Message"), { target: { value } })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("EchoPanel", () => {
  it("renders the echoed message and length on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(200, { message: "hello", length: 5 }))
    )

    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("hello")
    fireEvent.click(screen.getByRole("button", { name: "Send echo" }))

    await waitFor(() => expect(screen.getByText("hello")).toBeTruthy())

    expect(screen.getByText("5")).toBeTruthy()
    expect(screen.getByText("req-echo")).toBeTruthy()
  })

  it("counts code points, not UTF-16 units", () => {
    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("🙂🙂🙂🙂🙂")

    expect(screen.getByText("5 / 280")).toBeTruthy()
  })

  it("blocks submission of an empty message before it reaches the server", () => {
    const fetchMock = vi.fn(async () => jsonResponse(200, { message: "", length: 0 }))
    vi.stubGlobal("fetch", fetchMock)

    render(<EchoPanel />, { wrapper: Wrapper })

    const submit = screen.getByRole("button", { name: "Send echo" })

    expect(submit.hasAttribute("disabled")).toBe(true)
    expect(screen.getByLabelText("Message").getAttribute("aria-invalid")).toBe("true")
    expect(screen.getByText("A message is required.")).toBeTruthy()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it("marks the input invalid when a too-long value is typed", () => {
    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("a".repeat(281))

    expect(screen.getByLabelText("Message").getAttribute("aria-invalid")).toBe("true")
    expect(screen.getByText(/at most 280 characters/i)).toBeTruthy()
    expect(screen.getByRole("button", { name: "Send echo" }).hasAttribute("disabled")).toBe(
      true
    )
  })

  it("attaches a server VALIDATION_FAILED detail to the input", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(400, {
          error: {
            code: "VALIDATION_FAILED",
            message: "the request contains one or more invalid values",
            request_id: "req-echo",
            details: [
              { field: "message", code: "TOO_LONG", message: "at most 280 characters" },
            ],
          },
        })
      )
    )

    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("hello")
    fireEvent.click(screen.getByRole("button", { name: "Send echo" }))

    await waitFor(() =>
      expect(screen.getByLabelText("Message").getAttribute("aria-invalid")).toBe("true")
    )

    expect(screen.getByText("at most 280 characters")).toBeTruthy()
    expect(
      screen.queryByText("the request contains one or more invalid values")
    ).toBeNull()
    expect(screen.getByText("req-echo")).toBeTruthy()
  })

  it("shows a panel-level treatment for a 413", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(413, {
          error: {
            code: "PAYLOAD_TOO_LARGE",
            message: "the request body is too large",
            request_id: "req-echo",
          },
        })
      )
    )

    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("hello")
    fireEvent.click(screen.getByRole("button", { name: "Send echo" }))

    await waitFor(() =>
      expect(screen.getByText("The request body was too large")).toBeTruthy()
    )

    expect(screen.getByRole("alert").textContent).toContain("64 KB")
  })

  it("disables the submit button while the request is in flight", async () => {
    let release = () => {}
    const pending = new Promise<void>((resolve) => {
      release = resolve
    })

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        await pending
        return jsonResponse(200, { message: "hello", length: 5 })
      })
    )

    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("hello")
    fireEvent.click(screen.getByRole("button", { name: "Send echo" }))

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Sending…" }).hasAttribute("disabled")).toBe(
        true
      )
    )

    release()

    await waitFor(() => expect(screen.getByRole("button", { name: "Send echo" })).toBeTruthy())
  })

  it("points out that the server trimmed whitespace before counting", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(200, { message: "hello", length: 5 }))
    )

    render(<EchoPanel />, { wrapper: Wrapper })

    typeMessage("   hello   ")
    fireEvent.click(screen.getByRole("button", { name: "Send echo" }))

    await waitFor(() => expect(screen.getByText(/Your counter read 11/)).toBeTruthy())
  })
})
