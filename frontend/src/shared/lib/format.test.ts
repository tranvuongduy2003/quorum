import { describe, expect, it } from "vitest"
import {
  countCharacters,
  formatClockSkew,
  formatDurationMs,
  formatRelativeAge,
} from "./format"

describe("countCharacters", () => {
  it("counts code points rather than UTF-16 units", () => {
    expect(countCharacters("🙂🙂🙂🙂🙂")).toBe(5)
    expect("🙂🙂🙂🙂🙂".length).toBe(10)
  })

  it("counts multi-byte characters as one each", () => {
    expect(countCharacters("你好世界")).toBe(4)
  })
})

describe("formatRelativeAge", () => {
  const now = Date.parse("2026-08-27T18:20:31Z")

  it("renders seconds", () => {
    expect(formatRelativeAge("2026-08-27T18:20:28Z", now)).toBe("3 seconds ago")
  })

  it("renders the singular", () => {
    expect(formatRelativeAge("2026-08-27T18:20:30Z", now)).toBe("1 second ago")
  })

  it("renders minutes", () => {
    expect(formatRelativeAge("2026-08-27T18:15:31Z", now)).toBe("5 minutes ago")
  })

  it("survives an unparseable timestamp", () => {
    expect(formatRelativeAge("not a time", now)).toBe("at an unknown time")
  })
})

describe("formatDurationMs", () => {
  it("renders milliseconds below a second", () => {
    expect(formatDurationMs(12.4)).toBe("12 ms")
  })

  it("renders seconds above a second", () => {
    expect(formatDurationMs(15200)).toBe("15.20 s")
  })
})

describe("formatClockSkew", () => {
  const browserTime = Date.parse("2026-08-27T18:20:31Z")

  it("reports a database clock that runs ahead", () => {
    expect(formatClockSkew("2026-08-27T18:20:36Z", browserTime)).toBe(
      "5.0 s ahead of the browser"
    )
  })

  it("reports a database clock that runs behind", () => {
    expect(formatClockSkew("2026-08-27T18:20:26Z", browserTime)).toBe(
      "5.0 s behind the browser"
    )
  })

  it("reports agreement within a second", () => {
    expect(formatClockSkew("2026-08-27T18:20:31Z", browserTime)).toBe("0 ms (in step)")
  })
})
