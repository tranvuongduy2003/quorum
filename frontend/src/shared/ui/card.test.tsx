import { expect, test } from "vitest"
import { render, screen } from "@testing-library/react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./card"
import { Badge } from "./badge"
import { Skeleton } from "./skeleton"

test("a card is a first-level glass panel by default", () => {
  const { container } = render(<Card>Panel</Card>)

  expect(container.firstElementChild?.className).toContain("glass-panel")
})

test("a nested card drops to the second-level glass surface", () => {
  const { container } = render(<Card nested>Row</Card>)

  expect(container.firstElementChild?.className).toContain("glass-card")
  expect(container.firstElementChild?.className).not.toContain("glass-panel")
})

test("card titles and descriptions use the type scale", () => {
  render(
    <Card>
      <CardHeader>
        <CardTitle>Dependencies</CardTitle>
        <CardDescription>Status</CardDescription>
      </CardHeader>
      <CardContent>Body</CardContent>
    </Card>
  )

  expect(screen.getByRole("heading", { level: 3 }).className).toContain("text-h3")
  expect(screen.getByText("Status").className).toContain("text-small")
})

test("status badges use status colour, not brand blue", () => {
  render(<Badge variant="success">Healthy</Badge>)

  const badge = screen.getByText("Healthy")
  expect(badge.className).toContain("emerald")
  expect(badge.className).not.toContain("brand")
})

test("skeletons are decorative and shimmer on glass", () => {
  const { container } = render(<Skeleton className="w-1/2" />)

  const skeleton = container.firstElementChild
  expect(skeleton?.className).toContain("glass-skeleton")
  expect(skeleton?.getAttribute("aria-hidden")).toBe("true")
})

test("the type scale survives class merging", () => {
  render(<CardDescription className="text-slate-600">Merged</CardDescription>)

  const description = screen.getByText("Merged")
  expect(description.className).toContain("text-small")
  expect(description.className).toContain("text-slate-600")
  expect(description.className).not.toContain("text-slate-500")
})
