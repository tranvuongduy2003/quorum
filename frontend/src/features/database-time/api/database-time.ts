import { apiGet, type RequestMeta } from "../../../shared/api/api"

export interface DatabaseTimeBody {
  database_time: string
}

export interface DatabaseTimeResult {
  body: DatabaseTimeBody
  meta: RequestMeta
  browserTime: number
}

export async function fetchDatabaseTime(): Promise<DatabaseTimeResult> {
  const result = await apiGet<DatabaseTimeBody>("/api/v1/db/time")

  return { body: result.data, meta: result.meta, browserTime: Date.now() }
}
