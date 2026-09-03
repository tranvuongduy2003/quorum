import { apiGet, type RequestMeta } from "../../../shared/api/api"

export interface PingBody {
  message: string
}

export interface PingResult {
  body: PingBody
  meta: RequestMeta
}

export async function sendPing(): Promise<PingResult> {
  const result = await apiGet<PingBody>("/api/v1/ping")

  return { body: result.data, meta: result.meta }
}
