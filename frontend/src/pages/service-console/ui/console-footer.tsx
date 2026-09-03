import { ExternalLink } from "lucide-react"
import { API_BASE_URL } from "../../../shared/api/api"
import { useDocsAvailability } from "../model/use-docs-availability"

function ConsoleFooter() {
  const docsAvailable = useDocsAvailability()

  return (
    <footer className="mx-auto max-w-6xl px-4 pb-10 sm:px-6">
      <div className="glass-divider flex flex-wrap items-center justify-between gap-3 border-t pt-6">
        <p className="text-caption text-slate-600">
          Quorum service foundation — configuration, dependencies, health, correlation, error
          envelope and rate limiting.
        </p>
        {docsAvailable ? (
          <a
            href={`${API_BASE_URL}/docs`}
            target="_blank"
            rel="noreferrer"
            className="interactive-lift inline-flex min-h-10 items-center gap-1.5 rounded-md px-3 text-small font-medium text-brand-500 hover:text-brand-600 focus-ring"
          >
            API reference
            <ExternalLink className="size-3.5" aria-hidden="true" />
          </a>
        ) : (
          <p className="text-caption text-slate-500">
            The API reference is not published by this server.
          </p>
        )}
      </div>
    </footer>
  )
}

export { ConsoleFooter }
