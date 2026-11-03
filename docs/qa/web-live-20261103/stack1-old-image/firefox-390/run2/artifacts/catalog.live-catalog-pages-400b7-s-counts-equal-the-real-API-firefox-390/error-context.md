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
    - generic [ref=e13]: Disconnected
  - generic [ref=e14]:
    - banner [ref=e15]:
      - generic [ref=e17]:
        - link "C Catalogizer" [ref=e18] [cursor=pointer]:
          - /url: /
          - generic [ref=e20]: C
          - generic [ref=e21]: Catalogizer
        - button [ref=e22] [cursor=pointer]:
          - img [ref=e23]
    - main [ref=e27]:
      - generic [ref=e28]:
        - generic [ref=e29]:
          - generic [ref=e30]:
            - heading "Welcome back, admin!" [level=1] [ref=e31]
            - paragraph [ref=e32]: Here's what's happening with your media library today.
          - 'button "Last updated: 12:41:23 AM" [ref=e34] [cursor=pointer]':
            - img [ref=e35]
            - text: "Last updated: 12:41:23 AM"
        - generic [ref=e39]:
          - generic [ref=e40]:
            - generic [ref=e41]:
              - heading "Total Media" [level=3] [ref=e42]
              - img [ref=e43]
            - generic [ref=e47]:
              - generic [ref=e48]: "39"
              - generic [ref=e49]:
                - generic [ref=e50]:
                  - img [ref=e51]
                  - generic [ref=e54]: 12.5%
                - generic [ref=e55]: vs last month
            - img [ref=e57]
          - generic [ref=e61]:
            - generic [ref=e62]:
              - heading "Storage Used" [level=3] [ref=e63]
              - img [ref=e64]
            - generic [ref=e69]:
              - generic [ref=e70]: 2.31 MB
              - generic [ref=e71]:
                - generic [ref=e72]:
                  - img [ref=e73]
                  - generic [ref=e76]: 8.2%
                - generic [ref=e77]: vs last month
            - img [ref=e79]
          - generic [ref=e84]:
            - generic [ref=e85]:
              - heading "Recent Additions" [level=3] [ref=e86]
              - img [ref=e87]
            - generic [ref=e91]:
              - generic [ref=e92]: "39"
              - generic [ref=e93]:
                - generic [ref=e94]:
                  - img [ref=e95]
                  - generic [ref=e98]: 15.3%
                - generic [ref=e99]: last 7 days
            - img [ref=e101]
          - generic [ref=e105]:
            - generic [ref=e106]:
              - heading "Quality Score" [level=3] [ref=e107]
              - img [ref=e108]
            - generic [ref=e110]:
              - generic [ref=e111]: HD
              - generic [ref=e112]:
                - generic [ref=e113]:
                  - img [ref=e114]
                  - generic [ref=e117]: 5.7%
                - generic [ref=e118]: average quality
            - img [ref=e120]
          - generic [ref=e122]:
            - generic [ref=e123]:
              - heading "Active Users" [level=3] [ref=e124]
              - img [ref=e125]
            - generic [ref=e130]:
              - generic [ref=e131]: "3"
              - generic [ref=e132]:
                - generic [ref=e133]:
                  - img [ref=e134]
                  - generic [ref=e137]: 3.2%
                - generic [ref=e138]: online now
            - img [ref=e140]
          - generic [ref=e145]:
            - generic [ref=e146]:
              - heading "Total Users" [level=3] [ref=e147]
              - img [ref=e148]
            - generic [ref=e153]:
              - generic [ref=e154]: "12"
              - generic [ref=e155]:
                - generic [ref=e156]:
                  - img [ref=e157]
                  - generic [ref=e160]: 18.7%
                - generic [ref=e161]: registered users
            - img [ref=e163]
          - generic [ref=e168]:
            - generic [ref=e169]:
              - heading "Sessions Today" [level=3] [ref=e170]
              - img [ref=e171]
            - generic [ref=e174]:
              - generic [ref=e175]: "24"
              - generic [ref=e176]:
                - generic [ref=e177]:
                  - img [ref=e178]
                  - generic [ref=e181]: 22.1%
                - generic [ref=e182]: viewing sessions
            - img [ref=e184]
          - generic [ref=e187]:
            - generic [ref=e188]:
              - heading "Avg Session" [level=3] [ref=e189]
              - img [ref=e190]
            - generic [ref=e193]:
              - generic [ref=e194]: 45m
              - generic [ref=e195]:
                - generic [ref=e196]:
                  - img [ref=e197]
                  - generic [ref=e200]: 7.5%
                - generic [ref=e201]: viewing duration
            - img [ref=e203]
        - generic [ref=e207]:
          - heading "Media Entities (18 total)" [level=3] [ref=e209]:
            - img [ref=e210]
            - text: Media Entities (18 total)
          - generic [ref=e220]:
            - generic [ref=e221]:
              - img [ref=e222]
              - generic [ref=e231]:
                - generic [ref=e232]: "7"
                - generic [ref=e233]: movie
            - generic [ref=e234]:
              - img [ref=e235]
              - generic [ref=e239]:
                - generic [ref=e240]: "4"
                - generic [ref=e241]: music album
            - generic [ref=e242]:
              - img [ref=e243]
              - generic [ref=e246]:
                - generic [ref=e247]: "3"
                - generic [ref=e248]: book
            - generic [ref=e249]:
              - img [ref=e250]
              - generic [ref=e252]:
                - generic [ref=e253]: "1"
                - generic [ref=e254]: comic
            - generic [ref=e255]:
              - img [ref=e256]
              - generic [ref=e265]:
                - generic [ref=e266]: "1"
                - generic [ref=e267]: tv episode
            - generic [ref=e268]:
              - img [ref=e269]
              - generic [ref=e278]:
                - generic [ref=e279]: "1"
                - generic [ref=e280]: tv season
            - generic [ref=e281]:
              - img [ref=e282]
              - generic [ref=e285]:
                - generic [ref=e286]: "1"
                - generic [ref=e287]: tv show
        - generic [ref=e289]:
          - heading "Storage Statistics" [level=3] [ref=e291]:
            - img [ref=e292]
            - text: Storage Statistics
          - generic [ref=e298]:
            - generic [ref=e299]:
              - img [ref=e300]
              - generic [ref=e305]: "13"
              - generic [ref=e306]: Directories
            - generic [ref=e307]:
              - img [ref=e308]
              - generic [ref=e314]: "39"
              - generic [ref=e315]: Total Files
            - generic [ref=e316]:
              - img [ref=e317]
              - generic [ref=e322]: 0.0 GB
              - generic [ref=e323]: Total Size
            - generic [ref=e324]:
              - img [ref=e325]
              - generic [ref=e328]: "0"
              - generic [ref=e329]: Duplicate Groups
            - generic [ref=e330]:
              - img [ref=e331]
              - generic [ref=e334]: "+39"
              - generic [ref=e335]: Files This Week
        - generic [ref=e336]:
          - generic [ref=e338]:
            - heading "Media Distribution" [level=3] [ref=e340]:
              - img [ref=e341]
              - text: Media Distribution
            - generic [ref=e346]:
              - img [ref=e347]:
                - generic [ref=e348]:
                  - generic [ref=e349]:
                    - img [ref=e351]
                    - img [ref=e353]
                    - img [ref=e355]
                    - img [ref=e357]
                    - img [ref=e359]
                    - img [ref=e361]
                  - generic [ref=e362]:
                    - generic [ref=e363]: 26%
                    - generic [ref=e364]: 10%
                    - generic [ref=e365]: 5%
                    - generic [ref=e366]: 18%
                    - generic [ref=e367]: 13%
                    - generic [ref=e368]: 28%
              - list [ref=e370]:
                - listitem [ref=e371]:
                  - img [ref=e372]
                  - generic [ref=e375]: "Audio: 10"
                - listitem [ref=e376]:
                  - img [ref=e377]
                  - generic [ref=e380]: "Book: 4"
                - listitem [ref=e381]:
                  - img [ref=e382]
                  - generic [ref=e385]: "Document: 2"
                - listitem [ref=e386]:
                  - img [ref=e387]
                  - generic [ref=e390]: "Image: 7"
                - listitem [ref=e391]:
                  - img [ref=e392]
                  - generic [ref=e395]: "Other: 5"
                - listitem [ref=e396]:
                  - img [ref=e397]
                  - generic [ref=e400]: "Video: 11"
          - generic [ref=e402]:
            - heading "System Status" [level=3] [ref=e404]:
              - img [ref=e405]
              - text: System Status
            - generic [ref=e408]:
              - generic [ref=e410]:
                - generic [ref=e411]: CPU Usage
                - generic [ref=e412]: 45%
              - generic [ref=e416]:
                - generic [ref=e417]: Memory Usage
                - generic [ref=e418]: 62%
              - generic [ref=e422]:
                - generic [ref=e423]: Disk Usage
                - generic [ref=e424]: 78%
              - generic [ref=e427]:
                - generic [ref=e428]: Network
                - generic [ref=e429]: Online
              - generic [ref=e430]:
                - generic [ref=e431]: Uptime
                - generic [ref=e432]: 5d 12h 34m
        - generic [ref=e433]:
          - generic [ref=e435]:
            - generic [ref=e436]:
              - heading "Recent Activity" [level=3] [ref=e437]:
                - img [ref=e438]
                - text: Recent Activity
              - generic [ref=e440]:
                - button "All" [ref=e441] [cursor=pointer]
                - button "Playing" [ref=e442] [cursor=pointer]
                - button "Uploads" [ref=e443] [cursor=pointer]
            - generic [ref=e445]:
              - generic [ref=e446]:
                - img [ref=e448]
                - generic [ref=e450]:
                  - generic [ref=e451]:
                    - paragraph [ref=e452]: Started watching "Sample Movie.mp4"
                    - generic [ref=e453]: 5 minutes ago
                  - generic [ref=e454]:
                    - generic [ref=e455]: John Doe
                    - generic [ref=e456]: • 1080p • 2h 15m
              - generic [ref=e457]:
                - img [ref=e459]
                - generic [ref=e463]:
                  - generic [ref=e464]:
                    - paragraph [ref=e465]: Uploaded "New Episode.mkv"
                    - generic [ref=e466]: 15 minutes ago
                  - generic [ref=e467]:
                    - generic [ref=e468]: Jane Smith
                    - generic [ref=e469]: • 1.2GB • MKV
              - generic [ref=e470]:
                - img [ref=e472]
                - generic [ref=e477]:
                  - generic [ref=e478]:
                    - paragraph [ref=e479]: User "Mike Johnson" logged in
                    - generic [ref=e480]: 30 minutes ago
                  - generic [ref=e482]: Mike Johnson
              - generic [ref=e483]:
                - img [ref=e485]
                - generic [ref=e488]:
                  - generic [ref=e489]:
                    - paragraph [ref=e490]: Library scan completed successfully
                    - generic [ref=e491]: about 1 hour ago
                  - generic [ref=e493]: System
          - generic [ref=e495]:
            - heading "Quick Actions" [level=3] [ref=e497]
            - generic [ref=e499]:
              - button "Upload Media" [ref=e500] [cursor=pointer]:
                - img [ref=e501]
                - generic [ref=e505]: Upload Media
              - button "Scan Library" [ref=e506] [cursor=pointer]:
                - img [ref=e507]
                - generic [ref=e509]: Scan Library
              - button "Search" [ref=e510] [cursor=pointer]:
                - img [ref=e511]
                - generic [ref=e514]: Search
              - button "Settings" [ref=e515] [cursor=pointer]:
                - img [ref=e516]
                - generic [ref=e519]: Settings
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