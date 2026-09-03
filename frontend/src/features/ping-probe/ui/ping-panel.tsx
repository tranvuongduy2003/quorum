import { useMutation } from "@tanstack/react-query"
import { ApiErrorView, requestMetaFromError, useRetryGate } from "../../../entities/api-error"
import type { ApiRequestError } from "../../../shared/api/api"
import { Button } from "../../../shared/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../shared/ui/card"
import { RequestMetaRow } from "../../../shared/ui/request-meta"
import { sendPing, type PingResult } from "../api/ping"

function PingPanel() {
  const gate = useRetryGate()
  const ping = useMutation<PingResult, ApiRequestError, void>({
    mutationFn: sendPing,
    onSuccess: () => gate.clear(),
    onError: (error) => gate.noteResult(error),
  })

  const meta = ping.isSuccess
    ? {
        status: ping.data.meta.status,
        requestId: ping.data.meta.requestId,
        durationMs: ping.data.meta.durationMs,
      }
    : requestMetaFromError(ping.error)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Ping</CardTitle>
        <CardDescription>
          The smallest versioned request there is. Rate limited, so it spends budget.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Button
          onClick={() => ping.mutate()}
          disabled={ping.isPending || gate.isBlocked}
          aria-busy={ping.isPending}
        >
          {ping.isPending ? "Sending ping…" : "Send ping"}
        </Button>

        <div aria-live="polite" className="space-y-3">
          {ping.isSuccess ? (
            <p className="text-small text-slate-900">
              The server answered{" "}
              <code className="font-mono text-caption text-slate-900">
                {ping.data.body.message}
              </code>
              .
            </p>
          ) : null}

          {ping.isError ? (
            <ApiErrorView error={ping.error} retrySeconds={gate.secondsRemaining} />
          ) : null}

          {ping.isSuccess || ping.isError ? (
            <RequestMetaRow
              status={meta.status}
              requestId={meta.requestId}
              durationMs={meta.durationMs}
            />
          ) : (
            <p className="text-caption text-slate-500">No ping sent yet.</p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export { PingPanel }
