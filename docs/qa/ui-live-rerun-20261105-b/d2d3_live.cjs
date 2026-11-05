// Live UI-D2/D3 check. env: NM (dir with playwright-core), CHROME, API, BASE, U, P (read from 0600 env, never printed)
const {chromium}=require(process.env.NM+'/playwright-core');
(async()=>{
 const out={};
 const r=await fetch(process.env.API+'/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:process.env.U,password:process.env.P})});
 const j=await r.json();const H={Authorization:'Bearer '+j.session_token};
 for(const p of ['/identities','/smb/identities','/entities/3','/entities/35','/entities/35/duplicates','/recommendations/similar/35'])
  out['api '+p]=(await fetch(process.env.API+p,{headers:H})).status;
 const b=await chromium.launch({executablePath:process.env.CHROME});const page=await b.newPage({viewport:{width:1440,height:900}});
 await page.goto(process.env.BASE+'/login');
 await page.evaluate(([t,u])=>{localStorage.setItem('auth_token',t);localStorage.setItem('user',JSON.stringify(u));},[j.session_token,j.user]);
 const bad=[];page.on('response',rs=>{if(rs.url().includes('/api/')&&rs.status()>=400)bad.push(rs.status()+' '+rs.url().replace(/^https?:\/\/[^/]+/,''))});
 for(const path of ['/entity/3','/entity/35','/identities']){
  bad.length=0;await page.goto(process.env.BASE+path);await page.waitForTimeout(6000);
  out['ui '+path]={url:page.url().replace(/^https?:\/\/[^/]+/,''),alerts:await page.locator('[role=alert]').allInnerTexts(),
   notFoundText:await page.getByText('Entity not found').count(),h1:await page.locator('h1').allInnerTexts(),
   body:(await page.innerText('main')).replace(/\s+/g,' ').slice(0,250),http4xx:[...bad]};
  await page.screenshot({path:process.env.SHOTDIR+'/d2d3_'+path.replace(/\//g,'_')+'_run'+process.env.RUN+'.png'});
 }
 // /media item -> does a card link to /entity?
 await page.goto(process.env.BASE+'/media');await page.waitForTimeout(6000);
 out['media entity links']=await page.evaluate(()=>[...document.querySelectorAll('a[href*="/entity/"]')].length);
 out['media sample hrefs']=await page.evaluate(()=>[...document.querySelectorAll('a')].map(a=>a.getAttribute('href')).filter(h=>h&&h.startsWith('/')).slice(0,15));
 console.log(JSON.stringify(out,null,1));await b.close();})();
