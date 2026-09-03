import { request, unwrap, type ApiSuccess } from "../../../shared/api/api"

export interface FailureProbe {
  id: string
  label: string
  expectation: string
  warning: string | null
  debugRoute: boolean
  timeoutExpected: boolean
  run: () => Promise<ApiSuccess<unknown>>
}

const OVERSIZED_MESSAGE = "a".repeat(70000)

export const FAILURE_PROBES: FailureProbe[] = [
  {
    id: "not-found",
    label: "Not found",
    expectation: "404 NOT_FOUND",
    warning: null,
    debugRoute: false,
    timeoutExpected: false,
    run: async () => unwrap(await request("/api/v1/no-such-route")),
  },
  {
    id: "method-not-allowed",
    label: "Method not allowed",
    expectation: "405 METHOD_NOT_ALLOWED",
    warning: null,
    debugRoute: false,
    timeoutExpected: false,
    run: async () => unwrap(await request("/api/v1/ping", { method: "DELETE" })),
  },
  {
    id: "invalid-json",
    label: "Invalid JSON",
    expectation: "400 INVALID_JSON",
    warning: null,
    debugRoute: false,
    timeoutExpected: false,
    run: async () =>
      unwrap(
        await request("/api/v1/echo", { method: "POST", rawBody: '{"message": ' })
      ),
  },
  {
    id: "validation-failed",
    label: "Validation failed",
    expectation: "400 VALIDATION_FAILED",
    warning: null,
    debugRoute: false,
    timeoutExpected: false,
    run: async () =>
      unwrap(await request("/api/v1/echo", { method: "POST", body: { message: "" } })),
  },
  {
    id: "payload-too-large",
    label: "Payload too large",
    expectation: "413 PAYLOAD_TOO_LARGE",
    warning: null,
    debugRoute: false,
    timeoutExpected: false,
    run: async () =>
      unwrap(
        await request("/api/v1/echo", {
          method: "POST",
          body: { message: OVERSIZED_MESSAGE },
        })
      ),
  },
  {
    id: "server-panic",
    label: "Server panic",
    expectation: "500 INTERNAL",
    warning: null,
    debugRoute: true,
    timeoutExpected: false,
    run: async () => unwrap(await request("/api/v1/debug/panic")),
  },
  {
    id: "request-timeout",
    label: "Request timeout",
    expectation: "504 REQUEST_TIMEOUT",
    warning:
      "This one waits for the server to give up, which takes up to fifteen seconds. The elapsed time is shown while it runs.",
    debugRoute: true,
    timeoutExpected: true,
    run: async () => unwrap(await request("/api/v1/debug/slow?ms=20000")),
  },
]
