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
    // PW-F7: probe prefixed AND unprefixed keys, in both directions, so a
    // store that only separates some key shapes cannot pass.
    try {
      for (const key of ['iso-l', 'k']) {
        localStorage.setItem(key, 'a')
        expect(sessionStorage.getItem(key)).toBeNull()
        localStorage.removeItem(key)
      }
      for (const key of ['iso-s', 'k']) {
        sessionStorage.setItem(key, 'b')
        expect(localStorage.getItem(key)).toBeNull()
        sessionStorage.removeItem(key)
      }
    } finally {
      for (const key of ['iso-l', 'iso-s', 'k']) {
        localStorage.removeItem(key)
        sessionStorage.removeItem(key)
      }
    }
  })

  it('sessionStorage clear(), key() and length behave', () => {
    // PW-F8
    try {
      sessionStorage.clear()
      localStorage.setItem('keep', 'l')
      expect(sessionStorage.length).toBe(0)
      sessionStorage.setItem('only', 'v')
      expect(sessionStorage.length).toBe(1)
      expect(sessionStorage.key(0)).toBe('only')
      sessionStorage.setItem('second', 'w')
      expect(sessionStorage.length).toBe(2)
      sessionStorage.clear()
      expect(sessionStorage.length).toBe(0)
      expect(sessionStorage.key(0)).toBeNull()
      expect(sessionStorage.getItem('only')).toBeNull()
      // clearing sessionStorage must not touch localStorage
      expect(localStorage.getItem('keep')).toBe('l')
    } finally {
      sessionStorage.clear()
      localStorage.removeItem('keep')
    }
  })

  it('removeItem on one store never touches the same key in the other (both directions)', () => {
    // PW-F9
    try {
      localStorage.setItem('xs', 'L')
      sessionStorage.setItem('xs', 'S')
      localStorage.removeItem('xs')
      expect(localStorage.getItem('xs')).toBeNull()
      expect(sessionStorage.getItem('xs')).toBe('S')
      localStorage.setItem('xs', 'L')
      sessionStorage.removeItem('xs')
      expect(sessionStorage.getItem('xs')).toBeNull()
      expect(localStorage.getItem('xs')).toBe('L')
    } finally {
      localStorage.removeItem('xs')
      sessionStorage.removeItem('xs')
    }
  })

  it('key(i) enumerates exactly the stored keys as a set', () => {
    // PW-F10: order is unspecified for Web Storage, so compare as a set
    try {
      sessionStorage.clear()
      sessionStorage.setItem('ka', '1')
      sessionStorage.setItem('kb', '2')
      expect(sessionStorage.length).toBe(2)
      const keys = [sessionStorage.key(0), sessionStorage.key(1)]
      expect(new Set(keys)).toEqual(new Set(['ka', 'kb']))
      expect(sessionStorage.key(2)).toBeNull()
      // localStorage must not leak into sessionStorage enumeration
      localStorage.setItem('kc', '3')
      expect(sessionStorage.length).toBe(2)
      expect(sessionStorage.key(2)).toBeNull()
    } finally {
      sessionStorage.clear()
      localStorage.removeItem('kc')
    }
  })
})
