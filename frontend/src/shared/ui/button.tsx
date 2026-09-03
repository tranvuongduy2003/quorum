import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cn } from "../lib/utils"

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean
  variant?: "primary" | "secondary" | "ghost"
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "primary", asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"
    return (
      <Comp
        ref={ref}
        className={cn(
          "interactive-lift inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium focus-ring disabled:pointer-events-none disabled:opacity-50 disabled:cursor-not-allowed [&>svg]:size-4 [&>svg]:shrink-0",
          "h-10 px-4 py-2",
          {
            "bg-brand-500 text-white shadow-sm hover:bg-brand-600 active:bg-brand-700":
              variant === "primary",
            "border border-white/70 bg-white/60 text-slate-900 backdrop-blur-glass-control hover:bg-white/80 active:bg-white/90":
              variant === "secondary",
            "text-brand-500 hover:bg-brand-50 active:bg-brand-100":
              variant === "ghost",
          },
          className
        )}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

export { Button }
