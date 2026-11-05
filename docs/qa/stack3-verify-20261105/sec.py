import sys,json,time,subprocess,urllib.request,re
import lib
PH=sys.argv[1]; R=sys.argv[2]
res=[]
def chk(n,ok,d=None,skip=None):
    r={"phase":PH,"round":R,"check":n,"result":"SKIP" if skip else ("PASS" if ok else "FAIL"),"detail":skip or d}
    res.append(r); print(json.dumps(r,default=str)[:600],flush=True)
s,j=lib.login(); tok=j['session_token']; ST=lib.E['METRICS_SCRAPE_TOKEN']
H=lib.req('GET','/health')[1]; chk("env.health_build",H.get('build_number','')[:8]=="015896fd",H.get('build_number'))
def logs(since):
    r=subprocess.run(["podman","logs","--since",since,"catqa3-api"],capture_output=True,text=True); return r.stdout+r.stderr
def rawreq(m,p,hdr=None,body=None):
    r=urllib.request.Request(lib.B+p,data=body,method=m,headers=hdr or {})
    try:
        x=urllib.request.urlopen(r,timeout=30); return x.status,dict(x.headers),x.read(),x.headers.get_all('Set-Cookie') or []
    except urllib.error.HTTPError as e: return e.code,dict(e.headers),e.read(),e.headers.get_all('Set-Cookie') or []
