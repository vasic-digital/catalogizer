const NM='/home/milosvasic/.cache/claude-tmp/ui-d5d6.Z902AZ/repo/catalog-web/node_modules/';
const {chromium}=require(NM+'playwright-core');const fs=require('fs');
const axeSrc=fs.readFileSync(NM+'axe-core/axe.min.js','utf8');
(async()=>{
 const b=await chromium.launch({executablePath:'/home/milosvasic/.cache/ms-playwright/chromium_headless_shell-1228/chrome-headless-shell-linux64/chrome-headless-shell'});const out={};
 for(const theme of (process.env.THEMES||'light,dark').split(',')){
 const ctx=await b.newContext({viewport:{width:1440,height:900}});const p=await ctx.newPage();
 await p.goto(process.env.BASE+'/login');
 await p.fill('input[type=text],input[name=username]',process.env.U);await p.fill('input[type=password]',process.env.P);
 await p.keyboard.press('Enter');await p.waitForTimeout(3000);
 await p.evaluate(t=>{localStorage.setItem('catalog-web-theme',t)},theme);
 for(const path of (process.env.PATHS||'/dashboard,/analytics,/browse,/subtitles,/admin,/settings,/ai,/identities,/media').split(',')){
  await p.goto(process.env.BASE+path);await p.waitForTimeout(6000);
  const cls=await p.evaluate(()=>document.documentElement.className);
  await p.evaluate(axeSrc);
  const r=await p.evaluate(async()=>{const r=await axe.run(document,{runOnly:{type:'rule',values:['color-contrast','svg-img-alt']}});return r.violations.map(v=>[v.id,v.nodes.length,v.nodes.map(n=>({h:n.html.slice(0,160),t:n.target.join(' '),s:(n.any[0]&&n.any[0].message||'').slice(0,160)}))])});
  out[theme+' '+path+' ['+cls+']']=r;}
 await ctx.close();}
 console.log(JSON.stringify(out,null,1));await b.close();})();
