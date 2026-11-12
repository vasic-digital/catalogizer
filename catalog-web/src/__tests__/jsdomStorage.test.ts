// CAT-PREEXIST-8 follow-up PW-F2: test-setup.ts installs jsdom's Web Storage
// because Node 26's native global localStorage shadows it and is unusable
// without --localstorage-file (getItem/setItem are not functions).
//
// Documented second run (also must pass):
//   NODE_OPTIONS=--localstorage-file=$(mktemp -d)/ls.db npx vitest run src/__tests__/jsdomStorage.test.ts
describe('test env Web Storage', () => {
  it('localStorage and sessionStorage are real jsdom Storage objects', () => {
    expect(window.localStorage).toBeInstanceOf(Storage)
    expect(window.sessionStorage).toBeInstanceOf(Storage)
    // PW-F5: `globalThis.localStorage toBe window.localStorage` was tautological
    // (window === globalThis under vitest jsdom) and is dropped; the two stores
    // must instead be distinct objects.
    expect(localStorage).not.toBe(sessionStorage)
  })

  it('round-trips values', () => {
    localStorage.setItem('k', 'v')
    expect(localStorage.getItem('k')).toBe('v')
    localStorage.removeItem('k')
    expect(localStorage.getItem('k')).toBeNull()
  })

  it('sessionStorage round-trips values', () => {
    sessionStorage.setItem('k', 'v')
    expect(sessionStorage.getItem('k')).toBe('v')
    sessionStorage.removeItem('k')
    expect(sessionStorage.getItem('k')).toBeNull()
  })

  it('localStorage and sessionStorage are isolated from each other', () => {
    try {
      localStorage.setItem('iso-l', 'a')
      expect(sessionStorage.getItem('iso-l')).toBeNull()
      sessionStorage.setItem('iso-s', 'b')
      expect(localStorage.getItem('iso-s')).toBeNull()
    } finally {
      localStorage.removeItem('iso-l')
      sessionStorage.removeItem('iso-s')
    }
  })
})
