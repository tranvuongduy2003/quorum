import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { act, render, screen } from "@testing-library/react"
import { rateLimitStore } from "../../../shared/lib/rate-limit-store"
import { RequestBudgetPanel } from "./request-budget-panel"

beforeEach(() => {
  rateLimitStore.reset()
})

afterEach(() => {
  rateLimitStore.reset()
})

describe("RequestBudgetPanel", () => {
  it("asks for a request before any versioned call has been made", () => {
    render(<RequestBudgetPanel />)

    expect(screen.getByText("Make a request to see your budget.")).toBeTruthy()
    expect(screen.getByRole("meter").getAttribute("aria-valuetext")).toBe(
      "no budget information"
    )
  })

  it("renders the counts when the headers are present", () => {
    render(<RequestBudgetPanel />)

    act(() => {
      rateLimitStore.publish({
        headers: { limit: 60, remaining: 42, reset: 30 },
        headersPresent: true,
        status: 200,
        observedAt: Date.now(),
      })
    })

    expect(screen.getByText("42 / 60")).toBeTruthy()
    expect(screen.getByText("42 of 60 requests left in this window.")).toBeTruthy()
    expect(screen.getByRole("meter").getAttribute("aria-valuenow")).toBe("42")
  })

  it("reports missing headers as unavailable rather than as an empty budget", () => {
    render(<RequestBudgetPanel />)

    act(() => {
      rateLimitStore.publish({
        headers: {},
        headersPresent: false,
        status: 200,
        observedAt: Date.now(),
      })
    })

    expect(
      screen.getByText(
        "Rate limiting is disabled or unavailable, so the server reported no budget."
      )
    ).toBeTruthy()
    expect(screen.queryByText("0 / 0")).toBeNull()
    expect(screen.getByRole("meter").getAttribute("aria-valuetext")).toBe(
      "no budget information"
    )
  })

  it("renders the spent state with a retry countdown after a 429", () => {
    render(<RequestBudgetPanel />)

    act(() => {
      rateLimitStore.publish({
        headers: { limit: 5, remaining: 0, reset: 12, retryAfter: 12 },
        headersPresent: true,
        status: 429,
        observedAt: Date.now(),
      })
    })

    expect(screen.getByText("budget spent")).toBeTruthy()
    expect(screen.getByText("Retry in 12 seconds.")).toBeTruthy()
  })
})
