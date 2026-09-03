import { useEffect, useRef, useState } from "react"
import { Check, Copy } from "lucide-react"
import { cn } from "../lib/utils"

export interface CopyButtonProps {
  value: string
  label: string
  className?: string
}

function CopyButton({ value, label, className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(
    () => () => {
      if (timer.current !== null) {
        clearTimeout(timer.current)
      }
    },
    []
  )

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
    } catch {
      setCopied(false)
      return
    }

    setCopied(true)

    if (timer.current !== null) {
      clearTimeout(timer.current)
    }

    timer.current = setTimeout(() => setCopied(false), 1500)
  }

  return (
    <button
      type="button"
      onClick={copy}
      aria-label={copied ? `${label} copied` : label}
      className={cn(
        "interactive-lift inline-flex size-10 items-center justify-center rounded-md text-slate-500 hover:bg-brand-50 hover:text-brand-600 focus-ring",
        className
      )}
    >
      {copied ? (
        <Check className="size-3.5 text-emerald-500" />
      ) : (
        <Copy className="size-3.5" />
      )}
    </button>
  )
}

export { CopyButton }
