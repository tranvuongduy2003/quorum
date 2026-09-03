import { expect, it } from "vitest"
import { isValidRequestId, newRequestId } from "./request-id"

it("generates ids the server will accept", () => {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    expect(isValidRequestId(newRequestId())).toBe(true)
  }
})

it("rejects ids outside the documented character set", () => {
  expect(isValidRequestId("")).toBe(false)
  expect(isValidRequestId("has space")).toBe(false)
  expect(isValidRequestId("has/slash")).toBe(false)
  expect(isValidRequestId("a".repeat(129))).toBe(false)
  expect(isValidRequestId("a".repeat(128))).toBe(true)
})