if PH=="A":
    # S1 /metrics auth
    a=rawreq('GET','/metrics')[0]; b=rawreq('GET','/metrics',{"Authorization":"Bearer bogus"+"x"*40})[0]
    c=rawreq('GET','/metrics',{"Authorization":"Bearer "+ST}); d=rawreq('GET','/metrics',{"Authorization":"Bearer "+tok})
    chk("S1.metrics_anon_401",a==401,a); chk("S1.metrics_bogus_401",b==401,b)
    chk("S1.metrics_scrape_token_200",c[0]==200 and b'go_goroutines' in c[2],{"status":c[0],"bytes":len(c[2])})
    chk("S1.metrics_admin_session_200",d[0]==200,d[0])
    st=ST[:-1]+('0' if ST[-1]!='0' else '1'); chk("S1.metrics_scrape_token_off_by_one_401",rawreq('GET','/metrics',{"Authorization":"Bearer "+st})[0]==401)
    chk("S1.scrape_token_not_accepted_elsewhere",all(rawreq('GET',p,{"Authorization":"Bearer "+ST})[0] in(401,403) for p in('/api/v1/admin/users','/api/v1/media/stats','/api/v1/auth/profile')),"scrape token on admin/users, media/stats, auth/profile")
    pt=json.loads(urllib.request.urlopen("http://127.0.0.1:39090/api/v1/targets",timeout=10).read())['data']['activeTargets']
    chk("S1.prometheus_scrapes_with_static_token",any(t['labels']['job']=='catalogizer-api' and t['health']=='up' for t in pt),[(t['labels']['job'],t['health']) for t in pt])
    chk("S1.pprof_off_by_default_404",rawreq('GET','/debug/pprof/',{"Authorization":"Bearer "+tok})[0]==404,"HELIX_PPROF_ENABLED unset")
    # S3 CSRF cookie
    st_,hd,_,ck=rawreq('GET','/api/v1/admin/users?limit=1',{"Authorization":"Bearer "+tok})
    c0=[x for x in ck if x.startswith('csrf-dev=') or x.startswith('__Host-csrf=')]
    chk("S3.csrf_cookie_dev_mode",st_==200 and len(c0)==1 and c0[0].startswith('csrf-dev=') and 'Secure' not in c0[0] and 'HttpOnly' in c0[0] and not any(x.startswith('__Host-') for x in ck),{"status":st_,"cookie_attrs":re.sub(r'=[^;]+','=<v>',c0[0]) if c0 else None})
    cv=c0[0].split(';')[0] if c0 else ''; xt=hd.get('X-Csrf-Token') or hd.get('X-CSRF-Token')
    stp,_,bp,_=rawreq('PUT','/api/v1/admin/users/999999',{"Authorization":"Bearer "+tok,"Content-Type":"application/json"},b'{"status":"active","role":"user"}')
    chk("S3.csrf_missing_cookie_403",stp==403,[stp,bp[:80]])
    stp2,_,bp2,_=rawreq('PUT','/api/v1/admin/users/999999',{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv},b'{"status":"active","role":"user"}')
    chk("S3.csrf_missing_header_403",stp2==403,[stp2,bp2[:80]])
    stp3,_,bp3,_=rawreq('PUT','/api/v1/admin/users/999999',{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv,"X-CSRF-Token":"0"*64},b'{"status":"active","role":"user"}')
    chk("S3.csrf_wrong_token_403",stp3==403,[stp3,bp3[:80]])
    stp4,_,bp4,_=rawreq('PUT','/api/v1/admin/users/999999',{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv,"X-CSRF-Token":xt or ""},b'{"status":"active","role":"user"}')
    chk("S3.csrf_valid_pair_passes_guard",stp4!=403 and stp4 in(404,400,200,409),[stp4,bp4[:80]])
    # S4 log redaction
    t0='60s'
    SENT="PWSENTINEL-7391-zq"; ps,_,_,_=rawreq('GET','/api/v1/smb/test?host=127.0.0.1&share=x&username=u&password=%s&pass=%s2&secret=%s3&token=%s4&keep=VISIBLEVAL'%((SENT,)*4),{"Authorization":"Bearer "+tok})
    time.sleep(1); lg=logs(t0)
    line=[l for l in lg.splitlines() if 'smb/test' in l and 'HTTP Request' in l]
    chk("P2g.access_log_has_request_line",len(line)>=1,{"status":ps,"lines":len(line)})
    chk("P2g.sentinel_never_in_api_log",SENT not in lg,"sentinel absent from full api log window")
    chk("P2g.control_noncredential_param_kept",any('keep=VISIBLEVAL' in l for l in line) or any('VISIBLEVAL' in l for l in line),"non-credential param still logged (needle)")
    chk("P2g.redaction_marker_present",any(re.search(r'password=(%5B)?\[?REDACTED|password=\*+|password=%5BREDACTED',l,re.I) for l in line),[l[l.find('smb/test'):][:200] for l in line][:1])
    # S5 cross-user authz
    un="secq_%d"%int(time.time()); pw="Sq-"+ST[:12]+"-Zz9"
    sr,b=lib.req('POST','/api/v1/auth/register',{"username":un,"email":un+"@example.test","password":pw,"first_name":"S","last_name":"Q"})
    uid=(b.get('user') or b).get('id') if isinstance(b,dict) else None
    chk("S5.second_user_registered",sr in(200,201) and uid,[sr,str(b)[:120]])
    if uid:
        _,hd2,_,ck2=rawreq('GET','/api/v1/admin/users?limit=1',{"Authorization":"Bearer "+tok}); cv2=[x for x in ck2 if x.startswith('csrf-dev=')][0].split(';')[0]; xt2=hd2.get('X-Csrf-Token') or hd2.get('X-CSRF-Token')
        pa,_,pb,_=rawreq('PUT','/api/v1/admin/users/%s'%uid,{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv2,"X-CSRF-Token":xt2},json.dumps({"status":"active","role":"user"}).encode())
        sl,jl=lib.login(un,pw); ut=jl.get('session_token') if isinstance(jl,dict) else None
        chk("S5.user_active_and_login",pa==200 and bool(ut),[pa,sl])
        if ut:
            chk("S5.user_metrics_not_200",rawreq('GET','/metrics',{"Authorization":"Bearer "+ut})[0] in(401,403),rawreq('GET','/metrics',{"Authorization":"Bearer "+ut})[0])
            chk("S5.user_admin_users_403",rawreq('GET','/api/v1/admin/users',{"Authorization":"Bearer "+ut})[0]==403)
            chk("S5.user_generate_thumbnails_403",rawreq('POST','/api/v1/admin/covers/generate-thumbnails',{"Authorization":"Bearer "+ut,"Content-Type":"application/json"},b'{}')[0] in(401,403))
            chk("S5.user_cannot_read_admin_conversion_job",all(rawreq('GET','/api/v1/conversion/jobs/%d'%i,{"Authorization":"Bearer "+ut})[0] in(401,403,404) for i in(1,2,150)),"jobs 1,2,150")
            chk("S5.control_admin_admin_users_200",rawreq('GET','/api/v1/admin/users?limit=1',{"Authorization":"Bearer "+tok})[0]==200)
            lo=lib.req('POST','/api/v1/auth/logout',None,ut)[0]; chk("S5.user_logout_revokes_only_own_token",lo==200 and rawreq('GET','/api/v1/auth/profile',{"Authorization":"Bearer "+tok})[0]==200 and rawreq('GET','/api/v1/auth/profile',{"Authorization":"Bearer "+ut})[0] in(401,403),"admin session still valid after user logout (SECQ-6 cross-token)")
        dd=rawreq('DELETE','/api/v1/users/%s'%uid,{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv2,"X-CSRF-Token":xt2})[0]
        chk("S5.cleanup_user",dd in(200,204),dd)
