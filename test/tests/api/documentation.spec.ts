import { expect, test } from "@playwright/test"
import { apiBaseURL } from "./support.js"

test("documentation routes publish every public operation with their delivery contracts", async ({ request }) => {
  const document = await request.get(`${apiBaseURL}/openapi.yaml`)
  const reference = await request.get(`${apiBaseURL}/docs`)
  const yaml = await document.text()
  const html = await reference.text()

  expect(document.status()).toBe(200)
  expect(document.headers()["content-type"]).toContain("application/yaml")
  expect(document.headers()["cache-control"]).toBe("no-cache")
  expect(reference.status()).toBe(200)
  expect(reference.headers()["content-type"]).toContain("text/html")
  expect(reference.headers()["cache-control"]).toBe("no-cache")
  expect(html).toContain("Scalar.createApiReference")
  expect(html).toContain("url: '/openapi.yaml'")

  for (const path of [
    "/healthz",
    "/readyz",
    "/openapi.yaml",
    "/docs",
    "/api/v1/ping",
    "/api/v1/echo",
    "/api/v1/db/time",
    "/api/v1/debug/panic",
    "/api/v1/debug/slow",
  ]) {
    expect(yaml).toContain(`  ${path}:`)
  }

  const operationIDs = [...yaml.matchAll(/^\s+operationId: (.+)$/gm)].map((match) => match[1])

  expect(operationIDs).toHaveLength(9)
  expect(new Set(operationIDs).size).toBe(operationIDs.length)
})
