import { useEffect, useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { ApiErrorView, requestMetaFromError, useRetryGate } from "../../../entities/api-error"
import type { ApiRequestError, ApiSuccess } from "../../../shared/api/api"
import { formatDurationMs } from "../../../shared/lib/format"
import { Button } from "../../../shared/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../shared/ui/card"
import { JsonBlock } from "../../../shared/ui/json-block"
import { RequestMetaRow } from "../../../shared/ui/request-meta"
import { FAILURE_PROBES, type FailureProbe } from "../api/failure-probes"

function useElapsed(active: boolean, startedAt: number): number {
  const [now, setNow] = useState(startedAt)

  useEffect(() => {
    if (!active) {
      return
    }

    const timer = setInterval(() => setNow(performance.now()), 100)

    return () => clearInterval(timer)
  }, [active])

  return Math.max(0, now - startedAt)
}

function FailureExplorerPanel() {
  const gate = useRetryGate()
  const probe = useMutation<ApiSuccess<unknown>, ApiRequestError, FailureProbe>({
    mutationFn: (selected) => selected.run(),
    onSuccess: () => gate.clear(),
    onError: (error) => gate.noteResult(error),
  })

  const [startedAt, setStartedAt] = useState(0)
  const running = probe.variables ?? null
  const elapsed = useElapsed(probe.isPending, startedAt)

  const meta = probe.isSuccess
    ? {
        status: probe.data.meta.status,
        requestId: probe.data.meta.requestId,
        durationMs: probe.data.meta.durationMs,
      }
    : requestMetaFromError(probe.error)

  const rawBody = probe.isSuccess
    ? probe.data.data
    : probe.isError && probe.error.envelope !== null
      ? { error: probe.error.envelope }
      : null

  return (
    <Card>
      <CardHeader>
        <CardTitle>Failure explorer</CardTitle>
        <CardDescription>
          Every documented failure, provoked on purpose. Each one answers in the same error
          envelope.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap gap-2">
          {FAILURE_PROBES.map((candidate) => (
            <Button
              key={candidate.id}
              variant="secondary"
              onClick={() => {
                setStartedAt(performance.now())
                probe.mutate(candidate)
              }}
              disabled={probe.isPending || gate.isBlocked}
              aria-busy={probe.isPending && running?.id === candidate.id}
              title={`Expected: ${candidate.expectation}`}
            >
              {candidate.label}
            </Button>
          ))}
        </div>

        {FAILURE_PROBES.filter((candidate) => candidate.warning !== null).map((candidate) => (
          <p key={candidate.id} className="text-caption text-slate-600">
            {candidate.label}: {candidate.warning}
          </p>
        ))}

        <div aria-live="polite" className="space-y-3">
          {probe.isPending && running !== null ? (
            <p className="text-small text-slate-700">
              Waiting on {running.label} —{" "}
              <span className="font-mono tabular-nums">{formatDurationMs(elapsed)}</span>{" "}
              elapsed.
            </p>
          ) : null}

          {!probe.isPending && running !== null ? (
            <p className="text-caption text-slate-600">
              {running.label} — expected {running.expectation}.
            </p>
          ) : null}

          {probe.isSuccess ? (
            <p className="text-small text-slate-900">
              This one answered successfully rather than failing.
            </p>
          ) : null}

          {probe.isError && running !== null ? (
            <ApiErrorView
              error={probe.error}
              options={{
                debugRoute: running.debugRoute,
                timeoutExpected: running.timeoutExpected,
              }}
              retrySeconds={gate.secondsRemaining}
            />
          ) : null}

          {probe.isSuccess || probe.isError ? (
            <>
              <RequestMetaRow
                status={meta.status}
                requestId={meta.requestId}
                durationMs={meta.durationMs}
              />
              {rawBody === null ? null : (
                <JsonBlock summary="Show the raw response body" value={rawBody} />
              )}
            </>
          ) : (
            <p className="text-caption text-slate-500">
              Pick a failure to see the envelope it produces.
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export { FailureExplorerPanel }
