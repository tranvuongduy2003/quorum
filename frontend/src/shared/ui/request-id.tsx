import { cn } from "../lib/utils"
import { CopyButton } from "./copy-button"

export interface RequestIdProps {
  requestId: string | null
  prominent?: boolean
  className?: string
}

function RequestId({ requestId, prominent = false, className }: RequestIdProps) {
  if (requestId === null || requestId === "") {
    return (
      <span className={cn("text-caption text-slate-500", className)}>
        request id unavailable
      </span>
    )
  }

  return (
    <span className={cn("inline-flex items-center gap-1", className)}>
      <span className="text-caption text-slate-500">request id</span>
      <code
        className={cn(
          "glass-code break-all px-1.5 py-0.5 text-caption",
          prominent ? "text-slate-900" : "text-slate-600"
        )}
      >
        {requestId}
      </code>
      <CopyButton value={requestId} label={`Copy request id ${requestId}`} />
    </span>
  )
}

export { RequestId }
