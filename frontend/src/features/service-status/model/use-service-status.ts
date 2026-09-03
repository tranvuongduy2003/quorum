import { useQuery } from "@tanstack/react-query"
import { isApiRequestError } from "../../../shared/api/api"
import type { StatusTone } from "../../../shared/ui/status-dot"
import {
  fetchLiveness,
  fetchReadiness,
  type DependencyStatus,
} from "../api/service-status"

const POLL_INTERVAL_MS = 5000

export interface LivenessView {
  tone: StatusTone
  text: string
  detail: string | null
}

export interface ReadinessView {
  tone: StatusTone
  text: string
  detail: string | null
  dependencies: DependencyStatus[]
  checkedAt: string | null
  requestId: string | null
}

export function dependencyTone(status: string): StatusTone {
  if (status === "up") {
    return "ok"
  }

  if (status === "down") {
    return "bad"
  }

  return "unknown"
}

function unreachableDetail(error: unknown): string {
  if (isApiRequestError(error) && error.reason === "http") {
    return `The server answered ${error.status ?? "an unexpected status"} instead.`
  }

  return "Nothing answered. The backend may not be running."
}

export function useServiceStatus(): { liveness: LivenessView; readiness: ReadinessView } {
  const liveness = useQuery({
    queryKey: ["liveness"],
    queryFn: fetchLiveness,
    refetchInterval: POLL_INTERVAL_MS,
  })

  const readiness = useQuery({
    queryKey: ["readiness"],
    queryFn: fetchReadiness,
    refetchInterval: POLL_INTERVAL_MS,
  })

  const livenessView: LivenessView = liveness.isError
    ? { tone: "bad", text: "unreachable", detail: unreachableDetail(liveness.error) }
    : liveness.data === undefined
      ? { tone: "unknown", text: "checking…", detail: null }
      : { tone: "ok", text: "alive", detail: null }

  const readinessView = ((): ReadinessView => {
    if (readiness.isError) {
      return {
        tone: "bad",
        text: "unreachable",
        detail: unreachableDetail(readiness.error),
        dependencies: [],
        checkedAt: null,
        requestId: isApiRequestError(readiness.error) ? readiness.error.requestId : null,
      }
    }

    if (readiness.data === undefined) {
      return {
        tone: "unknown",
        text: "checking…",
        detail: null,
        dependencies: [],
        checkedAt: null,
        requestId: null,
      }
    }

    const { body, meta } = readiness.data
    const ready = body.status === "ready"
    const dependencies = body.dependencies ?? []
    const down = dependencies.filter((dependency) => dependency.status !== "up")

    return {
      tone: ready ? "ok" : "warn",
      text: ready ? "ready" : "not ready",
      detail: ready
        ? null
        : down.length > 0
          ? `Not serving traffic: ${down.map((dependency) => dependency.name).join(", ")} ${down.length === 1 ? "is" : "are"} down.`
          : "Not serving traffic. The server did not name a failing dependency.",
      dependencies,
      checkedAt: body.checked_at ?? null,
      requestId: meta.requestId,
    }
  })()

  return { liveness: livenessView, readiness: readinessView }
}
