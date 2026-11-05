const {chromium}=require('/home/milosvasic/.cache/claude-tmp/lv.lRxQmt/repo/catalog-web/node_modules/playwright-core');const fs=require('fs');
const axeSrc=fs.readFileSync('/home/milosvasic/.cache/claude-tmp/lv.lRxQmt/repo/catalog-web/node_modules/axe-core/axe.min.js','utf8');
(async()=>{
 const b=await chromium.launch({executablePath:'/home/milosvasic/.cache/ms-playwright/chromium_headless_shell-1228/chrome-headless-shell-linux64/chrome-headless-shell'});const out={};
 const ctx=await b.newContext({viewport:{width:1440,height:900}});const p=await ctx.newPage();
 await p.goto(process.env.BASE+'/login');
 await p.fill('input[type=text],input[name=username]',process.env.U);await p.fill('input[type=password]',process.env.P);
 await p.keyboard.press('Enter');await p.waitForTimeout(3000);console.error('URL',p.url());
 for(const path of ['/media','/collections','/favorites','/playlists','/ai','/dashboard','/analytics']){
  await p.goto(process.env.BASE+path);await p.waitForTimeout(7000);
  await p.evaluate(axeSrc);
  const r=await p.evaluate(async()=>{const r=await axe.run(document,{runOnly:{type:'rule',values:['button-name','select-name','svg-img-alt']}});return r.violations.map(v=>[v.id,v.nodes.length,v.nodes.slice(0,80).map(n=>n.html.slice(0,260))])});
  out[path]=r;console.error(path,p.url());}
 await p.setViewportSize({width:360,height:800});await p.goto(process.env.BASE+'/admin');await p.waitForTimeout(6000);console.error((await p.innerText('body')).slice(0,300));
 out.admin360=await p.evaluate(()=>{const w=[];document.querySelectorAll('*').forEach(e=>{const r=e.getBoundingClientRect();if(r.right>361)w.push(e.tagName+'.'+String(e.className).slice(0,60)+' '+Math.round(r.right))});return {sw:document.documentElement.scrollWidth,bw:document.body.scrollWidth,cw:document.documentElement.clientWidth,wide:w.slice(0,8)}});
 console.log(JSON.stringify(out,null,1));await b.close();})();
