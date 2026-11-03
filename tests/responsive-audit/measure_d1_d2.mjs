// Targeted live measurement for responsive defects (docs/FOLLOW_UPS_NEXT_PHASE.md D1 + D2), run against the REAL built
// catalog-web and the REAL catalog-api (no mocks, constitution 11.4.289). Machine evidence = JSON on stdout / MEASURE_OUT.
//  D1: the connection-status badge ("Disconnected" while the WebSocket is down) must not intersect any other visible
//      interactive element or text box at any audited viewport, in light and dark.
//  D2: every visible interactive element meets the target-size floor: 44x44 on coarse pointers (touch viewports),
//      24x24 CSS px (WCAG 2.2 AA 2.5.8) on fine pointers.
// Exit 0 = no violation AND the badge was actually observed at every case (an unobserved badge is invalid, not clean).
// Env: AUDIT_WORK (dir with run/secrets.env), AUDIT_API_URL, AUDIT_WEB_URL. Args: --browser=chromium|firefox|webkit --pages=a,b
import { chromium, firefox, webkit } from 'playwright-core';
import fs from 'node:fs'; import path from 'node:path';
import { oracleFn } from './oracle.mjs';
const args = Object.fromEntries(process.argv.slice(2).map(a=>{const [k,v]=a.replace(/^--/,'').split('=');return [k,v??'1']}));
const W = path.resolve(process.env.AUDIT_WORK || '..');
const API = process.env.AUDIT_API_URL, WEB = process.env.AUDIT_WEB_URL;
if (!API || !WEB) { console.error('AUDIT_API_URL and AUDIT_WEB_URL are required'); process.exit(2); }
const browserName = args.browser || 'chromium'; const B = {chromium,firefox,webkit}[browserName];
const secrets = Object.fromEntries(fs.readFileSync(path.join(W,'run/secrets.env'),'utf8').trim().split('\n').map(l=>l.split(/=(.*)/s).slice(0,2)));
const PAGES = (args.pages || 'dashboard,collections,media,playlists,admin,settings').split(',');
const VPS = [[320,568,3],[375,812,3],[768,1024,2],[1024,768,1],[1440,900,1]];
const login = await (await fetch(API+'/api/v1/auth/login',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({username:secrets.ADMIN_USERNAME,password:secrets.ADMIN_PASSWORD})})).json();

// Runs in the page. Returns the badge rect and every other visible box that intersects it.
const badgeFn = () => {
  const label = Array.from(document.querySelectorAll('span')).find(s => /^(Disconnected|Connecting\.\.\.|Disconnecting\.\.\.)$/.test((s.textContent||'').trim()));
  if (!label) return { present: false };
  const chip = label.parentElement; const box = (el) => { const r = el.getBoundingClientRect(); return { x:+r.x.toFixed(2), y:+r.y.toFixed(2), w:+r.width.toFixed(2), h:+r.height.toFixed(2) }; };
  const cr = chip.getBoundingClientRect();
  const vis = (el) => { const cs = getComputedStyle(el); if (cs.display==='none'||cs.visibility==='hidden'||+cs.opacity===0) return null; const r = el.getBoundingClientRect(); return (r.width>0&&r.height>0) ? r : null; };
  const hits = []; const seen = new Set();
  const consider = (el, r, kind) => {
    if (chip.contains(el) || el.contains(chip) || el.closest('.sr-only')) return;
    const ix = Math.min(cr.right,r.right)-Math.max(cr.left,r.left), iy = Math.min(cr.bottom,r.bottom)-Math.max(cr.top,r.top);
    if (ix > 0.5 && iy > 0.5) { const k = el.tagName+kind+Math.round(r.x)+','+Math.round(r.y); if (seen.has(k)) return; seen.add(k);
      hits.push({ kind, el: el.tagName.toLowerCase()+(el.getAttribute('aria-label')?` [${el.getAttribute('aria-label')}]`:'')+` "${(el.textContent||'').trim().replace(/\s+/g,' ').slice(0,30)}"`, intersection:{ w:+ix.toFixed(2), h:+iy.toFixed(2) } }); }
  };
  for (const el of document.querySelectorAll('a[href],button,input:not([type=hidden]),select,textarea,[role=button],[role=tab],[role=menuitem]')) { const r = vis(el); if (r) consider(el, r, 'interactive'); }
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) { if (!n.nodeValue.trim()) continue; const el = n.parentElement; if (!el || ['SCRIPT','STYLE','NOSCRIPT'].includes(el.tagName) || !vis(el)) continue;
    const rg = document.createRange(); rg.selectNodeContents(n); for (const rr of rg.getClientRects()) if (rr.width>1&&rr.height>1) consider(el, rr, 'text'); }
  return { present: true, rect: box(chip), position: getComputedStyle(chip.closest('.fixed')||chip).position, vw: window.innerWidth, hits };
};

