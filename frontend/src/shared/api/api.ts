import { newRequestId } from "../lib/request-id"
import { rateLimitStore, type RateLimitHeaders } from "../lib/rate-limit-store"

const configuredBaseUrl: unknown = import.meta.env.VITE_API_BASE_URL

export const API_BASE_URL: string =
  typeof configuredBaseUrl === "string" && configuredBaseUrl !== ""
    ? configuredBaseUrl
    : "http://localhost:8080"

export const ERROR_CODES = [
  "INVALID_JSON",
  "VALIDATION_FAILED",
  "NOT_FOUND",
  "METHOD_NOT_ALLOWED",
  "PAYLOAD_TOO_LARGE",
  "RATE_LIMITED",
  "SERVICE_UNAVAILABLE",
  "REQUEST_TIMEOUT",
  "INTERNAL",
] as const

export type KnownErrorCode = (typeof ERROR_CODES)[number]

export type ApiErrorCode = KnownErrorCode | (string & {})

export function isKnownErrorCode(code: string): code is KnownErrorCode {
  return (ERROR_CODES as readonly string[]).includes(code)
}

export interface ApiErrorDetail {
  field: string
  code: string
  message: string
}

export interface ApiErrorBody {
  code: ApiErrorCode
  message: string
  request_id: string
  details?: ApiErrorDetail[]
}

export interface RequestMeta {
  status: number
  requestId: string | null
  durationMs: number
  rateLimit: RateLimitHeaders
  rateLimitHeadersPresent: boolean
}

export interface ApiSuccess<TBody> {
  ok: true
  data: TBody
  meta: RequestMeta
}

export interface ApiFailure {
  ok: false
  reason: "http"
  error: ApiErrorBody
  meta: RequestMeta
}

export interface ApiUnreachable {
  ok: false
  reason: "network"
  message: string
  requestId: string
  durationMs: number
}

export type ApiResult<TBody> = ApiSuccess<TBody> | ApiFailure | ApiUnreachable

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE"
  body?: unknown
  rawBody?: string
  acceptStatuses?: number[]
  expectJson?: boolean
  signal?: AbortSignal
}

export class ApiRequestError extends Error {
  readonly reason: "http" | "network"
  readonly status: number | null
  readonly requestId: string | null
  readonly durationMs: number
  readonly envelope: ApiErrorBody | null
  readonly rateLimit: RateLimitHeaders

  constructor(failure: ApiFailure | ApiUnreachable) {
    super(failure.reason === "http" ? failure.error.message : failure.message)
    this.name = "ApiRequestError"
    this.reason = failure.reason
    this.status = failure.reason === "http" ? failure.meta.status : null
    this.requestId =
      failure.reason === "http" ? failure.meta.requestId : failure.requestId
    this.durationMs =
      failure.reason === "http" ? failure.meta.durationMs : failure.durationMs
    this.envelope = failure.reason === "http" ? failure.error : null
    this.rateLimit = failure.reason === "http" ? failure.meta.rateLimit : {}
  }
}

export function isApiRequestError(value: unknown): value is ApiRequestError {
  return value instanceof ApiRequestError
}

export function unwrap<TBody>(result: ApiResult<TBody>): ApiSuccess<TBody> {
  if (result.ok) {
    return result
  }

  throw new ApiRequestError(result)
}

function parseIntegerHeader(headers: Headers, name: string): number | undefined {
  const raw = headers.get(name)

  if (raw === null || raw.trim() === "") {
    return undefined
  }

  const parsed = Number.parseInt(raw, 10)

  return Number.isNaN(parsed) ? undefined : parsed
}

function readRateLimitHeaders(headers: Headers): RateLimitHeaders {
  return {
    limit: parseIntegerHeader(headers, "X-RateLimit-Limit"),
    remaining: parseIntegerHeader(headers, "X-RateLimit-Remaining"),
    reset: parseIntegerHeader(headers, "X-RateLimit-Reset"),
    retryAfter: parseIntegerHeader(headers, "Retry-After"),
  }
}

