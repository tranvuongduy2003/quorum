import { Layers } from "lucide-react"
import { API_BASE_URL } from "../../../shared/api/api"

function ConsoleHeader() {
  return (
    <header className="glass-navigation">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-4 py-5 sm:px-6 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-500 text-white shadow-sm">
            <Layers className="size-5" aria-hidden="true" />
          </span>
          <div>
            <h1 className="text-h2 text-slate-900">Quorum service console</h1>
            <p className="text-small text-slate-600">
              Health, request budget and the shape of every failure, in one page.
            </p>
          </div>
        </div>
        <p className="text-caption text-slate-600">
          API base URL{" "}
          <code className="glass-code break-all px-1.5 py-0.5 text-caption">
            {API_BASE_URL}
          </code>
        </p>
      </div>
    </header>
  )
}

export { ConsoleHeader }
