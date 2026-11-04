// Local stand-in for the stream-ticket API, for REAL-BROWSER component tests only (NOT live evidence, see README):
// POST /api/v1/stream/:id/ticket mints "<id>.<ms>"; GET /api/v1/stream/:id?ticket= serves Range requests while the ticket is
// younger than TTL_MS and answers 401 afterwards (the real server's 60 s lifetime, shortened). Throttled to ~1.5x the media
// bitrate so the browser cannot buffer the whole file. POST /ctl?cut=1 makes streams older than TTL drop the connection.
// POST /ctl?ttl=N|mintDelay=N|getDelay=N changes the TTL and delays mints / valid stream responses (lifecycle races).
import http from 'node:http'
import fs from 'node:fs'

export function startTicketServer({ port, file, ttlMs: ttl0, durationS }) {
  let ttlMs = ttl0 // POST /ctl?ttl=N changes it at run time
  const size = fs.statSync(file).size
  const rate = (size / durationS) * 1.5 // bytes per second
  const log = []
  let cut = false
  let mintDelay = 0 // ms; POST /ctl?mintDelay=N delays every ticket mint (holds a seek-mint in flight)
  let getDelay = 0 // ms; POST /ctl?getDelay=N delays every VALID stream response (holds a reload before loadedmetadata)
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
  const cors = (res) => { res.setHeader('Access-Control-Allow-Origin', '*'); res.setHeader('Access-Control-Allow-Headers', '*'); res.setHeader('Access-Control-Allow-Methods', 'GET,POST,OPTIONS') }
  const srv = http.createServer(async (req, res) => {
    cors(res)
    if (req.method === 'OPTIONS') { res.writeHead(204); return res.end() }
    if (req.method === 'POST' && req.url.startsWith('/ctl')) {
      const q = new URL(req.url, 'http://x').searchParams
      if (q.has('cut')) cut = q.get('cut') === '1'
      if (q.has('ttl')) ttlMs = Number(q.get('ttl'))
      if (q.has('mintDelay')) mintDelay = Number(q.get('mintDelay'))
      if (q.has('getDelay')) getDelay = Number(q.get('getDelay'))
      res.writeHead(200); return res.end('ok')
    }
    let m = req.url.match(/^\/api\/v1\/stream\/(\d+)\/ticket/)
    if (req.method === 'POST' && m) {
      req.resume()
      if (mintDelay) await sleep(mintDelay)
      const t = `${m[1]}.${Date.now()}`
      log.push({ at: Date.now(), kind: 'mint', id: Number(m[1]) })
      res.writeHead(200, { 'Content-Type': 'application/json' }); return res.end(JSON.stringify({ ticket: t, expires_in: ttlMs / 1000 }))
    }
    m = req.url.match(/^\/api\/v1\/stream\/(\d+)\?ticket=([^&]+)/)
    if (req.method === 'GET' && m) {
      const [tid, ts] = decodeURIComponent(m[2]).split('.')
      const age = Date.now() - Number(ts)
      if (Number(tid) !== Number(m[1]) || age > ttlMs) { log.push({ at: Date.now(), kind: '401', range: req.headers.range || '' }); res.writeHead(401, { 'Content-Length': 0 }); return res.end() }
      if (getDelay) await sleep(getDelay)
      let start = 0, end = size - 1
      const r = /bytes=(\d+)-(\d*)/.exec(req.headers.range || '')
      if (r) { start = Number(r[1]); if (r[2]) end = Number(r[2]) }
      log.push({ at: Date.now(), kind: 'get', range: req.headers.range || '' })
      res.writeHead(r ? 206 : 200, { 'Content-Type': 'video/mp4', 'Accept-Ranges': 'bytes', ...(r ? { 'Content-Range': `bytes ${start}-${end}/${size}` } : {}), 'Content-Length': end - start + 1 })
      const rs = fs.createReadStream(file, { start, end, highWaterMark: 16384 })
      let sent = 0
      const t0 = Date.now()
      rs.on('data', (c) => {
        if (cut && Date.now() - Number(ts) > ttlMs) { log.push({ at: Date.now(), kind: 'cut' }); rs.destroy(); res.destroy(); return }
        res.write(c); sent += c.length
        const ahead = sent / rate * 1000 - (Date.now() - t0)
        if (ahead > 0) { rs.pause(); setTimeout(() => rs.resume(), ahead) }
      })
      rs.on('end', () => res.end())
      res.on('close', () => rs.destroy())
      return
    }
    res.writeHead(404); res.end()
  })
  return new Promise((resolve) => srv.listen(port, '127.0.0.1', () => resolve({ log, close: () => new Promise((r) => { srv.closeAllConnections?.(); srv.close(r) }) })))
}
