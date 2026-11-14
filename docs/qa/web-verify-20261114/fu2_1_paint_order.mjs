// FU2-1: live-browser paint-order check for HistoryDrawer (real component, real Tailwind CSS, real chromium layout).
// usage: node fu2_1_paint_order.mjs <repo-root> <out-dir> [--mutate]
// PASS: document.elementFromPoint(dialog centre) is inside [role=dialog]. --mutate lowers the dialog z-index below the backdrop; must FAIL (exit 1).
// Exit: 0 PASS, 1 FAIL, 2 BLIND/SKIP. Control needle: a known-present element (#fu21-probe-under) must be hit at ITS centre, and the dialog centre must NOT report a fake id.
import fs from 'node:fs'; import path from 'node:path'; import crypto from 'node:crypto';
import { createRequire } from 'node:module';
const [root, out, flag] = [process.argv[2], process.argv[3], process.argv[4]];
const mutate = flag === '--mutate';
const web = path.join(root, 'catalog-web');
const req = createRequire(path.join(web, 'package.json'));
const { createServer } = await import(req.resolve('vite'));
const { chromium } = req('playwright');
const h = path.join(web, 'src', '__fu21_harness__'); fs.mkdirSync(h, { recursive: true }); fs.mkdirSync(out, { recursive: true });
fs.writeFileSync(path.join(web, 'fu21.html'), `<!doctype html><html><head><meta charset="utf-8"><title>fu21</title></head><body><div id="root"></div><script type="module" src="/src/__fu21_harness__/main.tsx"></script></body></html>`);
fs.writeFileSync(path.join(h, 'main.tsx'), `import React from 'react'
import ReactDOM from 'react-dom/client'
import '@/index.css'
import { HistoryDrawer } from '@/components/media/HistoryDrawer'
${mutate ? "const s=document.createElement('style');s.textContent='[role=dialog]{z-index:-1 !important}';document.head.appendChild(s)" : ''}
ReactDOM.createRoot(document.getElementById('root')!).render(
  <div style={{padding:20}}><button id="fu21-probe-under" style={{position:'fixed',left:10,top:10,width:40,height:20}}>under</button>
  <HistoryDrawer mediaItemId={1} mediaTitle="Harness Title" open={true} onClose={() => {}} /></div>)`);
let server, browser, code = 2;
try {
  server = await createServer({ root: web, configFile: path.join(web, 'vite.config.ts'), server: { port: 0, host: '127.0.0.1', hmr: false }, logLevel: 'error' });
  await server.listen(); const url = server.resolvedUrls.local[0] + 'fu21.html';
  browser = await chromium.launch({ headless: true, executablePath: process.env.PW_CHROMIUM || undefined });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await page.goto(url); await page.waitForSelector('[role=dialog]', { timeout: 60000 });
  await page.waitForTimeout(500);
  const r = await page.evaluate(() => {
    const d = document.querySelector('[role=dialog]'); const b = d.getBoundingClientRect();
    const cx = b.left + b.width / 2, cy = b.top + b.height / 2;
    const hit = document.elementFromPoint(cx, cy);
    const hits = [...document.elementsFromPoint(cx, cy)].slice(0, 4).map(e => e.tagName + (e.getAttribute('data-testid') ? '[' + e.getAttribute('data-testid') + ']' : ''));
    // corner of the dialog title (always inside the dialog, never covered by content scroll)
    const t = document.getElementById('history-drawer-title').getBoundingClientRect();
    const th = document.elementFromPoint(t.left + t.width / 2, t.top + t.height / 2);
    const p = document.getElementById('fu21-probe-under').getBoundingClientRect();
    const ph = document.elementFromPoint(p.left + 5, p.top + 5);
    return { rect: [b.left, b.top, b.width, b.height].map(Math.round), centreInsideDialog: !!hit && d.contains(hit), titleHitInsideDialog: !!th && d.contains(th), hits,
      backdropHit: !!hit && hit.getAttribute('data-testid') === 'history-drawer-backdrop',
      // control: the probe sits under the backdrop+overlay; ordinary stacking says the fixed overlay covers it
      probeHitIsOverlayNotProbe: !!ph && ph.id !== 'fu21-probe-under', fakeNeedleFound: !!document.getElementById('fu21-absent-zq') };
  });
  const png = path.join(out, mutate ? 'drawer_mutated.png' : 'drawer.png');
  await page.screenshot({ path: png });
  const sha = crypto.createHash('sha256').update(fs.readFileSync(png)).digest('hex');
  const ok = r.centreInsideDialog && r.titleHitInsideDialog && !r.backdropHit && !r.fakeNeedleFound;
  console.log(JSON.stringify({ mutate, ...r, screenshotSha256: sha, verdict: ok ? 'PASS' : 'FAIL' }));
  code = ok ? 0 : 1;
} catch (e) { console.log('BLIND/SKIP: ' + String(e).split('\n')[0]); code = 2; }
finally { if (browser) await browser.close(); if (server) await server.close();
  fs.rmSync(path.join(web, 'fu21.html'), { force: true }); if (h.endsWith('__fu21_harness__') && h.startsWith(web)) fs.rmSync(h, { recursive: true, force: true }); }
process.exit(code);
