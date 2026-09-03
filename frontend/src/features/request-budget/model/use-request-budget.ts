import { useEffect, useState, useSyncExternalStore } from "react"
import { rateLimitStore } from "../../../shared/lib/rate-limit-store"
import type { StatusTone } from "../../../shared/ui/status-dot"

export type BudgetState = "idle" | "unavailable" | "known" | "spent"

export interface BudgetView {
  state: BudgetState
  limit: number | null
  remaining: number | null
  resetSeconds: number | null
  retrySeconds: number | null
  tone: StatusTone
  message: string
}

function countdown(seconds: number | undefined, observedAt: number, now: number): number | null {
  if (seconds === undefined) {
    return null
  }

  const elapsed = Math.max(0, Math.floor((now - observedAt) / 1000))

  return Math.max(0, seconds - elapsed)
}

function toneFor(remaining: number | null, limit: number | null): StatusTone {
  if (remaining === null || limit === null || limit <= 0) {
    return "unknown"
  }

  if (remaining === 0) {
    return "bad"
  }

  return remaining / limit > 0.5 ? "ok" : "warn"
}

export function useRequestBudget(): BudgetView {
  const observation = useSyncExternalStore(rateLimitStore.subscribe, rateLimitStore.getSnapshot)
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)

    return () => clearInterval(timer)
  }, [])

  if (observation === null) {
    return {
      state: "idle",
      limit: null,
      remaining: null,
      resetSeconds: null,
      retrySeconds: null,
      tone: "unknown",
      message: "Make a request to see your budget.",
    }
  }

  if (!observation.headersPresent) {
    return {
      state: "unavailable",
      limit: null,
      remaining: null,
      resetSeconds: null,
      retrySeconds: null,
      tone: "unknown",
      message: "Rate limiting is disabled or unavailable, so the server reported no budget.",
    }
  }

  const limit = observation.headers.limit ?? null
  const remaining = observation.headers.remaining ?? null
  const resetSeconds = countdown(observation.headers.reset, observation.observedAt, now)
  const retrySeconds = countdown(observation.headers.retryAfter, observation.observedAt, now)
  const spent = observation.status === 429

  return {
    state: spent ? "spent" : "known",
    limit,
    remaining,
    resetSeconds,
    retrySeconds,
    tone: spent ? "bad" : toneFor(remaining, limit),
    message: spent
      ? "The request budget is spent. The server refused the last versioned request."
      : `${remaining ?? "unknown"} of ${limit ?? "unknown"} requests left in this window.`,
  }
}
