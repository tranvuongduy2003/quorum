export interface RateLimitHeaders {
  limit?: number
  remaining?: number
  reset?: number
  retryAfter?: number
}

export interface RateLimitObservation {
  headers: RateLimitHeaders
  headersPresent: boolean
  status: number
  observedAt: number
}

let observation: RateLimitObservation | null = null
const listeners = new Set<() => void>()

function subscribe(listener: () => void): () => void {
  listeners.add(listener)

  return () => {
    listeners.delete(listener)
  }
}

function getSnapshot(): RateLimitObservation | null {
  return observation
}

function publish(next: RateLimitObservation): void {
  observation = next

  for (const listener of listeners) {
    listener()
  }
}

function reset(): void {
  observation = null

  for (const listener of listeners) {
    listener()
  }
}

export const rateLimitStore = { subscribe, getSnapshot, publish, reset }
