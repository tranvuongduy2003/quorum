import { isApiRequestError } from "../../../shared/api/api"

export interface RequestMetaTriple {
  status: number | null
  requestId: string | null
  durationMs: number | null
}

export function requestMetaFromError(error: unknown): RequestMetaTriple {
  if (!isApiRequestError(error)) {
    return { status: null, requestId: null, durationMs: null }
  }

  return {
    status: error.status,
    requestId: error.requestId,
    durationMs: error.durationMs,
  }
}
