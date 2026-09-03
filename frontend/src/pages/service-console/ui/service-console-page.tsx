import { DatabaseTimePanel } from "../../../features/database-time"
import { EchoPanel } from "../../../features/echo-message"
import { FailureExplorerPanel } from "../../../features/failure-explorer"
import { PingPanel } from "../../../features/ping-probe"
import { RequestBudgetPanel } from "../../../features/request-budget"
import { ServiceStatusPanel } from "../../../features/service-status"
import { ConsoleFooter } from "./console-footer"
import { ConsoleHeader } from "./console-header"

function ServiceConsolePage() {
  return (
    <div className="relative min-h-screen overflow-x-clip">
      <ConsoleHeader />

      <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
        <div className="grid gap-6 md:grid-cols-2">
          <ServiceStatusPanel />
          <RequestBudgetPanel />
          <PingPanel />
          <DatabaseTimePanel />
          <div className="md:col-span-2">
            <EchoPanel />
          </div>
          <div className="md:col-span-2">
            <FailureExplorerPanel />
          </div>
        </div>
      </main>

      <ConsoleFooter />
    </div>
  )
}

export { ServiceConsolePage }
