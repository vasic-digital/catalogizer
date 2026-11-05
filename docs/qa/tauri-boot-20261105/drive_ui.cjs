// CAT-TAURI-1 UI driver: renders the real desktop/wizard UI (vite QA servers) in headless Chromium,
// logs in to the QA API as qa_admin (password from env ADMIN_PASSWORD, never printed), records evidence.
// usage: ADMIN_USERNAME=.. ADMIN_PASSWORD=.. node drive_ui.cjs <playwright-module-dir> <outdir> <desktopUrl> <wizardUrl>
const { chromium } = require(process.argv[2] + '/@playwright/test');
const fs = require('fs'); const path = require('path');
const [, , , out, desktop, wizard] = process.argv;
(async () => {
  const res = { desktop: {}, wizard: {} };
  const b = await chromium.launch();
  // ---- desktop
  { const p = await (await b.newContext({ viewport: { width: 1200, height: 800 } })).newPage();
    const apiCalls = []; const errs = [];
    p.on('response', r => { if (r.url().includes('/qa-api/')) apiCalls.push(r.request().method() + ' ' + new URL(r.url()).pathname + ' ' + r.status()); });
    p.on('pageerror', e => errs.push(String(e).slice(0, 200)));
    await p.goto(desktop); await p.waitForSelector('#username', { timeout: 20000 });
    await p.screenshot({ path: path.join(out, 'desktop-1-login.png') });
    res.desktop.loginRendered = true;
    await p.fill('#username', process.env.ADMIN_USERNAME); await p.fill('#password', process.env.ADMIN_PASSWORD);
    await p.click('button[type=submit]'); await p.waitForTimeout(4000);
    await p.screenshot({ path: path.join(out, 'desktop-3-after-login.png') });
    res.desktop.urlAfter = p.url(); res.desktop.bodyText = (await p.innerText('body')).slice(0, 300).replace(/\s+/g, ' ');
    res.desktop.passwordFieldStillPresent = await p.locator('#password').count();
    res.desktop.apiCalls = apiCalls; res.desktop.pageErrors = errs;
    res.desktop.ipcCalls = await p.evaluate(() => window.__QA_IPC_CALLS__); }
  // ---- wizard
  { const p = await (await b.newContext({ viewport: { width: 1000, height: 700 } })).newPage();
    const errs = []; p.on('pageerror', e => errs.push(String(e).slice(0, 200)));
    await p.goto(wizard); await p.waitForTimeout(3000);
    await p.screenshot({ path: path.join(out, 'wizard-1-first.png') });
    res.wizard.bodyText = (await p.innerText('body')).slice(0, 300).replace(/\s+/g, ' ');
    res.wizard.buttons = await p.locator('button').allInnerTexts();
    res.wizard.pageErrors = errs; res.wizard.ipcCalls = await p.evaluate(() => window.__QA_IPC_CALLS__); }
  await b.close();
  fs.writeFileSync(path.join(out, 'result.json'), JSON.stringify(res, null, 2));
  console.log(JSON.stringify(res, null, 2));
})().catch(e => { console.error('FAIL', String(e).slice(0, 400)); process.exit(1); });
