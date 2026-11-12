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
    expect(globalThis.localStorage).toBe(window.localStorage)
  })

  it('round-trips values', () => {
    localStorage.setItem('k', 'v')
    expect(localStorage.getItem('k')).toBe('v')
    localStorage.removeItem('k')
    expect(localStorage.getItem('k')).toBeNull()
  })
})