const browser = await B.launch({ args: browserName==='chromium'?['--disable-gpu']:[] });
const cases = []; let violations = 0, unobserved = 0;
for (const [w,h,dpr] of VPS) for (const theme of ['light','dark']) {
  const touch = w <= 1024;
  const ctx = await browser.newContext({ viewport:{width:w,height:h}, deviceScaleFactor:dpr, colorScheme:theme, hasTouch:touch, isMobile: touch && w<=900 && browserName!=='firefox' });
  await ctx.addInitScript(([t,u])=>{ try{localStorage.setItem('auth_token',t);localStorage.setItem('user',u);}catch{} }, [login.session_token, JSON.stringify(login.user)]);
  for (const name of PAGES) {
    const page = await ctx.newPage(); const rec = { page:name, vp:`${w}x${h}`, theme, touch, floor: touch?44:24 };
    try {
      await page.goto(WEB+'/'+name, { waitUntil:'domcontentloaded', timeout:20000 });
      await page.waitForLoadState('networkidle',{timeout:8000}).catch(()=>{});
      await page.waitForFunction(()=>!/Your media collection, organized/.test(document.body.innerText),null,{timeout:15000}).catch(()=>{});
      await page.waitForFunction(()=>Array.from(document.querySelectorAll('span')).some(s=>/^(Disconnected|Connecting\.\.\.)$/.test((s.textContent||'').trim())),null,{timeout:8000}).catch(()=>{});
      await page.waitForTimeout(800);
      const o = await page.evaluate(oracleFn, { touch, cap:50, vw:w, vh:h });
      const small = o.findings.filter(f => /^tap_target_small/.test(f.type));
      const badge = await page.evaluate(badgeFn);
      rec.badge = badge; rec.target_size_violations = small; rec.n_target_size_violations = (o.n_tap_target_small||0)+(o.n_tap_target_small_desktop||0);
      rec.badge_overlaps = badge.present ? badge.hits : null;
      if (!badge.present) { rec.status='UNOBSERVED'; unobserved++; }
      else if (badge.hits.length || rec.n_target_size_violations) { rec.status='FAIL'; violations++; } else rec.status='PASS';
      if (args.shots) { const d = path.join(args.shots, browserName); fs.mkdirSync(d,{recursive:true}); await page.screenshot({ path: path.join(d, `${name}__${w}x${h}__${theme}.png`) }); }
    } catch (e) { rec.status='ERROR'; rec.err=String(e).slice(0,200); violations++; }
    cases.push(rec); await page.close();
  }
  await ctx.close();
}
await browser.close();
const out = { browser:browserName, cases:cases.length, violations, unobserved, pass: cases.filter(c=>c.status==='PASS').length, results: cases };
if (process.env.MEASURE_OUT) fs.writeFileSync(process.env.MEASURE_OUT, JSON.stringify(out,null,1));
console.log(JSON.stringify({ browser:out.browser, cases:out.cases, pass:out.pass, violations, unobserved }));
process.exit(violations||unobserved ? 1 : 0);
