import { apiGet, request, unwrap, type RequestMeta } from "../../../shared/api/api"

export interface HealthBody {
  status: string
}

export interface DependencyStatus {
  name: string
  status: string
}

export interface ReadinessBody {
  status: string
  dependencies: DependencyStatus[]
  checked_at: string
}

export interface LivenessSnapshot {
  body: HealthBody
  meta: RequestMeta
}

export interface ReadinessSnapshot {
  body: ReadinessBody
  meta: RequestMeta
}

export async function fetchLiveness(): Promise<LivenessSnapshot> {
  const result = await apiGet<HealthBody>("/healthz")

  return { body: result.data, meta: result.meta }
}

export async function fetchReadiness(): Promise<ReadinessSnapshot> {
  const result = unwrap(
    await request<ReadinessBody>("/readyz", { acceptStatuses: [503] })
  )

  return { body: result.data, meta: result.meta }
}
