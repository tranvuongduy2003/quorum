const REQUEST_ID_PATTERN = /^[A-Za-z0-9._-]{1,128}$/

export function isValidRequestId(value: string): boolean {
  return REQUEST_ID_PATTERN.test(value)
}

export function newRequestId(): string {
  const source: Crypto | undefined = globalThis.crypto

  if (source && typeof source.randomUUID === "function") {
    return source.randomUUID()
  }

  const bytes = new Uint8Array(16)

  if (source && typeof source.getRandomValues === "function") {
    source.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256)
    }
  }

  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("")
}
