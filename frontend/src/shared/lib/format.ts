export function countCharacters(value: string): number {
  return Array.from(value).length
}

export function formatLocalTime(isoTimestamp: string): string {
  const parsed = new Date(isoTimestamp)

  if (Number.isNaN(parsed.getTime())) {
    return isoTimestamp
  }

  return parsed.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  })
}

export function formatRelativeAge(isoTimestamp: string, now: number = Date.now()): string {
  const parsed = new Date(isoTimestamp)

  if (Number.isNaN(parsed.getTime())) {
    return "at an unknown time"
  }

  const seconds = Math.max(0, Math.round((now - parsed.getTime()) / 1000))

  if (seconds < 1) {
    return "just now"
  }

  if (seconds < 60) {
    return `${seconds} second${seconds === 1 ? "" : "s"} ago`
  }

  const minutes = Math.round(seconds / 60)

  if (minutes < 60) {
    return `${minutes} minute${minutes === 1 ? "" : "s"} ago`
  }

  const hours = Math.round(minutes / 60)

  return `${hours} hour${hours === 1 ? "" : "s"} ago`
}

export function formatDurationMs(durationMs: number): string {
  if (durationMs < 1000) {
    return `${Math.round(durationMs)} ms`
  }

  return `${(durationMs / 1000).toFixed(2)} s`
}

export function formatClockSkew(databaseTime: string, browserTime: number): string {
  const parsed = new Date(databaseTime)

  if (Number.isNaN(parsed.getTime())) {
    return "unknown"
  }

  const skewMs = parsed.getTime() - browserTime
  const magnitude = Math.abs(skewMs)
  const rendered = magnitude < 1000 ? `${magnitude} ms` : `${(magnitude / 1000).toFixed(1)} s`

  if (magnitude < 1000) {
    return `${rendered} (in step)`
  }

  return skewMs > 0 ? `${rendered} ahead of the browser` : `${rendered} behind the browser`
}
