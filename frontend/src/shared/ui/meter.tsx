import { cn } from "../lib/utils"
import type { StatusTone } from "./status-dot"

export interface MeterProps {
  value: number | null
  max: number | null
  tone: StatusTone
  label: string
  valueText: string
  className?: string
}

const TONE_CLASSES: Record<StatusTone, string> = {
  ok: "bg-emerald-500",
  warn: "bg-amber-500",
  bad: "bg-red-500",
  unknown: "bg-slate-400",
}

function Meter({ value, max, tone, label, valueText, className }: MeterProps) {
  const filled =
    value === null || max === null || max <= 0
      ? 0
      : Math.min(100, Math.max(0, (value / max) * 100))

  return (
    <div
      role="meter"
      aria-label={label}
      aria-valuenow={value ?? undefined}
      aria-valuemin={0}
      aria-valuemax={max ?? undefined}
      aria-valuetext={valueText}
      className={cn(
        "h-2 w-full overflow-hidden rounded-full border border-white/60 bg-white/50",
        className
      )}
    >
      <div
        className={cn("h-full rounded-full transition-[width]", TONE_CLASSES[tone])}
        style={{ width: `${filled}%` }}
      />
    </div>
  )
}

export { Meter }
