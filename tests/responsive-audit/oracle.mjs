// Layout oracle: runs inside the page. Returns machine-readable findings.
export const oracleFn = (opts) => {
  const vw = opts.vw || window.innerWidth, vh = opts.vh || window.innerHeight; // intended CSS viewport; isMobile emulation grows innerWidth to fit overflowing content, which would hide overflow
  const touch = opts.touch;
  const vis = (el) => {
    const cs = getComputedStyle(el); if (cs.display==='none'||cs.visibility==='hidden'||+cs.opacity===0) return null;
    const r = el.getBoundingClientRect(); if (r.width<=0||r.height<=0) return null;
    // intentionally invisible-to-users: visually-hidden skip links / sr-only text, and controls inside a closed off-canvas drawer (fixed ancestor fully outside the viewport)
    if (el.closest('.visually-hidden,.sr-only,[aria-hidden="true"],[inert]')) return null;
    for (let p=el;p&&p!==document.documentElement;p=p.parentElement){ if(getComputedStyle(p).position==='fixed'){ const pr=p.getBoundingClientRect(); if(pr.right<=1||pr.left>=(opts.vw||window.innerWidth)-1) return null; } }
    return r;
  };
  const sel = (el) => { let s=el.tagName.toLowerCase(); if(el.id) s+='#'+el.id; else if(typeof el.className==='string'&&el.className.trim()) s+='.'+el.className.trim().split(/\s+/).slice(0,3).join('.'); const t=(el.getAttribute('aria-label')||el.textContent||'').trim().replace(/\s+/g,' ').slice(0,30); return s+(t?` "${t}"`:''); };
  const scrollAncestor = (el) => { for (let p=el.parentElement;p&&p!==document.body&&p!==document.documentElement;p=p.parentElement){ const cs=getComputedStyle(p); if(/(auto|scroll|hidden|clip)/.test(cs.overflowX)) { const pr=p.getBoundingClientRect(); return {p,pr}; } } return null; };
  const scrolledOutOfClip = (el, x, y) => { // point lies outside a scroll/hidden ancestor's visible box: the element is clipped (reachable by scrolling), not occluded
    for (let p=el.parentElement;p&&p!==document.documentElement&&p!==document.body;p=p.parentElement){ const cs=getComputedStyle(p); if(/(auto|scroll|hidden|clip)/.test(cs.overflowX+cs.overflowY)){ const pr=p.getBoundingClientRect(); if(x<pr.left||x>pr.right||y<pr.top||y>pr.bottom) return true; } } return false; };
  const hiddenByAncestor = (el, r) => { // clipped intentionally by an overflow ancestor (scroll/hidden) -> not an off-screen control
    for (let p=el.parentElement;p&&p!==document.documentElement;p=p.parentElement){ const cs=getComputedStyle(p); if(/(auto|scroll|hidden|clip)/.test(cs.overflowX)&&p!==document.body){ const pr=p.getBoundingClientRect(); if(r.right>pr.right+1||r.left<pr.left-1) return true; } } return false; };
  const out = { vw, vh, docScrollWidth: Math.max(document.documentElement.scrollWidth, window.innerWidth), bodyScrollWidth: document.body.scrollWidth, textLen:(document.body.innerText||'').trim().length, findings: [] };
  const add = (type, el, extra) => { if (out.findings.filter(f=>f.type===type).length<opts.cap) out.findings.push({type, el: el?sel(el):null, ...extra}); out['n_'+type]=(out['n_'+type]||0)+1; };
  const all = Array.from(document.querySelectorAll('body *'));
  // 1 horizontal overflow of the document
  if (Math.max(document.documentElement.scrollWidth, window.innerWidth) > vw+1) {
    add('h_overflow', document.documentElement, {scrollWidth: Math.max(document.documentElement.scrollWidth, window.innerWidth), vw});
    for (const el of all) { const r=vis(el); if(!r) continue; const cs=getComputedStyle(el); if(cs.position==='fixed') continue; if(r.right>vw+1 && !hiddenByAncestor(el,r)) { add('h_overflow_culprit', el, {right: Math.round(r.right), vw}); } }
  }
  const interactiveSel = 'a[href],button,input:not([type=hidden]),select,textarea,[role=button],[role=tab],[role=menuitem],[role=link],[tabindex]:not([tabindex="-1"])';
  const inter = Array.from(document.querySelectorAll(interactiveSel));
  for (const el of inter) {
    const r = vis(el); if(!r) continue;
    // 2 off-screen controls (horizontal) not intentionally inside a scroller
    if ((r.left < -1 || r.right > vw+1) && !hiddenByAncestor(el,r)) {
      const cs=getComputedStyle(el); let off=false; for(let p=el;p;p=p.parentElement){ const t=getComputedStyle(p).transform; if(/translate|matrix/.test(t)&&t!=='none'&&p!==el){ /* sliding drawers */ off=off; } }
      add('offscreen_control', el, {left: Math.round(r.left), right: Math.round(r.right), vw});
    }
    // 3 tap targets
    const minPx = 44;
    const isInlineText = el.tagName==='A' && getComputedStyle(el).display==='inline' && el.closest('p,li,span,td');
    const sr = el.type==='checkbox'||el.type==='radio'||el.type==='range';
    if ((r.width < minPx || r.height < minPx) && !isInlineText) {
      // checkboxes/radios often wrapped by a larger <label>; measure label too
      let rr=r; const lab = el.labels&&el.labels[0]; if (lab){ const lr=lab.getBoundingClientRect(); if(lr.width*lr.height>rr.width*rr.height) rr=lr; }
      if (rr.width < minPx || rr.height < minPx) add(touch?'tap_target_small':'tap_target_small_desktop', el, {w: Math.round(rr.width), h: Math.round(rr.height)});
    }
    // 4 occlusion: centre of control hit-test (only when within the viewport)
    const cx=Math.min(Math.max(r.left+r.width/2,0),vw-1), cy=r.top+r.height/2;
    if (cy>=0 && cy<vh && r.width>2 && r.height>2) { const top=document.elementFromPoint(cx,cy); if(top && top!==el && !el.contains(top) && !top.contains(el) && !scrolledOutOfClip(el,cx,cy)) { const tr=top.getBoundingClientRect(); add('occluded_control', el, {by: sel(top)}); } }
  }
  // 5 small text, 6 clipped text, 7 overlapping text
  const textEls = [];
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  const seen = new Set();
  for (let n=walker.nextNode(); n; n=walker.nextNode()) { if(!n.nodeValue.trim()) continue; const el=n.parentElement; if(!el||seen.has(el)) continue; if(['SCRIPT','STYLE','NOSCRIPT'].includes(el.tagName)) continue; const r=vis(el); if(!r) continue; seen.add(el); textEls.push([el,r]); }
  for (const [el,r] of textEls) {
    const cs=getComputedStyle(el); const fs=parseFloat(cs.fontSize);
    if (fs < 12) add('small_text', el, {fontSize: fs});
    // clipped: content wider than box with hidden overflow and no ellipsis handling
    if ((cs.overflowX==='hidden'||cs.overflowX==='clip') && el.scrollWidth > el.clientWidth+2 && cs.textOverflow!=='ellipsis' && !(cs.webkitLineClamp&&cs.webkitLineClamp!=='none')) add('clipped_text', el, {scrollWidth: el.scrollWidth, clientWidth: el.clientWidth});
    if (r.right>vw+1 && !hiddenByAncestor(el,r) && getComputedStyle(el).position!=='fixed') add('text_off_viewport', el, {right: Math.round(r.right), vw});
  }
  // overlapping text boxes (non-ancestral, both visible, in viewport band) — use Range rects of the text node for accuracy
  for (const [el] of textEls) { const cs=getComputedStyle(el); if(cs.position==='fixed'||el.closest('.sr-only')) continue; const rg=document.createRange(); rg.selectNodeContents(el); const rr=rg.getClientRects()[0]; if(rr&&rr.width>4&&rr.height>4&&rr.left>=0&&(rr.left<8||rr.right>vw-8)&&!hiddenByAncestor(el,rr)) add('no_gutter', el, {left: Math.round(rr.left), right: Math.round(rr.right), vw}); }
  const lineRects = [];
  for (const [el] of textEls) { const rg=document.createRange(); rg.selectNodeContents(el); for (const rr of Array.from(rg.getClientRects())) if (rr.width>2&&rr.height>2&&rr.top<vh*3) lineRects.push([el,rr]); }
  lineRects.sort((a,b)=>a[1].top-b[1].top);
  const reported = new Set();
  for (let i=0;i<lineRects.length;i++){ for(let j=i+1;j<lineRects.length;j++){ const [a,ra]=lineRects[i],[b,rb]=lineRects[j]; if(rb.top>=ra.bottom) break; if(a===b||a.contains(b)||b.contains(a)) continue; const ix=Math.min(ra.right,rb.right)-Math.max(ra.left,rb.left), iy=Math.min(ra.bottom,rb.bottom)-Math.max(ra.top,rb.top); if(ix>2&&iy>2){ const ov=ix*iy/Math.min(ra.width*ra.height,rb.width*rb.height); if(ov>0.3){ const k=sel(a)+'|'+sel(b); if(!reported.has(k)){ reported.add(k); add('overlapping_text', a, {with: sel(b), overlap: +ov.toFixed(2)}); } } } } }
  return out;
};
