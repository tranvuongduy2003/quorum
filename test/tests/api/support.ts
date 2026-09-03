import { randomUUID } from "node:crypto"

export const apiBaseURL = process.env.PLAYWRIGHT_API_BASE_URL ?? "http://127.0.0.1:8080"

export const uniqueForwardedAddress = () =>
  `2001:db8:${randomUUID().replaceAll("-", "").slice(0, 24).replace(/(.{4})(?=.)/g, "$1:")}`

export const requestHeaders = (requestID?: string): Record<string, string> => ({
  "X-Forwarded-For": uniqueForwardedAddress(),
  ...(requestID === undefined ? {} : { "X-Request-Id": requestID }),
})

export interface ErrorEnvelope {
  error: {
    code: string
    message: string
    request_id: string
    details?: Array<{ field: string; code: string; message: string }>
  }
}
