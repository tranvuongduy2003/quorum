import { useId, useState } from "react"
import { useMutation } from "@tanstack/react-query"
import { ApiErrorView, requestMetaFromError, useRetryGate } from "../../../entities/api-error"
import type { ApiRequestError } from "../../../shared/api/api"
import { countCharacters } from "../../../shared/lib/format"
import { cn } from "../../../shared/lib/utils"
import { Button } from "../../../shared/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../shared/ui/card"
import { RequestMetaRow } from "../../../shared/ui/request-meta"
import { Textarea } from "../../../shared/ui/textarea"
import { sendEcho, type EchoResult } from "../api/echo"
import { MAX_MESSAGE_CHARACTERS, validateMessage } from "../model/validate-message"

interface ExampleValue {
  label: string
  description: string
  value: string
}

const EXAMPLES: ExampleValue[] = [
  {
    label: "A normal message",
    description: "five characters, accepted",
    value: "hello",
  },
  {
    label: "281 characters",
    description: "one over the limit, rejected",
    value: "a".repeat(281),
  },
  {
    label: "100 Chinese characters",
    description: "300 bytes, 100 characters, accepted",
    value: "字".repeat(100),
  },
  {
    label: "Padded with spaces",
    description: "the server trims before counting",
    value: "   hello   ",
  },
]

function serverFieldMessage(error: ApiRequestError): string | null {
  if (error.envelope?.code !== "VALIDATION_FAILED") {
    return null
  }

  const detail = error.envelope.details?.find((entry) => entry.field === "message")

  return detail?.message ?? null
}

function EchoPanel() {
  const inputId = useId()
  const errorId = useId()
  const counterId = useId()
  const [value, setValue] = useState("")
  const gate = useRetryGate()

  const echo = useMutation<EchoResult, ApiRequestError, string>({
    mutationFn: sendEcho,
    onSuccess: () => gate.clear(),
    onError: (error) => gate.noteResult(error),
  })

  const clientError = validateMessage(value)
  const serverField = echo.isError ? serverFieldMessage(echo.error) : null
  const fieldError = echo.variables === value ? serverField : null
  const inlineError = clientError ?? fieldError
  const typedCharacters = countCharacters(value)
  const overLimit = countCharacters(value.trim()) > MAX_MESSAGE_CHARACTERS

  const meta = echo.isSuccess
    ? {
        status: echo.data.meta.status,
        requestId: echo.data.meta.requestId,
        durationMs: echo.data.meta.durationMs,
      }
    : requestMetaFromError(echo.error)

  const submitted = echo.variables ?? ""
  const trimmedByServer = echo.isSuccess && echo.data.body.length !== countCharacters(submitted)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Echo</CardTitle>
        <CardDescription>
          A round trip through validation. Characters are counted as code points, the way the
          server counts them.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <form
          className="space-y-2"
          onSubmit={(event) => {
            event.preventDefault()

            if (clientError !== null) {
              return
            }

            echo.mutate(value)
          }}
        >
          <div className="flex items-baseline justify-between gap-3">
            <label htmlFor={inputId} className="text-caption text-slate-700">
              Message
            </label>
            <span
              id={counterId}
              className={cn(
                "font-mono text-caption tabular-nums",
                overLimit ? "text-red-500" : "text-slate-500"
              )}
            >
              {typedCharacters} / {MAX_MESSAGE_CHARACTERS}
            </span>
          </div>

          <Textarea
            id={inputId}
            value={value}
            onChange={(event) => setValue(event.target.value)}
            aria-invalid={inlineError !== null}
            aria-describedby={inlineError === null ? counterId : `${errorId} ${counterId}`}
            placeholder="hello"
          />

          <p id={errorId} className="min-h-5 text-caption text-red-600">
            {inlineError ?? ""}
          </p>

          {value !== value.trim() && value.trim() !== "" ? (
            <p className="text-caption text-slate-600">
              The server trims surrounding whitespace before it counts, so it will see{" "}
              {countCharacters(value.trim())} characters.
            </p>
          ) : null}

          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="submit"
              disabled={clientError !== null || echo.isPending || gate.isBlocked}
              aria-busy={echo.isPending}
            >
              {echo.isPending ? "Sending…" : "Send echo"}
            </Button>
          </div>
        </form>

        <div className="flex flex-wrap gap-2">
          {EXAMPLES.map((example) => (
            <button
              key={example.label}
              type="button"
              onClick={() => setValue(example.value)}
              className="interactive-lift inline-flex min-h-10 items-center rounded-full border border-white/70 bg-white/50 px-3.5 text-caption text-slate-700 hover:bg-white/80 focus-ring"
              title={example.description}
            >
              {example.label}
            </button>
          ))}
        </div>

        <div aria-live="polite" className="space-y-3">
          {echo.isSuccess ? (
            <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3">
              <p className="text-small text-slate-900">
                The server echoed{" "}
                <code className="font-mono text-caption text-slate-900">
                  {echo.data.body.message}
                </code>{" "}
                and counted{" "}
                <span className="font-mono tabular-nums">{echo.data.body.length}</span>{" "}
                {echo.data.body.length === 1 ? "character" : "characters"}.
              </p>
              {trimmedByServer ? (
                <p className="mt-1 text-caption text-slate-600">
                  Your counter read {countCharacters(submitted)}. The difference is the
                  surrounding whitespace, which the server trims before it counts.
                </p>
              ) : null}
            </div>
          ) : null}

          {echo.isError && serverField === null ? (
            <ApiErrorView error={echo.error} retrySeconds={gate.secondsRemaining} />
          ) : null}

          {echo.isSuccess || echo.isError ? (
            <RequestMetaRow
              status={meta.status}
              requestId={meta.requestId}
              durationMs={meta.durationMs}
            />
          ) : (
            <p className="text-caption text-slate-500">Nothing sent yet.</p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export { EchoPanel }
