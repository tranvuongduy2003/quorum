import * as React from "react"
import { cn } from "../lib/utils"

export type SkeletonProps = React.HTMLAttributes<HTMLDivElement>

function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div aria-hidden="true" className={cn("glass-skeleton h-4 w-full", className)} {...props} />
  )
}

export { Skeleton }
