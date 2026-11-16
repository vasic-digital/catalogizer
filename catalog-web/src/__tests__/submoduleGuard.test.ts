import fs from 'fs'
import os from 'os'
import path from 'path'
import { execFileSync } from 'child_process'
import { assertSubmodulesPopulated } from '../../submoduleGuard'

describe('WBF1-m2 submodule guard', () => {
  let tmp: string
  beforeEach(() => { tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'subguard-')) })
  afterEach(() => { fs.rmSync(tmp, { recursive: true, force: true }) })

  it('fails loudly, naming the package and the empty dir, when a submodule is uninitialised', () => {
    fs.mkdirSync(path.join(tmp, 'media_types_ts'))
    expect(() => assertSubmodulesPopulated(tmp, { '@vasic-digital/media-types': 'media_types_ts' }))
      .toThrow(/@vasic-digital\/media-types.*media_types_ts.*empty or not a package[\s\S]*git submodule update --init/)
  })

  it('names a missing directory too', () => {
    expect(() => assertSubmodulesPopulated(tmp, { '@vasic-digital/x': 'nope' })).toThrow(/nope.*missing/)
  })

  it('passes on a populated submodule', () => {
    fs.mkdirSync(path.join(tmp, 'media_types_ts'))
    fs.writeFileSync(path.join(tmp, 'media_types_ts', 'package.json'), '{}')
    expect(() => assertSubmodulesPopulated(tmp, { '@vasic-digital/media-types': 'media_types_ts' })).not.toThrow()
  })

  it('the real vitest config uses the guard: the real submodules tree is populated here', () => {
    const out = execFileSync('grep', ['-c', 'assertSubmodulesPopulated', path.resolve(__dirname, '../../vite.config.ts')]).toString().trim()
    expect(Number(out)).toBeGreaterThanOrEqual(2)
  })
})
