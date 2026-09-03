import {
  API_BASE_URL,
  isKnownErrorCode,
  type ApiErrorDetail,
  type ApiRequestError,
} from "../../../shared/api/api"
import type { StatusTone } from "../../../shared/ui/status-dot"

export interface DescribeOptions {
  debugRoute?: boolean
  timeoutExpected?: boolean
}

export interface ApiErrorDescription {
  code: string | null
  status: number | null
  title: string
  detail: string
  hint: string | null
  tone: StatusTone
  emphasiseRequestId: boolean
  fieldDetails: ApiErrorDetail[]
}

export function describeApiError(
  error: ApiRequestError,
  options: DescribeOptions = {}
): ApiErrorDescription {
  if (error.reason === "network") {
    return {
      code: null,
      status: null,
      title: "Cannot reach the API",
      detail: `Nothing answered at ${API_BASE_URL}. Is the backend running?`,
      hint: "A browser reports a blocked cross-origin request the same way it reports a dead server. If the backend is running, check that this page was opened at http://localhost:5173, the one origin its CORS allowlist accepts.",
      tone: "bad",
      emphasiseRequestId: false,
      fieldDetails: [],
    }
  }

  const envelope = error.envelope
  const code = envelope?.code ?? "INTERNAL"
  const status = error.status
  const fieldDetails = envelope?.details ?? []
  const serverFailed = status !== null && status >= 500

  const base = {
    code,
    status,
    fieldDetails,
    tone: (serverFailed ? "bad" : "warn") as StatusTone,
    emphasiseRequestId: serverFailed,
    hint: null as string | null,
  }

  if (!isKnownErrorCode(code)) {
    return {
      ...base,
      title: `The server returned ${code}`,
      detail: envelope?.message ?? "The server did not explain the failure.",
    }
  }

  switch (code) {
    case "VALIDATION_FAILED":
      return {
        ...base,
        title: "The server rejected a value",
        detail:
          fieldDetails.length > 0
            ? "One or more fields broke a rule."
            : (envelope?.message ?? "One or more values broke a rule."),
      }
    case "INVALID_JSON":
      return {
        ...base,
        title: "The request body was not valid JSON",
        detail: "The server could not parse the body, or a field had the wrong type.",
      }
    case "PAYLOAD_TOO_LARGE":
      return {
        ...base,
        title: "The request body was too large",
        detail: "The server accepts at most 64 KB (65536 bytes) per request body.",
      }
    case "NOT_FOUND":
      return {
        ...base,
        title: options.debugRoute ? "This route is disabled" : "No such route",
        detail: options.debugRoute
          ? "Debug routes are turned off on this server, so the route answers 404 exactly as a route that never existed would."
          : "The server has no route or resource at that path.",
      }
    case "METHOD_NOT_ALLOWED":
      return {
        ...base,
        title: "That method is not allowed",
        detail: "The path exists, but it does not accept the method that was used.",
      }
    case "RATE_LIMITED":
      return {
        ...base,
        title: "The request budget is spent",
        detail: "The server is refusing further requests until the window resets.",
      }
    case "SERVICE_UNAVAILABLE":
      return {
        ...base,
        title: "A dependency is not answering",
        detail: "The server reached its limit waiting on a dependency.",
        hint: "The reason is in the server logs against this request id. The service status panel shows which dependency is down.",
        emphasiseRequestId: true,
      }
    case "REQUEST_TIMEOUT":
      return {
        ...base,
        title: "The request exceeded the server time limit",
        detail: options.timeoutExpected
          ? "This is the expected outcome: the requested sleep is longer than the server per-request deadline."
          : "The server gave up before the handler finished.",
      }
    case "INTERNAL":
      return {
        ...base,
        title: "The server failed handling this request",
        detail: envelope?.message ?? "an unexpected error occurred",
        hint: "The cause is in the server logs against this request id.",
        emphasiseRequestId: true,
      }
    default:
      return {
        ...base,
        title: `The server returned ${code}`,
        detail: envelope?.message ?? "The server did not explain the failure.",
      }
  }
}
