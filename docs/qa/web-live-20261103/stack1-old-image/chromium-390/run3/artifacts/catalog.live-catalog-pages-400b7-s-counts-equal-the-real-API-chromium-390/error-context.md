# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: catalog.live.spec.ts >> catalog pages over the seeded corpus (real backend) >> Media page lists the seeded items: counts equal the real API
- Location: e2e/live/catalog.live.spec.ts:58:7

# Error details

```
Test timeout of 120000ms exceeded.
```

```
Error: locator.click: Test timeout of 120000ms exceeded.
Call log:
  - waiting for locator('a:has-text("Media")').filter({ visible: true }).first()

```

# Page snapshot

```yaml
- generic [ref=e2]:
  - generic [ref=e4]:
    - img [ref=e5]
    - generic [ref=e12]: Disconnected
  - generic [ref=e13]:
    - banner [ref=e14]:
      - generic [ref=e16]:
        - link "C Catalogizer" [ref=e17] [cursor=pointer]:
          - /url: /
          - generic [ref=e19]: C
          - generic [ref=e20]: Catalogizer
        - button [ref=e21] [cursor=pointer]:
          - img [ref=e22]
    - main [ref=e23]:
      - generic [ref=e24]:
        - generic [ref=e25]:
          - generic [ref=e26]:
            - heading "Welcome back, admin!" [level=1] [ref=e27]
            - paragraph [ref=e28]: Here's what's happening with your media library today.
          - 'button "Last updated: 12:35:52 AM" [ref=e30] [cursor=pointer]':
            - img [ref=e31]
            - text: "Last updated: 12:35:52 AM"
        - generic [ref=e35]:
          - generic [ref=e36]:
            - generic [ref=e37]:
              - heading "Total Media" [level=3] [ref=e38]
              - img [ref=e39]
            - generic [ref=e43]:
              - generic [ref=e44]: "39"
              - generic [ref=e45]:
                - generic [ref=e46]:
                  - img [ref=e47]
                  - generic [ref=e50]: 12.5%
                - generic [ref=e51]: vs last month
            - img [ref=e53]
          - generic [ref=e57]:
            - generic [ref=e58]:
              - heading "Storage Used" [level=3] [ref=e59]
              - img [ref=e60]
            - generic [ref=e62]:
              - generic [ref=e63]: 2.31 MB
              - generic [ref=e64]:
                - generic [ref=e65]:
                  - img [ref=e66]
                  - generic [ref=e69]: 8.2%
                - generic [ref=e70]: vs last month
            - img [ref=e72]
          - generic [ref=e74]:
            - generic [ref=e75]:
              - heading "Recent Additions" [level=3] [ref=e76]
              - img [ref=e77]
            - generic [ref=e79]:
              - generic [ref=e80]: "39"
              - generic [ref=e81]:
                - generic [ref=e82]:
                  - img [ref=e83]
                  - generic [ref=e86]: 15.3%
                - generic [ref=e87]: last 7 days
            - img [ref=e89]
          - generic [ref=e91]:
            - generic [ref=e92]:
              - heading "Quality Score" [level=3] [ref=e93]
              - img [ref=e94]
            - generic [ref=e96]:
              - generic [ref=e97]: HD
              - generic [ref=e98]:
                - generic [ref=e99]:
                  - img [ref=e100]
                  - generic [ref=e103]: 5.7%
                - generic [ref=e104]: average quality
            - img [ref=e106]
          - generic [ref=e108]:
            - generic [ref=e109]:
              - heading "Active Users" [level=3] [ref=e110]
              - img [ref=e111]
            - generic [ref=e116]:
              - generic [ref=e117]: "3"
              - generic [ref=e118]:
                - generic [ref=e119]:
                  - img [ref=e120]
                  - generic [ref=e123]: 3.2%
                - generic [ref=e124]: online now
            - img [ref=e126]
          - generic [ref=e131]:
            - generic [ref=e132]:
              - heading "Total Users" [level=3] [ref=e133]
              - img [ref=e134]
            - generic [ref=e139]:
              - generic [ref=e140]: "12"
              - generic [ref=e141]:
                - generic [ref=e142]:
                  - img [ref=e143]
                  - generic [ref=e146]: 18.7%
                - generic [ref=e147]: registered users
            - img [ref=e149]
          - generic [ref=e154]:
            - generic [ref=e155]:
              - heading "Sessions Today" [level=3] [ref=e156]
              - img [ref=e157]
            - generic [ref=e160]:
              - generic [ref=e161]: "24"
              - generic [ref=e162]:
                - generic [ref=e163]:
                  - img [ref=e164]
                  - generic [ref=e167]: 22.1%
                - generic [ref=e168]: viewing sessions
            - img [ref=e170]
          - generic [ref=e173]:
            - generic [ref=e174]:
              - heading "Avg Session" [level=3] [ref=e175]
              - img [ref=e176]
            - generic [ref=e179]:
              - generic [ref=e180]: 45m
              - generic [ref=e181]:
                - generic [ref=e182]:
                  - img [ref=e183]
                  - generic [ref=e186]: 7.5%
                - generic [ref=e187]: viewing duration
            - img [ref=e189]
        - generic [ref=e193]:
          - heading "Media Entities (18 total)" [level=3] [ref=e195]:
            - img [ref=e196]
            - text: Media Entities (18 total)
          - generic [ref=e199]:
            - generic [ref=e200]:
              - img [ref=e201]
              - generic [ref=e203]:
                - generic [ref=e204]: "7"
                - generic [ref=e205]: movie
            - generic [ref=e206]:
              - img [ref=e207]
              - generic [ref=e211]:
                - generic [ref=e212]: "4"
                - generic [ref=e213]: music album
            - generic [ref=e214]:
              - img [ref=e215]
              - generic [ref=e218]:
                - generic [ref=e219]: "3"
                - generic [ref=e220]: book
            - generic [ref=e221]:
              - img [ref=e222]
              - generic [ref=e224]:
                - generic [ref=e225]: "1"
                - generic [ref=e226]: comic
            - generic [ref=e227]:
              - img [ref=e228]
              - generic [ref=e230]:
                - generic [ref=e231]: "1"
                - generic [ref=e232]: tv episode
            - generic [ref=e233]:
              - img [ref=e234]
              - generic [ref=e236]:
                - generic [ref=e237]: "1"
                - generic [ref=e238]: tv season
            - generic [ref=e239]:
              - img [ref=e240]
              - generic [ref=e243]:
                - generic [ref=e244]: "1"
                - generic [ref=e245]: tv show
        - generic [ref=e247]:
          - heading "Storage Statistics" [level=3] [ref=e249]:
            - img [ref=e250]
            - text: Storage Statistics
          - generic [ref=e253]:
            - generic [ref=e254]:
              - img [ref=e255]
              - generic [ref=e260]: "13"
              - generic [ref=e261]: Directories
            - generic [ref=e262]:
              - img [ref=e263]
              - generic [ref=e267]: "39"
              - generic [ref=e268]: Total Files
            - generic [ref=e269]:
              - img [ref=e270]
              - generic [ref=e272]: 0.0 GB
              - generic [ref=e273]: Total Size
            - generic [ref=e274]:
              - img [ref=e275]
              - generic [ref=e278]: "0"
              - generic [ref=e279]: Duplicate Groups
            - generic [ref=e280]:
              - img [ref=e281]
              - generic [ref=e284]: "+39"
              - generic [ref=e285]: Files This Week
        - generic [ref=e286]:
          - generic [ref=e288]:
            - heading "Media Distribution" [level=3] [ref=e290]:
              - img [ref=e291]
              - text: Media Distribution
            - generic [ref=e296]:
              - img [ref=e297]:
                - generic [ref=e298]:
                  - img [ref=e300]
                  - img [ref=e302]
                  - img [ref=e304]
                  - img [ref=e306]
                  - img [ref=e308]
                  - img [ref=e310]
                  - generic [ref=e311]:
                    - generic [ref=e313]: 26%
                    - generic [ref=e315]: 10%
                    - generic [ref=e317]: 5%
                    - generic [ref=e319]: 18%
                    - generic [ref=e321]: 13%
                    - generic [ref=e323]: 28%
              - list [ref=e325]:
                - listitem [ref=e326]:
                  - img [ref=e327]
                  - generic [ref=e330]: "Audio: 10"
                - listitem [ref=e331]:
                  - img [ref=e332]
                  - generic [ref=e335]: "Book: 4"
                - listitem [ref=e336]:
                  - img [ref=e337]
                  - generic [ref=e340]: "Document: 2"
                - listitem [ref=e341]:
                  - img [ref=e342]
                  - generic [ref=e345]: "Image: 7"
                - listitem [ref=e346]:
                  - img [ref=e347]
                  - generic [ref=e350]: "Other: 5"
                - listitem [ref=e351]:
                  - img [ref=e352]
                  - generic [ref=e355]: "Video: 11"
          - generic [ref=e357]:
            - heading "System Status" [level=3] [ref=e359]:
              - img [ref=e360]
              - text: System Status
            - generic [ref=e363]:
              - generic [ref=e365]:
                - generic [ref=e366]: CPU Usage
                - generic [ref=e367]: 45%
              - generic [ref=e371]:
                - generic [ref=e372]: Memory Usage
                - generic [ref=e373]: 62%
              - generic [ref=e377]:
                - generic [ref=e378]: Disk Usage
                - generic [ref=e379]: 78%
              - generic [ref=e382]:
                - generic [ref=e383]: Network
                - generic [ref=e384]: Online
              - generic [ref=e385]:
                - generic [ref=e386]: Uptime
                - generic [ref=e387]: 5d 12h 34m
        - generic [ref=e388]:
          - generic [ref=e390]:
            - generic [ref=e391]:
              - heading "Recent Activity" [level=3] [ref=e392]:
                - img [ref=e393]
                - text: Recent Activity
              - generic [ref=e395]:
                - button "All" [ref=e396] [cursor=pointer]
                - button "Playing" [ref=e397] [cursor=pointer]
                - button "Uploads" [ref=e398] [cursor=pointer]
            - generic [ref=e400]:
              - generic [ref=e401]:
                - img [ref=e403]
                - generic [ref=e405]:
                  - generic [ref=e406]:
                    - paragraph [ref=e407]: Started watching "Sample Movie.mp4"
                    - generic [ref=e408]: 5 minutes ago
                  - generic [ref=e409]:
                    - generic [ref=e410]: John Doe
                    - generic [ref=e411]: • 1080p • 2h 15m
              - generic [ref=e412]:
                - img [ref=e414]
                - generic [ref=e417]:
                  - generic [ref=e418]:
                    - paragraph [ref=e419]: Uploaded "New Episode.mkv"
                    - generic [ref=e420]: 15 minutes ago
                  - generic [ref=e421]:
                    - generic [ref=e422]: Jane Smith
                    - generic [ref=e423]: • 1.2GB • MKV
              - generic [ref=e424]:
                - img [ref=e426]
                - generic [ref=e431]:
                  - generic [ref=e432]:
                    - paragraph [ref=e433]: User "Mike Johnson" logged in
                    - generic [ref=e434]: 30 minutes ago
                  - generic [ref=e436]: Mike Johnson
              - generic [ref=e437]:
                - img [ref=e439]
                - generic [ref=e442]:
                  - generic [ref=e443]:
                    - paragraph [ref=e444]: Library scan completed successfully
                    - generic [ref=e445]: about 1 hour ago
                  - generic [ref=e447]: System
          - generic [ref=e449]:
            - heading "Quick Actions" [level=3] [ref=e451]
            - generic [ref=e453]:
              - button "Upload Media" [ref=e454] [cursor=pointer]:
                - img [ref=e455]
                - generic [ref=e458]: Upload Media
              - button "Scan Library" [ref=e459] [cursor=pointer]:
                - img [ref=e460]
                - generic [ref=e462]: Scan Library
              - button "Search" [ref=e463] [cursor=pointer]:
                - img [ref=e464]
                - generic [ref=e467]: Search
              - button "Settings" [ref=e468] [cursor=pointer]:
                - img [ref=e469]
                - generic [ref=e472]: Settings
    - generic: v1.1.0
```

