import { cn } from "../lib/utils"
import { formatDurationMs } from "../lib/format"
import { RequestId } from "./request-id"

export interface RequestMetaRowProps {
  status: number | null
  requestId: string | null
  durationMs: number | null
  className?: string
}

function statusTextClass(status: number | null): string {
  if (status === null) {
    return "text-slate-500"
  }

  if (status < 400) {
    return "text-emerald-900"
  }

  if (status < 500) {
    return "text-amber-900"
  }

  return "text-red-900"
}

function RequestMetaRow({ status, requestId, durationMs, className }: RequestMetaRowProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-3 gap-y-1 text-caption text-slate-600",
        className
      )}
    >
      <span className="inline-flex items-center gap-1">
        <span className="text-slate-500">status</span>
        <span className={cn("font-mono tabular-nums", statusTextClass(status))}>
          {status === null ? "none" : status}
        </span>
      </span>
      <span className="inline-flex items-center gap-1">
        <span className="text-slate-500">took</span>
        <span className="font-mono tabular-nums text-slate-700">
          {durationMs === null ? "—" : formatDurationMs(durationMs)}
        </span>
      </span>
      <RequestId requestId={requestId} />
    </div>
  )
}

export { RequestMetaRow }
