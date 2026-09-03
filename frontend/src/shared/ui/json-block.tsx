import { cn } from "../lib/utils"

export interface JsonBlockProps {
  summary: string
  value: unknown
  className?: string
}

function JsonBlock({ summary, value, className }: JsonBlockProps) {
  return (
    <details className={cn("group", className)}>
      <summary className="cursor-pointer list-none rounded-md px-1 py-0.5 text-caption text-brand-600 hover:text-brand-700 focus-ring">
        {summary}
      </summary>
      <pre className="glass-card mt-2 max-h-64 overflow-auto p-3 text-caption text-slate-700">
        {JSON.stringify(value, null, 2)}
      </pre>
    </details>
  )
}

export { JsonBlock }