if PH=="B":   # pod restarted with HELIX_PPROF_ENABLED=true
    for p in('/debug/pprof/','/debug/pprof/cmdline','/debug/pprof/heap','/debug/pprof/goroutine?debug=1'):
        chk("S2.pprof_anon_401 "+p,rawreq('GET',p)[0]==401,rawreq('GET',p)[0])
    chk("S2.pprof_scrape_token_rejected",rawreq('GET','/debug/pprof/cmdline',{"Authorization":"Bearer "+ST})[0] in(401,403),"scrape token only unlocks /metrics")
    un="secp_%d"%int(time.time()); pw="Sq-"+ST[:12]+"-Zz9"; sr,b=lib.req('POST','/api/v1/auth/register',{"username":un,"email":un+"@example.test","password":pw,"first_name":"S","last_name":"Q"}); uid=(b.get('user') or b).get('id') if isinstance(b,dict) else None
    _,hd2,_,ck2=rawreq('GET','/api/v1/admin/users?limit=1',{"Authorization":"Bearer "+tok}); cv2=[x for x in ck2 if x.startswith('csrf-dev=')][0].split(';')[0]; xt2=hd2.get('X-Csrf-Token') or hd2.get('X-CSRF-Token')
    rawreq('PUT','/api/v1/admin/users/%s'%uid,{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv2,"X-CSRF-Token":xt2},json.dumps({"status":"active","role":"user"}).encode())
    ut=lib.login(un,pw)[1].get('session_token')
    chk("S2.pprof_plain_user_403",rawreq('GET','/debug/pprof/cmdline',{"Authorization":"Bearer "+ut})[0]==403,rawreq('GET','/debug/pprof/cmdline',{"Authorization":"Bearer "+ut})[0])
    pr=rawreq('GET','/debug/pprof/cmdline',{"Authorization":"Bearer "+tok}); chk("S2.pprof_admin_200",pr[0]==200 and len(pr[2])>0,{"status":pr[0],"bytes":len(pr[2])})
    chk("S2.cleanup_user",rawreq('DELETE','/api/v1/users/%s'%uid,{"Authorization":"Bearer "+tok,"Content-Type":"application/json","Cookie":cv2,"X-CSRF-Token":xt2})[0] in(200,204))
json.dump(res,open("res_%s_%s.json"%(PH,R),"w"),default=str)
print("SUMMARY",PH,R,sum(r['result']=='PASS' for r in res),"PASS",sum(r['result']=='FAIL' for r in res),"FAIL",sum(r['result']=='SKIP' for r in res),"SKIP")
