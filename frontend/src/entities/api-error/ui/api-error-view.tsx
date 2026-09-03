import { AlertTriangle } from "lucide-react"
import type { ApiRequestError } from "../../../shared/api/api"
import { cn } from "../../../shared/lib/utils"
import { Badge } from "../../../shared/ui/badge"
import { RequestId } from "../../../shared/ui/request-id"
import { describeApiError, type DescribeOptions } from "../lib/describe-api-error"

export interface ApiErrorViewProps {
  error: ApiRequestError
  options?: DescribeOptions
  showFieldDetails?: boolean
  retrySeconds?: number
  className?: string
}

function ApiErrorView({
  error,
  options,
  showFieldDetails = true,
  retrySeconds,
  className,
}: ApiErrorViewProps) {
  const description = describeApiError(error, options)
  const rateLimited = description.code === "RATE_LIMITED"

  return (
    <div
      role="alert"
      className={cn(
        "rounded-lg border p-3",
        description.tone === "bad"
          ? "border-red-500/30 bg-red-500/10"
          : "border-amber-500/35 bg-amber-500/10",
        className
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <AlertTriangle
          aria-hidden="true"
          className={cn(
            "size-4 shrink-0",
            description.tone === "bad" ? "text-red-500" : "text-amber-500"
          )}
        />
        <span className="text-small font-semibold text-slate-900">{description.title}</span>
        {description.code === null ? null : (
          <Badge variant={description.tone === "bad" ? "destructive" : "warning"}>
            {description.code}
          </Badge>
        )}
        {description.status === null ? null : (
          <span className="font-mono text-caption tabular-nums text-slate-600">
            HTTP {description.status}
          </span>
        )}
      </div>

      <p className="mt-2 text-small text-slate-700">{description.detail}</p>

      {rateLimited && retrySeconds !== undefined ? (
        <p className="mt-2 text-small font-medium text-slate-900" aria-live="polite">
          {retrySeconds > 0
            ? `Retry in ${retrySeconds} second${retrySeconds === 1 ? "" : "s"}.`
            : "The window has reset — you can try again."}
        </p>
      ) : null}

      {showFieldDetails && description.fieldDetails.length > 0 ? (
        <ul className="mt-2 space-y-1">
          {description.fieldDetails.map((detail) => (
            <li key={`${detail.field}-${detail.code}`} className="text-small text-slate-700">
              <code className="font-mono text-caption text-slate-900">{detail.field}</code>
              <span className="text-slate-500"> — </span>
              <span>{detail.message}</span>
              <span className="text-slate-500"> ({detail.code})</span>
            </li>
          ))}
        </ul>
      ) : null}

      {description.hint === null ? null : (
        <p className="mt-2 text-caption text-slate-600">{description.hint}</p>
      )}

      <RequestId
        requestId={error.requestId}
        prominent={description.emphasiseRequestId}
        className="mt-2"
      />
    </div>
  )
}

export { ApiErrorView }
