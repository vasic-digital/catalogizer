// PW-F4 variant builder. usage (cwd catalog-web): node build_variant.mjs <catalog-web-dir> <outDir> <project|project-dedupe-empty|effective-none>
//  project              = vite.config.ts as committed
//  project-dedupe-empty = resolve.dedupe=[] in the project config only (plugin-react still adds react, react-dom)
//  effective-none       = additionally strips resolve.dedupe contributed by plugins (EFFECTIVE dedupe=[])
// Prints the EFFECTIVE dedupe (configResolved) on stderr for each variant.
import path from 'node:path'; import { createRequire } from 'node:module';
const [web, out, mode] = process.argv.slice(2);
const req = createRequire(path.join(web, 'package.json'));
const vite = await import(req.resolve('vite'));
const { config } = await vite.loadConfigFromFile({ command: 'build', mode: 'production' }, path.join(web, 'vite.config.ts'), web);
if (mode !== 'project') config.resolve.dedupe = [];
if (mode === 'effective-none') {
  config.plugins = config.plugins.flat(Infinity).map(p => {
    if (!p || !p.config) return p;
    const orig = p.config;
    return { ...p, config(...a) { const r = typeof orig === 'function' ? orig.apply(this, a) : orig.handler.apply(this, a);
      return Promise.resolve(r).then(v => { if (v && v.resolve && v.resolve.dedupe) { console.error('stripped dedupe from', p.name, JSON.stringify(v.resolve.dedupe)); delete v.resolve.dedupe; } return v; }); } };
  });
}
config.plugins.push({ name: 'probe', configResolved(c) { console.error('VARIANT', mode, 'EFFECTIVE dedupe=', JSON.stringify(c.resolve.dedupe)); } });
await vite.build({ ...config, configFile: false, root: web, logLevel: 'warn', build: { ...config.build, outDir: out, emptyOutDir: true } });
