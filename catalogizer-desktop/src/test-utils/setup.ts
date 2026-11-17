/**
 * Global test setup file for Vitest
 */
import '@testing-library/jest-dom';
import { vi, afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// Mock Tauri API v2 - core module
vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
}));

// Mock Tauri plugin shell
vi.mock('@tauri-apps/plugin-shell', () => ({
  open: vi.fn(),
}));

// Clear all mocks and cleanup after each test
afterEach(() => {
  vi.clearAllMocks();
  cleanup();
});

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation(query => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// Mock ResizeObserver
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));

// Mock IntersectionObserver
global.IntersectionObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
  takeRecords: vi.fn(),
}));

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
