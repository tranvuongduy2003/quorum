import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../shared/ui/card"
import { Meter } from "../../../shared/ui/meter"
import { StatusDot } from "../../../shared/ui/status-dot"
import { useRequestBudget } from "../model/use-request-budget"

function RequestBudgetPanel() {
  const budget = useRequestBudget()
  const hasCounts = budget.state === "known" || budget.state === "spent"

  return (
    <Card>
      <CardHeader>
        <CardTitle>Request budget</CardTitle>
        <CardDescription>
          Read from the rate-limit headers of the last versioned request any panel made. This panel
          sends no requests of its own.
        </CardDescription>
      </CardHeader>
      <CardContent aria-live="polite" className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <span className="inline-flex items-center gap-2">
            <StatusDot tone={budget.tone} />
            <span className="text-small font-medium text-slate-900">
              {budget.state === "spent"
                ? "budget spent"
                : budget.state === "known"
                  ? "within budget"
                  : budget.state === "unavailable"
                    ? "no budget reported"
                    : "not measured yet"}
            </span>
          </span>
          {hasCounts ? (
            <span className="font-mono text-small tabular-nums text-slate-900">
              {budget.remaining ?? "?"} / {budget.limit ?? "?"}
            </span>
          ) : null}
        </div>

        <Meter
          value={hasCounts ? budget.remaining : null}
          max={hasCounts ? budget.limit : null}
          tone={budget.tone}
          label="Requests remaining in the current window"
          valueText={
            hasCounts
              ? `${budget.remaining ?? 0} of ${budget.limit ?? 0} requests remaining`
              : "no budget information"
          }
        />

        <p className="text-small text-slate-700">{budget.message}</p>

        {budget.state === "spent" && budget.retrySeconds !== null ? (
          <p className="text-small font-medium text-slate-900">
            {budget.retrySeconds > 0
              ? `Retry in ${budget.retrySeconds} second${budget.retrySeconds === 1 ? "" : "s"}.`
              : "The window has reset — try again."}
          </p>
        ) : null}

        {budget.resetSeconds === null ? null : (
          <p className="text-caption text-slate-600">
            {budget.resetSeconds > 0
              ? `Window resets in ${budget.resetSeconds} second${budget.resetSeconds === 1 ? "" : "s"}.`
              : "Window reset — the next request starts a fresh one."}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

export { RequestBudgetPanel }
