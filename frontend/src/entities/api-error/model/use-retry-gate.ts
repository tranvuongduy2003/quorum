import { useCallback, useEffect, useState } from "react"
import { isApiRequestError } from "../../../shared/api/api"

export interface RetryGate {
  isBlocked: boolean
  secondsRemaining: number
  noteResult: (error: unknown) => void
  clear: () => void
}

function secondsUntil(deadline: number, now: number): number {
  return Math.max(0, Math.ceil((deadline - now) / 1000))
}

export function useRetryGate(): RetryGate {
  const [blockedUntil, setBlockedUntil] = useState<number | null>(null)
  const [secondsRemaining, setSecondsRemaining] = useState(0)

  useEffect(() => {
    if (blockedUntil === null) {
      return
    }

    const timer = setInterval(() => {
      const remaining = secondsUntil(blockedUntil, Date.now())
      setSecondsRemaining(remaining)

      if (remaining === 0) {
        setBlockedUntil(null)
      }
    }, 250)

    return () => clearInterval(timer)
  }, [blockedUntil])

  const clear = useCallback(() => {
    setBlockedUntil(null)
    setSecondsRemaining(0)
  }, [])

  const noteResult = useCallback(
    (error: unknown) => {
      if (!isApiRequestError(error) || error.envelope?.code !== "RATE_LIMITED") {
        clear()
        return
      }

      const retryAfter = Math.max(
        1,
        error.rateLimit.retryAfter ?? error.rateLimit.reset ?? 1
      )

      setBlockedUntil(Date.now() + retryAfter * 1000)
      setSecondsRemaining(retryAfter)
    },
    [clear]
  )

  return {
    isBlocked: blockedUntil !== null,
    secondsRemaining,
    noteResult,
    clear,
  }
}
