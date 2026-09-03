import { expect, test } from "vitest"
import { render, screen } from "@testing-library/react"
import { Button } from "./button"

test("primary is the default variant and carries the brand surface", () => {
  render(<Button>Apply</Button>)

  const button = screen.getByRole("button", { name: "Apply" })
  expect(button.className).toContain("bg-brand-500")
  expect(button.className).toContain("hover:bg-brand-600")
  expect(button.className).toContain("active:bg-brand-700")
})

test("secondary is a translucent glass surface", () => {
  render(<Button variant="secondary">Preview</Button>)

  const button = screen.getByRole("button", { name: "Preview" })
  expect(button.className).toContain("bg-white/60")
  expect(button.className).toContain("hover:bg-white/80")
  expect(button.className).toContain("active:bg-white/90")
})

test("ghost tints on hover instead of filling", () => {
  render(<Button variant="ghost">Reset</Button>)

  const button = screen.getByRole("button", { name: "Reset" })
  expect(button.className).toContain("text-brand-500")
  expect(button.className).toContain("hover:bg-brand-50")
  expect(button.className).toContain("active:bg-brand-100")
})

test("every variant keeps the shared focus ring and disabled treatment", () => {
  render(<Button disabled>Disabled</Button>)

  const button = screen.getByRole("button", { name: "Disabled" })
  expect(button.className).toContain("focus-ring")
  expect(button.className).toContain("interactive-lift")
  expect(button.className).toContain("h-10")
  expect(button.className).toContain("disabled:cursor-not-allowed")
  expect((button as HTMLButtonElement).disabled).toBe(true)
})

test("asChild renders the child element with the button styling", () => {
  render(
    <Button asChild>
      <a href="/healthz">Health</a>
    </Button>
  )

  const link = screen.getByRole("link", { name: "Health" })
  expect(link.className).toContain("bg-brand-500")
})

test("caller classes merge instead of duplicating", () => {
  render(<Button className="w-full">Apply</Button>)

  expect(screen.getByRole("button", { name: "Apply" }).className).toContain("w-full")
})
