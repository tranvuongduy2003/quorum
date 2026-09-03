import { apiPost, type RequestMeta } from "../../../shared/api/api"

export interface EchoBody {
  message: string
  length: number
}

export interface EchoResult {
  body: EchoBody
  meta: RequestMeta
}

export async function sendEcho(message: string): Promise<EchoResult> {
  const result = await apiPost<EchoBody>("/api/v1/echo", { message })

  return { body: result.data, meta: result.meta }
}
