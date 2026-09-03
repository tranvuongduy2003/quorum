import { useEffect, useState } from "react"
import { cn } from "../../../shared/lib/utils"
import { formatLocalTime, formatRelativeAge } from "../../../shared/lib/format"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../shared/ui/card"
import { RequestId } from "../../../shared/ui/request-id"
import { StatusDot, type StatusTone } from "../../../shared/ui/status-dot"
import { dependencyTone, useServiceStatus } from "../model/use-service-status"

interface StatusLineProps {
  label: string
  tone: StatusTone
  text: string
}

function StatusLine({ label, tone, text }: StatusLineProps) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-small text-slate-700">{label}</span>
      <span className="inline-flex items-center gap-2">
        <StatusDot tone={tone} />
        <span className="text-small font-medium text-slate-900">{text}</span>
      </span>
    </div>
  )
}

function useSecondTicker(): number {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)

    return () => clearInterval(timer)
  }, [])

  return now
}

function ServiceStatusPanel() {
  const { liveness, readiness } = useServiceStatus()
  const now = useSecondTicker()

  return (
    <Card>
      <CardHeader>
        <CardTitle>Service status</CardTitle>
        <CardDescription>
          Liveness and readiness, polled every five seconds. Neither route is rate limited.
        </CardDescription>
      </CardHeader>
      <CardContent aria-live="polite" className="space-y-4">
        <div className="space-y-2">
          <StatusLine label="Liveness" tone={liveness.tone} text={liveness.text} />
          {liveness.detail === null ? null : (
            <p className="text-caption text-slate-600">{liveness.detail}</p>
          )}
        </div>

        <div className="space-y-2">
          <StatusLine label="Readiness" tone={readiness.tone} text={readiness.text} />
          {readiness.detail === null ? null : (
            <p className="text-caption text-slate-600">{readiness.detail}</p>
          )}
        </div>

        <div className="space-y-2">
          {readiness.dependencies.length === 0 ? (
            <p className="text-caption text-slate-500">
              {readiness.text === "checking…"
                ? "Waiting for the first readiness answer."
                : readiness.text === "unreachable"
                  ? "No dependency report — the readiness route did not answer."
                  : "No dependencies reported."}
            </p>
          ) : (
            readiness.dependencies.map((dependency) => {
              const tone = dependencyTone(dependency.status)

              return (
                <Card
                  key={dependency.name}
                  nested
                  className={cn(
                    "flex items-center gap-3 p-3",
                    tone === "bad" ? "border-red-500/40 bg-red-500/10" : null
                  )}
                >
                  <StatusDot tone={tone} />
                  <span className="text-small text-slate-800">{dependency.name}</span>
                  <span className="ml-auto text-small font-medium text-slate-900">
                    {dependency.status}
                  </span>
                </Card>
              )
            })
          )}
        </div>

        <div className="space-y-1 border-t border-white/60 pt-3">
          <p className="text-caption text-slate-600">
            {readiness.checkedAt === null ? (
              "Not checked yet."
            ) : (
              <>
                Checked at{" "}
                <span className="tabular-nums">{formatLocalTime(readiness.checkedAt)}</span>{" "}
                — {formatRelativeAge(readiness.checkedAt, now)}
              </>
            )}
          </p>
          <RequestId requestId={readiness.requestId} />
        </div>
      </CardContent>
    </Card>
  )
}

export { ServiceStatusPanel }
