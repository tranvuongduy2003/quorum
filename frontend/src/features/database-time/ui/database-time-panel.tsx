import { useMutation } from "@tanstack/react-query"
import { ApiErrorView, requestMetaFromError, useRetryGate } from "../../../entities/api-error"
import type { ApiRequestError } from "../../../shared/api/api"
import { formatClockSkew, formatLocalTime } from "../../../shared/lib/format"
import { Button } from "../../../shared/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../shared/ui/card"
import { RequestMetaRow } from "../../../shared/ui/request-meta"
import { fetchDatabaseTime, type DatabaseTimeResult } from "../api/database-time"

function DatabaseTimePanel() {
  const gate = useRetryGate()
  const databaseTime = useMutation<DatabaseTimeResult, ApiRequestError, void>({
    mutationFn: fetchDatabaseTime,
    onSuccess: () => gate.clear(),
    onError: (error) => gate.noteResult(error),
  })

  const meta = databaseTime.isSuccess
    ? {
        status: databaseTime.data.meta.status,
        requestId: databaseTime.data.meta.requestId,
        durationMs: databaseTime.data.meta.durationMs,
      }
    : requestMetaFromError(databaseTime.error)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Database time</CardTitle>
        <CardDescription>
          A round trip that touches PostgreSQL, so it fails when the database does.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Button
          onClick={() => databaseTime.mutate()}
          disabled={databaseTime.isPending || gate.isBlocked}
          aria-busy={databaseTime.isPending}
        >
          {databaseTime.isPending ? "Asking the database…" : "Get database time"}
        </Button>

        <div aria-live="polite" className="space-y-3">
          {databaseTime.isSuccess ? (
            <div className="space-y-1">
              <p className="text-body tabular-nums text-slate-900">
                {formatLocalTime(databaseTime.data.body.database_time)}
              </p>
              <p className="font-mono text-caption text-slate-600">
                {databaseTime.data.body.database_time}
              </p>
              <p className="text-caption text-slate-600">
                Clock skew:{" "}
                {formatClockSkew(
                  databaseTime.data.body.database_time,
                  databaseTime.data.browserTime
                )}
              </p>
            </div>
          ) : null}

          {databaseTime.isError ? (
            <ApiErrorView error={databaseTime.error} retrySeconds={gate.secondsRemaining} />
          ) : null}

          {databaseTime.isSuccess || databaseTime.isError ? (
            <RequestMetaRow
              status={meta.status}
              requestId={meta.requestId}
              durationMs={meta.durationMs}
            />
          ) : (
            <p className="text-caption text-slate-500">The database has not been asked yet.</p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export { DatabaseTimePanel }
