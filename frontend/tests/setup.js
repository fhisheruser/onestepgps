import { vi } from 'vitest'


if (!window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

if (!global.crypto?.randomUUID) {
  global.crypto = { ...global.crypto, randomUUID: () => '00000000-0000-4000-8000-000000000000' }
}