# Test source

```ts
  7   | import net from 'node:net';
  8   | import { requireEnv } from '../fixtures/env';
  9   | 
  10  | export { expect };
  11  | 
  12  | export const API_URL = (process.env.LIVE_API_URL || 'http://127.0.0.1:38082').replace(/\/$/, '');
  13  | export const adminUser = () => requireEnv('ADMIN_USERNAME');
  14  | export const adminPass = () => requireEnv('ADMIN_PASSWORD');
  15  | // Directory the specs append machine-readable observations to (one JSONL per run), consumed by make_evidence.mjs.
  16  | export const OBS_DIR = process.env.LIVE_OBS_DIR || '';
  17  | 
  18  | // RED switch: LIVE_RED=1 makes the primary selector of a spec deliberately wrong so the RED polarity can be captured.
  19  | export const RED = process.env.LIVE_RED === '1';
  20  | 
  21  | export const randomSecret = (n = 18) => crypto.randomBytes(n).toString('base64url') + 'aA1!';
  22  | 
  23  | // Each page records every response of the real backend (method + status) so the evidence carries a real request histogram.
  24  | function record(testInfo: TestInfo, rec: Record<string, unknown>) {
  25  |   if (!OBS_DIR) return;
  26  |   fs.mkdirSync(OBS_DIR, { recursive: true });
  27  |   fs.appendFileSync(path.join(OBS_DIR, 'obs.jsonl'), JSON.stringify({ spec: path.basename(testInfo.file), project: testInfo.project.name, test: testInfo.title, ...rec }) + '\n');
  28  | }
  29  | 
  30  | export const test = base.extend<{ obs: void }>({
  31  |   obs: [async ({ page }, use, testInfo) => {
  32  |     page.on('response', (r) => {
  33  |       const u = r.url();
  34  |       if (u.includes('/api/') || u.includes('/health')) record(testInfo, { kind: 'http', method: r.request().method(), status: r.status() });
  35  |     });
  36  |     await use();
  37  |     record(testInfo, { kind: 'verdict', status: testInfo.status, expected: testInfo.expectedStatus });
  38  |   }, { auto: true }],
  39  | });
  40  | 
  41  | export async function shot(page: Page, testInfo: TestInfo, name: string) {
  42  |   const p = testInfo.outputPath(`${name}.png`);
  43  |   await page.screenshot({ path: p, fullPage: false });
  44  |   await testInfo.attach(name, { path: p, contentType: 'image/png' });
  45  |   record(testInfo, { kind: 'screenshot', name, file: p, sha256: crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex') });
  46  | }
  47  | 
  48  | // Real API login with backoff on the real login rate limiter (HTTP 429).
  49  | export async function apiLogin(request: APIRequestContext, username: string, password: string) {
  50  |   for (let i = 0; i < 6; i++) {
  51  |     const res = await request.post(`${API_URL}/api/v1/auth/login`, { data: { username, password } });
  52  |     if (res.status() === 429) { await new Promise((r) => setTimeout(r, 11_000)); continue; }
  53  |     return res;
  54  |   }
  55  |   throw new Error('login rate limit (429) did not clear');
  56  | }
  57  | 
  58  | export async function adminToken(request: APIRequestContext): Promise<string> {
  59  |   const res = await apiLogin(request, adminUser(), adminPass());
  60  |   expect(res.status(), 'admin API login').toBe(200);
  61  |   return (await res.json()).session_token as string;
  62  | }
  63  | 
  64  | const SEL = {
  65  |   user: 'input[placeholder*="username" i]',
  66  |   pass: 'input[placeholder*="password" i]',
  67  | };
  68  | 
  69  | // Login through the real login form (the SPA then holds the session; subsequent navigation uses client-side links).
  70  | export async function uiLogin(page: Page, username: string, password: string) {
  71  |   await page.goto('/login');
  72  |   await page.locator(SEL.user).fill(username);
  73  |   await page.locator(SEL.pass).fill(password);
  74  |   for (let i = 0; i < 4; i++) {
  75  |     const [resp] = await Promise.all([
  76  |       page.waitForResponse((r) => r.url().includes('/auth/login') && r.request().method() === 'POST'),
  77  |       page.locator('button[type="submit"]').click(),
  78  |     ]);
  79  |     if (resp.status() !== 429) return resp;
  80  |     await page.waitForTimeout(11_000);
  81  |   }
  82  |   throw new Error('UI login rate limited');
  83  | }
  84  | 
  85  | export async function headerReady(page: Page) {
  86  |   await page.locator('header').getByRole('link').filter({ visible: true }).first().waitFor();
  87  |   await page.waitForTimeout(300);
  88  | }
  89  | 
  90  | export async function uiAdminLogin(page: Page) {
  91  |   const resp = await uiLogin(page, adminUser(), adminPass());
  92  |   expect(resp.status(), 'real /auth/login status').toBe(200);
  93  |   await page.waitForURL('**/dashboard', { timeout: 20_000 });
  94  | }
  95  | 
  96  | // Navigate with the app's own navigation (works at desktop and 390px: opens the mobile menu when the link is hidden).
  97  | export async function navTo(page: Page, label: string) {
  98  |   const vis = () => page.locator(`a:has-text("${label}")`).filter({ visible: true }).first();
  99  |   // wait until the header has rendered one of the entry points (link, More, or mobile menu button)
  100 |   await headerReady(page);
  101 |   if (!(await vis().isVisible().catch(() => false))) {
  102 |     const more = page.getByRole('button', { name: /^more$/i }).filter({ visible: true }).first();
  103 |     const menu = page.getByRole('button', { name: /open menu/i }).filter({ visible: true }).first();
  104 |     if (await more.isVisible().catch(() => false)) await more.click();
  105 |     else if (await menu.isVisible().catch(() => false)) await menu.click();
  106 |   }
> 107 |   await vis().click();
      |               ^ Error: locator.click: Test timeout of 120000ms exceeded.
  108 | }
  109 | 
  110 | // Artifact fingerprint READ FROM THE TARGET at run time: the real /health payload plus the running container's image digest.
  111 | export async function targetFingerprint(request: APIRequestContext) {
  112 |   const res = await request.get(`${API_URL}/health`);
  113 |   return { status: res.status(), body: await res.json().catch(() => ({})) };
  114 | }
  115 | 
  116 | // Real TCP forwarder for network-fault injection: forwards real bytes to the real web server; cut() severs every live
  117 | // connection and refuses new ones (a genuine network outage as seen by the browser), restore() accepts again.
  118 | export async function startForwarder(targetHost: string, targetPort: number) {
  119 |   let blocked = false;
  120 |   const socks = new Set<net.Socket>();
  121 |   const server = net.createServer((c) => {
  122 |     if (blocked) { c.destroy(); return; }
  123 |     const up = net.connect(targetPort, targetHost);
  124 |     socks.add(c); socks.add(up);
  125 |     const drop = () => { c.destroy(); up.destroy(); socks.delete(c); socks.delete(up); };
  126 |     c.on('error', drop); up.on('error', drop); c.on('close', drop); up.on('close', drop);
  127 |     c.pipe(up); up.pipe(c);
  128 |   });
  129 |   await new Promise<void>((r) => server.listen(0, '127.0.0.1', r));
  130 |   const port = (server.address() as net.AddressInfo).port;
  131 |   return {
  132 |     url: `http://127.0.0.1:${port}`,
  133 |     cut() { blocked = true; for (const s of socks) s.destroy(); socks.clear(); },
  134 |     restore() { blocked = false; },
  135 |     close() { for (const s of socks) s.destroy(); server.close(); },
  136 |   };
  137 | }
  138 | 
```