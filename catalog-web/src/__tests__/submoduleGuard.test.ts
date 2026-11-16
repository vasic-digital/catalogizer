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

  // The real vite.config.ts is loaded the way vite/vitest load it, against a temporary submodules layout.
  const PKG_DIRS = ['auth_context_react', 'catalogizer_api_client_ts', 'collection_manager_react', 'dashboard_analytics_react',
    'media_browser_react', 'media_player_react', 'media_types_ts', 'ui_components_react', 'websocket_client_ts']
  // vite/vitest load the config in plain node; do the same in a child process (the jsdom test environment cannot run esbuild).
  const SCRIPT = `import { loadConfigFromFile } from 'vite'
try { const r = await loadConfigFromFile({ command: 'serve', mode: 'test' }, process.argv[1]); console.log('LOADED ' + Boolean(r && r.config)) }
catch (e) { console.log('THREW ' + String(e && e.message).replace(/\\n/g, ' | ')) }`
  const loadReal = async (root: string) => {
    const env = { ...process.env, CATALOG_WEB_SUBMODULES_ROOT: root }
    const out = execFileSync(process.execPath, ['--input-type=module', '-e', SCRIPT, path.resolve(__dirname, '../../vite.config.ts')],
      { cwd: path.resolve(__dirname, '../..'), env, encoding: 'utf8' })
    return out.trim().split('\n').pop() as string
  }

  it('loading the real vite.config.ts throws, naming the package and dir, when a submodule dir is empty', async () => {
    for (const d of PKG_DIRS) { fs.mkdirSync(path.join(tmp, d)); fs.writeFileSync(path.join(tmp, d, 'package.json'), '{}') }
    fs.rmSync(path.join(tmp, 'websocket_client_ts', 'package.json'))
    expect(await loadReal(tmp)).toMatch(/^THREW .*@vasic-digital\/websocket-client.*websocket_client_ts.*empty or not a package/)
  })

  it('loading the real vite.config.ts succeeds when every submodule dir is populated', async () => {
    for (const d of PKG_DIRS) { fs.mkdirSync(path.join(tmp, d)); fs.writeFileSync(path.join(tmp, d, 'package.json'), '{}') }
    expect(await loadReal(tmp)).toBe('LOADED true')
  })
})
