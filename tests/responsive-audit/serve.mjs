// static SPA server + /api,/ws reverse proxy. usage: node serve.mjs <dir> <port> [apiPort] [spa=1]
import http from 'node:http'; import fs from 'node:fs'; import path from 'node:path';
const [,, dir, port, apiPort, spa='1'] = process.argv;
const mime={'.html':'text/html','.js':'text/javascript','.mjs':'text/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg','.jpeg':'image/jpeg','.ico':'image/x-icon','.woff2':'font/woff2','.woff':'font/woff','.map':'application/json','.webp':'image/webp','.txt':'text/plain','.xml':'application/xml'};
const srv=http.createServer((req,res)=>{
  const u=new URL(req.url,'http://x');
  if(apiPort && (u.pathname.startsWith('/api')||u.pathname.startsWith('/health'))){
    const p=http.request({host:'127.0.0.1',port:apiPort,path:req.url,method:req.method,headers:req.headers},r=>{res.writeHead(r.statusCode,r.headers);r.pipe(res)});
    p.on('error',e=>{res.writeHead(502);res.end(String(e))}); req.pipe(p); return;}
  let f=path.join(dir,decodeURIComponent(u.pathname));
  if(!f.startsWith(path.resolve(dir))) {res.writeHead(403);return res.end();}
  const tryf=[f,f+'.html',path.join(f,'index.html')];
  for(const c of tryf){ try{ if(fs.statSync(c).isFile()){ res.writeHead(200,{'content-type':mime[path.extname(c)]||'application/octet-stream'}); return fs.createReadStream(c).pipe(res);} }catch{} }
  if(spa==='1'){ res.writeHead(200,{'content-type':'text/html'}); return fs.createReadStream(path.join(dir,'index.html')).pipe(res);}
  const nf=path.join(dir,'404.html'); res.writeHead(404,{'content-type':'text/html'}); fs.existsSync(nf)?fs.createReadStream(nf).pipe(res):res.end('404');
});
import net from 'node:net';
srv.on('upgrade',(req,sock,head)=>{ if(!apiPort) return sock.destroy(); const up=net.connect(+apiPort,'127.0.0.1',()=>{ let h=`${req.method} ${req.url} HTTP/1.1\r\n`; for(let i=0;i<req.rawHeaders.length;i+=2) h+=`${req.rawHeaders[i]}: ${req.rawHeaders[i+1]}\r\n`; up.write(h+'\r\n'); if(head&&head.length) up.write(head); sock.pipe(up); up.pipe(sock); }); up.on('error',()=>sock.destroy()); sock.on('error',()=>up.destroy()); });
srv.listen(+port,'127.0.0.1',()=>console.log('listening',port));
