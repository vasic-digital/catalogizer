const NM=process.env.NM; // catalog-web/node_modules/ with trailing slash
const CHROME=process.env.CHROME; // chrome-headless-shell executable
const {chromium}=require(NM+'playwright-core');
(async()=>{
 const b=await chromium.launch({executablePath:CHROME});
 const ctx=await b.newContext({viewport:{width:1440,height:900}});const p=await ctx.newPage();
 const calls=[];p.on('request',r=>{if(r.url().includes('/api/'))calls.push(r.method()+' '+r.url().replace(/^https?:\/\/[^/]+/,''))});
 await p.goto(process.env.BASE+'/login');
 await p.fill('input[type=text],input[name=username]',process.env.U);await p.fill('input[type=password]',process.env.P);
 await p.keyboard.press('Enter');await p.waitForTimeout(3000);
 await p.goto(process.env.BASE+'/dashboard');await p.waitForTimeout(2500);
 calls.length=0;
 await p.fill('#header-search','zzznomatchzzz');await p.keyboard.press('Enter');await p.waitForTimeout(4000);
 const out={url:p.url().replace(/^https?:\/\/[^/]+/,''),h1:await p.locator('h1').allInnerTexts(),
  searchInputValue:await p.evaluate(()=>[...document.querySelectorAll('input')].map(i=>i.placeholder+'='+i.value)),
  body:(await p.innerText('main')).slice(0,400),entityCalls:calls.filter(c=>c.includes('entit'))};
 console.log(JSON.stringify(out,null,1));await b.close();})();
