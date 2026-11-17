import '@testing-library/jest-dom'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Mock Tauri API
const mockInvoke = vi.fn()
const mockOpen = vi.fn()
const mockSave = vi.fn()

vi.mock('@tauri-apps/api/core', () => ({
  invoke: mockInvoke,
}))

vi.mock('@tauri-apps/plugin-dialog', () => ({
  open: mockOpen,
  save: mockSave,
}))

vi.mock('@tauri-apps/plugin-fs', () => ({
  readTextFile: vi.fn(),
  writeTextFile: vi.fn(),
}))

// Global test utilities
global.mockInvoke = mockInvoke
global.mockOpen = mockOpen
global.mockSave = mockSave

// Reset mocks and cleanup before each test
beforeEach(() => {
  vi.clearAllMocks()
})

// Cleanup after each test
afterEach(() => {
  cleanup()
})

// Node >= 25 ships an experimental native Web Storage global whose `localStorage`
// getter returns undefined unless --localstorage-file is passed. Vitest does not
// override an existing global, so it shadows jsdom's working implementation and
// `localStorage` / `sessionStorage` are undefined in tests. Install a spec-shaped
// in-memory Storage only when the global is unusable (test-environment fix; the
// production components are unchanged and run in a real webview).
function installStorageIfBroken(name: "localStorage" | "sessionStorage"): void {
  let usable = false;
  try {
    usable = typeof (globalThis as any)[name]?.getItem === "function";
  } catch {
    usable = false;
  }
  if (usable) return;
  const data = new Map<string, string>();
  const storage = {
    get length() { return data.size; },
    key: (i: number) => Array.from(data.keys())[i] ?? null,
    getItem: (k: string) => (data.has(String(k)) ? data.get(String(k))! : null),
    setItem: (k: string, v: string) => { data.set(String(k), String(v)); },
    removeItem: (k: string) => { data.delete(String(k)); },
    clear: () => { data.clear(); },
  };
  for (const target of [globalThis, (globalThis as any).window].filter(Boolean)) {
    Object.defineProperty(target, name, { value: storage, configurable: true, writable: true });
  }
}
installStorageIfBroken("localStorage");
installStorageIfBroken("sessionStorage");
