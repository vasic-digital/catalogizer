import fs from 'fs'
import path from 'path'

/**
 * WBF1-m2: the @vasic-digital/* packages are git submodules. When one is not initialised its directory exists but is
 * empty, and the test run then fails far away with an unrelated "failed to resolve import". Fail at config load instead,
 * naming the package and the empty target.
 */
export function assertSubmodulesPopulated(submodulesRoot: string, pkgs: Record<string, string>): void {
  const bad: string[] = []
  for (const [name, dir] of Object.entries(pkgs)) {
    const root = path.join(submodulesRoot, dir)
    if (!fs.existsSync(path.join(root, 'package.json'))) {
      bad.push(`  ${name} -> ${root} (${fs.existsSync(root) ? 'empty or not a package' : 'missing'})`)
    }
  }
  if (bad.length > 0) {
    throw new Error(
      `catalog-web: ${bad.length} @vasic-digital/* submodule(s) are not initialised:\n${bad.join('\n')}\n` +
        'Run: git submodule update --init (from the repository root)',
    )
  }
}
