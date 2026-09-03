import * as React from "react"
import { cn } from "../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "success" | "warning" | "destructive" | "outline"
}

function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold backdrop-blur-glass-control transition-colors focus:outline-none focus-ring [&>svg]:size-3 [&>svg]:shrink-0",
        {
          "border-brand-500/25 bg-brand-500/15 text-brand-900": variant === "default",
          "border-emerald-500/30 bg-emerald-500/15 text-emerald-900": variant === "success",
          "border-amber-500/35 bg-amber-500/20 text-amber-900": variant === "warning",
          "border-red-500/30 bg-red-500/15 text-red-900": variant === "destructive",
          "border-white/70 bg-white/40 text-slate-900": variant === "outline",
        },
        className
      )}
      {...props}
    />
  )
}

export { Badge }