function hasRateLimitHeaders(headers: RateLimitHeaders): boolean {
  return headers.limit !== undefined && headers.remaining !== undefined
}

function isErrorEnvelope(value: unknown): value is { error: ApiErrorBody } {
  if (typeof value !== "object" || value === null || !("error" in value)) {
    return false
  }

  const body = (value as { error: unknown }).error

  if (typeof body !== "object" || body === null) {
    return false
  }

  const candidate = body as Record<string, unknown>

  return (
    typeof candidate.code === "string" &&
    typeof candidate.message === "string" &&
    typeof candidate.request_id === "string"
  )
}

function synthesiseEnvelope(requestId: string | null): ApiErrorBody {
  return {
    code: "INTERNAL",
    message: "the server returned a response this client could not read",
    request_id: requestId ?? "",
  }
}

async function parseJson(response: Response): Promise<unknown> {
  const text = await response.text()

  if (text.trim() === "") {
    return undefined
  }

  return JSON.parse(text) as unknown
}

function buildUrl(path: string): string {
  return `${API_BASE_URL.replace(/\/+$/, "")}${path}`
}

function isVersionedPath(path: string): boolean {
  return path.startsWith("/api/v1")
}

export async function request<TBody>(
  path: string,
  options: RequestOptions = {}
): Promise<ApiResult<TBody>> {
  const {
    method = "GET",
    body,
    rawBody,
    acceptStatuses = [],
    expectJson = true,
    signal,
  } = options
  const requestId = newRequestId()
  const headers: Record<string, string> = { "X-Request-Id": requestId }
  const payload = rawBody ?? (body === undefined ? undefined : JSON.stringify(body))

  if (payload !== undefined) {
    headers["Content-Type"] = "application/json"
  }

  const startedAt = performance.now()

  let response: Response

  try {
    response = await fetch(buildUrl(path), { method, headers, body: payload, signal })
  } catch (cause) {
    return {
      ok: false,
      reason: "network",
      message:
        cause instanceof Error && cause.name === "AbortError"
          ? "the request was cancelled"
          : `cannot reach the API at ${API_BASE_URL}`,
      requestId,
      durationMs: performance.now() - startedAt,
    }
  }

  const durationMs = performance.now() - startedAt
  const rateLimit = readRateLimitHeaders(response.headers)
  const rateLimitHeadersPresent = hasRateLimitHeaders(rateLimit)
  const meta: RequestMeta = {
    status: response.status,
    requestId: response.headers.get("X-Request-Id"),
    durationMs,
    rateLimit,
    rateLimitHeadersPresent,
  }

  if (isVersionedPath(path)) {
    rateLimitStore.publish({
      headers: rateLimit,
      headersPresent: rateLimitHeadersPresent,
      status: response.status,
      observedAt: Date.now(),
    })
  }

  if (response.ok || acceptStatuses.includes(response.status)) {
    if (!expectJson) {
      return { ok: true, data: undefined as TBody, meta }
    }

    let parsed: unknown

    try {
      parsed = await parseJson(response)
    } catch {
      return { ok: false, reason: "http", error: synthesiseEnvelope(meta.requestId), meta }
    }

    return { ok: true, data: parsed as TBody, meta }
  }

  let parsed: unknown

  try {
    parsed = await parseJson(response)
  } catch {
    parsed = undefined
  }

  if (!isErrorEnvelope(parsed)) {
    return { ok: false, reason: "http", error: synthesiseEnvelope(meta.requestId), meta }
  }

  return { ok: false, reason: "http", error: parsed.error, meta }
}

export function apiGet<TBody>(
  path: string,
  options: Omit<RequestOptions, "method" | "body" | "rawBody"> = {}
): Promise<ApiSuccess<TBody>> {
  return request<TBody>(path, { ...options, method: "GET" }).then(unwrap)
}

export function apiPost<TBody>(
  path: string,
  body: unknown,
  options: Omit<RequestOptions, "method" | "body"> = {}
): Promise<ApiSuccess<TBody>> {
  return request<TBody>(path, { ...options, method: "POST", body }).then(unwrap)
}
