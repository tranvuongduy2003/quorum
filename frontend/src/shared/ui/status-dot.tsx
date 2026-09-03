import { cn } from "../lib/utils"

export type StatusTone = "ok" | "warn" | "bad" | "unknown"

export interface StatusDotProps {
  tone: StatusTone
  className?: string
}

const TONE_CLASSES: Record<StatusTone, string> = {
  ok: "bg-emerald-500",
  warn: "bg-amber-500",
  bad: "bg-red-500",
  unknown: "bg-slate-400",
}

function StatusDot({ tone, className }: StatusDotProps) {
  return (
    <span
      aria-hidden="true"
      className={cn("inline-block size-2.5 shrink-0 rounded-full", TONE_CLASSES[tone], className)}
    />
  )
}

export { StatusDot }
