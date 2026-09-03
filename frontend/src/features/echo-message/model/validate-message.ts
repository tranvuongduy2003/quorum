import { countCharacters } from "../../../shared/lib/format"

export const MAX_MESSAGE_CHARACTERS = 280

export function validateMessage(value: string): string | null {
  const trimmedLength = countCharacters(value.trim())

  if (trimmedLength === 0) {
    return "A message is required."
  }

  if (trimmedLength > MAX_MESSAGE_CHARACTERS) {
    return `A message may be at most ${MAX_MESSAGE_CHARACTERS} characters. This one is ${trimmedLength}.`
  }

  return null
}
