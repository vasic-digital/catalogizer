import sys,json,time,hashlib,os,uuid,subprocess; sys.path.insert(0,'.')
import lib
R=sys.argv[1]; T=time.strftime('%FT%T%z')
tok=lib.login()[1]['session_token']
res=[]
def chk(name,ok,detail=None,skip=None):
    r={"round":R,"check":name,"result":("SKIP" if skip else ("PASS" if ok else "FAIL")),"detail":detail if not skip else skip}
    res.append(r); print(json.dumps(r,default=lambda o: o.decode("latin1") if isinstance(o,bytes) else str(o))[:700],flush=True)
def jb(src,dst,sf,tf,ct,t=tok):
    return lib.req('POST','/api/v1/conversion/jobs',{"source_path":src,"target_path":dst,"source_format":sf,"target_format":tf,"conversion_type":ct,"quality":"medium","priority":1},t)
def wait(i,t=150,stop=('completed','failed','cancelled')):
    t0=time.time()
    while time.time()-t0<t:
        s,j=lib.req('GET','/api/v1/conversion/jobs/%d'%i,None,tok)
        if isinstance(j,dict) and j.get('status') in stop: return j
        time.sleep(0.5)
    return j
def dl(i): return lib.req('GET','/api/v1/conversion/jobs/%d/download'%i,None,tok,raw=True)
H=lib.req('GET','/health')[1]; chk("env.health_build",H.get('build_number','')[:8]=="015896fd",H.get('build_number'))
# ---- D1 formats
s,f=lib.req('GET','/api/v1/conversion/formats',None,tok)
chk("D1.formats_truthful",s==200 and f['document']['input']==['pdf'] and f['document']['output']==['txt'],f.get('document'))
# D1 pdf->txt
s,j=jb("/media/books/Known Text Document.pdf","/app/data/live3_r%s_pdf.txt"%R,"pdf","txt","document")
if s==200:
    fj=wait(j['id']); ds,body=dl(j['id'])
    txt=body.decode('utf8','replace') if ds==200 else ''
    chk("D1.pdf_to_txt_content",fj['status']=='completed' and ds==200 and 'ZEBRA-4242' in txt and 'Catalogizer PDF fixture page one' in txt and 'Third and final page' in txt and 'NOSUCHTOKEN-0' not in txt,{"status":fj['status'],"dl":ds,"len":len(body),"sha256":hashlib.sha256(body).hexdigest(),"err":fj.get('error_message')})
    jid=j['id']
    # D2 delete completed
    d1=lib.req('DELETE','/api/v1/conversion/jobs/%d'%jid,None,tok)
    g=lib.req('GET','/api/v1/conversion/jobs/%d'%jid,None,tok)[0]
    d2=lib.req('DELETE','/api/v1/conversion/jobs/%d'%jid,None,tok)[0]
    dl2=dl(jid)[0]
    chk("D2.delete_completed",d1[0] in(200,204) and g==404 and d2==404 and dl2==404,{"delete":d1[0],"get_after":g,"delete_again":d2,"download_after":dl2})
else: chk("D1.pdf_to_txt_content",False,[s,j])
# D1 refused pairs
for nm,src,sf,tf,ct,ext in (("pdf_to_png","/media/books/Known Text Document.pdf","pdf","png","document","png"),("txt_to_pdf","/media/books/Plain Notes.txt","txt","pdf","document","pdf"),("epub_to_txt","/media/books/Test Book.epub","epub","txt","document","txt")):
    s,j=jb(src,"/app/data/live3_r%s_%s.%s"%(R,nm,ext),sf,tf,ct)
    chk("D1.refused_"+nm,s==400 and isinstance(j,dict) and bool(j.get('reason') or j.get('error') or j.get('message')),[s,j])
# D10
s,j=jb("/media/music/Misc/Plain Tone.wav","/app/data/live3_r%s_mismatch.png"%R,"wav","mp3","audio")
chk("D10.ext_mismatch_400",s==400,[s,j])
s,j=jb("/media/music/Misc/Plain Tone.wav","/app/data/live3_r%s_ok.mp3"%R,"wav","mp3","audio")
if s==200:
    fj=wait(j['id']); ds,body=dl(j['id'])
    chk("D10.matching_ext_ok",fj['status']=='completed' and ds==200 and (body[:3]==b'ID3' or body[:2]==b'\xff\xfb') and len(body)>1000,{"status":fj['status'],"len":len(body),"magic":body[:3].hex(),"warning":fj.get('warning')})
    lib.req('DELETE','/api/v1/conversion/jobs/%d'%j['id'],None,tok)
else: chk("D10.matching_ext_ok",False,[s,j])
# D11a
def vid(src,nm):
    s,j=jb(src,"/app/data/live3_r%s_%s.webm"%(R,nm),"mp4","webm","video")
    if s!=200: return None,[s,j]
    fj=wait(j['id'],200); return fj,None
fj,e=vid("/media/negative/truncated-tail.mp4","trunc")
chk("D11a.truncated_warns",bool(fj) and fj['status']=='completed' and bool(fj.get('warning')),{"status":fj and fj['status'],"warning":fj and fj.get('warning'),"err":e})
if fj: lib.req('DELETE','/api/v1/conversion/jobs/%d'%fj['id'],None,tok)
fj,e=vid("/media/movies/Movie.Name.2020.1080p.mp4","good")
chk("D11a.control_good_no_warning",bool(fj) and fj['status']=='completed' and not fj.get('warning'),{"status":fj and fj['status'],"warning":fj and fj.get('warning'),"err":e})
if fj: lib.req('DELETE','/api/v1/conversion/jobs/%d'%fj['id'],None,tok)
# D2 running delete
s,j=jb("/media/movies/Big.Seek.Movie.2026.140s.mp4","/app/data/live3_r%s_long.webm"%R,"mp4","webm","video")
if s==200:
    jid=j['id']; wait(jid,20,('running','completed','failed'))
    st=lib.req('GET','/api/v1/conversion/jobs/%d'%jid,None,tok)[1].get('status')
    d=lib.req('DELETE','/api/v1/conversion/jobs/%d'%jid,None,tok)
    if st=='running':
        chk("D2.delete_running_409",d[0]==409,{"status_before":st,"delete":d[0]})
    else: chk("D2.delete_running_409",False,{"status_before":st,"delete":d[0]},skip="job not running at delete time (status %s)"%st)
    lib.req('POST','/api/v1/conversion/jobs/%d/cancel'%jid,None,tok); wait(jid,30)
    lib.req('DELETE','/api/v1/conversion/jobs/%d'%jid,None,tok)
else: chk("D2.delete_running_409",False,[s,j])
# D7 conv error sanitised
s,j=jb("/media/negative/corrupt-garbage.mp3","/app/data/live3_r%s_bad.wav"%R,"mp3","wav","audio")
if s==200:
    fj=wait(j['id']); em=fj.get('error_message') or ''
    chk("D7.error_message_sanitised",fj['status']=='failed' and em!='' and '/app/data' not in em and '/media/' not in em,{"status":fj['status'],"err":em[:200]})
    lib.req('DELETE','/api/v1/conversion/jobs/%d'%j['id'],None,tok)
else: chk("D7.error_message_sanitised",False,[s,j])
# D8
res8={}
for p in ('/api/v1/conversion/jobs/1/cancel','/api/v1/conversion/jobs/1/retry','/api/v1/auth/logout'):
    res8[p]=lib.req('POST',p,b'',None,raw=True,hdr={'Content-Type':'application/json'})[0]
chk("D8.anon_empty_json_body_401",all(v==401 for v in res8.values()),res8)
ctl=lib.req('POST','/api/v1/conversion/jobs/1/cancel',b'{bad',None,raw=True,hdr={'Content-Type':'application/json'})[0]
chk("D8.control_malformed_nonempty_still_rejected",ctl in(400,401),ctl)
# ---- D4
exp={"ZEBRA-4242":"books/Known Text Document.pdf","OKAPI-1357":"books/Plain Notes.txt","QUOKKA-9091":"live3/Live3 Markdown Note.md","NARWHAL-5521":"live3/Live3 Text Note.txt"}
for q,path in exp.items():
    s,b=lib.req('GET','/api/v1/search/files?q='+q,None,tok); fl=b['data']['files'] if s==200 else []
    chk("D4.search_"+q,s==200 and [x['path'] for x in fl]==[path],{"status":s,"paths":[x['path'] for x in fl]})
s,b=lib.req('GET','/api/v1/search/files?q=ZZNOTPRESENT-0000',None,tok)
chk("D4.control_absent_zero",s==200 and len(b['data']['files'])==0,len(b['data']['files']) if s==200 else s)
s,b=lib.req('GET','/api/v1/search/files?q=Known%20Text%20Document',None,tok)
chk("D4.control_name_search",s==200 and any(x['path']=="books/Known Text Document.pdf" for x in b['data']['files']),s)
# ---- D3
s,pp=lib.req('GET','/api/v1/entities/12/pdf-pages',None,tok)
chk("D3.page_count",s==200 and pp.get('total_pages',pp.get('total',pp.get('pages')))==3,[s,pp])
shas=[]
for n in (0,1,2):
    s,b=lib.req('GET','/api/v1/entities/12/pdf-pages/%d'%n,None,tok,raw=True); shas.append((s,b[:8]==b'\x89PNG\r\n\x1a\n',hashlib.sha256(b).hexdigest()[:16],len(b)))
chk("D3.pages_png_distinct",all(a and c for a,c,_,_ in [(x[0]==200,x[1],0,0) for x in shas]) and len({x[2] for x in shas})==3,shas)
chk("D3.page3_404",lib.req('GET','/api/v1/entities/12/pdf-pages/3',None,tok,raw=True)[0]==404)
chk("D3.bad_index_400",all(lib.req('GET','/api/v1/entities/12/pdf-pages/%s'%x,None,tok,raw=True)[0]==400 for x in ('-1','x')))
s,b=lib.req('GET','/api/v1/entities/7/pdf-pages',None,tok,raw=True)
chk("D3.malformed_422_no_leak",s==422 and b'fitz' not in b and b'/app' not in b,[s,b[:160]])
chk("D3.unauth_401",lib.req('GET','/api/v1/entities/12/pdf-pages',None,None,raw=True)[0]==401)
# ---- D11b + subtitles
M=15
sc=lib.req('GET','/api/v1/subtitles/media/%d'%M,None,tok)[1].get('subtitles',[])
side=sorted((x['path'],x['language_code'],x['format']) for x in sc if x['source']=='sidecar')
chk("D11b.sidecars_imported_idempotent",len(side)==4 and len({x[0] for x in side})==4,side)
# upload helper
def up(media,lang,code,fname,content,t=tok):
    bd=uuid.uuid4().hex; parts=[]
    for k,v in (("media_item_id",str(media)),("language",lang),("language_code",code)): parts.append(('--%s\r\nContent-Disposition: form-data; name="%s"\r\n\r\n%s\r\n'%(bd,k,v)).encode())
    parts.append(('--%s\r\nContent-Disposition: form-data; name="file"; filename="%s"\r\nContent-Type: application/octet-stream\r\n\r\n'%(bd,fname)).encode()+content+b'\r\n'); parts.append(('--%s--\r\n'%bd).encode())
    return lib.req('POST','/api/v1/subtitles/upload',b''.join(parts),t,hdr={'Content-Type':'multipart/form-data; boundary='+bd})
sub={f:open('/home/milosvasic/.cache/claude-tmp/media-corpus/movies/'+f,'rb').read() for f in os.listdir('/home/milosvasic/.cache/claude-tmp/media-corpus/movies') if f.endswith(('.srt','.vtt','.ass'))}
ids={}
for tag,f in (("srt","Movie.Name.2020.1080p.en.srt"),("vtt","Movie.Name.2020.1080p.en.vtt"),("ass","Movie.Name.2020.1080p.en.ass"),("oos","Movie.Name.2020.1080p.out-of-sync.srt")):
    s,b=up(M,"English","en","live3_%s_%s"%(R,f),sub[f]); ids[tag]=(b.get('subtitle_id') if isinstance(b,dict) else None)
    chk("D6.upload_"+tag,s==200 and ids[tag] is not None and any(x['id']==str(ids[tag]) for x in lib.req('GET','/api/v1/subtitles/media/%d'%M,None,tok)[1]['subtitles']),[s,{k:b.get(k) for k in('subtitle_id','success')} if isinstance(b,dict) else b])
for tag in ("srt","vtt","ass","oos"):
    s,b=lib.req('GET','/api/v1/subtitles/%s/verify-sync/%d'%(ids[tag],M),None,tok); r=b.get('sync_result',{}) if isinstance(b,dict) else {}
    want=("invalid" if tag=="oos" else "undetermined")
    chk("D5.corpus_media15_honest_"+tag,s==200 and r.get('status')==want and r.get('confidence')==0 and r.get('is_valid') is False and r.get('media_duration')==5,{"http":s,"status":r.get('status'),"reason":r.get('reason'),"conf":r.get('confidence'),"is_valid":r.get('is_valid'),"want_status":want,"note":"corpus audio is a continuous tone (silencedetect 0 events) -> undetermined by design; old tautology returned is_valid true conf 0.8"})
# measured on generated bursty media 201
B=201
s,b=up(B,"English","en","live3_%s_late35.srt"%R,open('burst_late35.srt','rb').read()); late=b.get('subtitle_id') if isinstance(b,dict) else None
sc2={x['path']:x['id'] for x in lib.req('GET','/api/v1/subtitles/media/%d'%B,None,tok)[1]['subtitles'] if x['source']=='sidecar'}
chk("D11b.sidecars_media201",set(sc2)=={'live3/Burst.Movie.2021.en.srt','live3/Burst.Movie.2021.out-of-sync.srt'},sc2)
for nm,sid,want in (("sidecar_aligned",sc2.get('live3/Burst.Movie.2021.en.srt'),0),("sidecar_plus2000",sc2.get('live3/Burst.Movie.2021.out-of-sync.srt'),2000),("upload_plus3500",late,3500)):
    s,b=lib.req('GET','/api/v1/subtitles/%s/verify-sync/%d'%(sid,B),None,tok); r=b.get('sync_result',{}) if isinstance(b,dict) else {}
    chk("D5.measured_"+nm,s==200 and r.get('status')=='measured' and abs((r.get('sync_offset') or 99999)-want)<=300 if want else (s==200 and r.get('status')=='measured' and abs(r.get('sync_offset',99999))<=300),{"http":s,"status":r.get('status'),"offset_ms":r.get('sync_offset'),"conf":r.get('confidence'),"method":r.get('method'),"want":want,"dur":r.get('media_duration')})
if late: lib.req('DELETE','/api/v1/subtitles/%s'%late,None,tok)
chk("D5.verify_unknown_sub_404",lib.req('GET','/api/v1/subtitles/does-not-exist/verify-sync/%d'%M,None,tok)[0]==404)
chk("D5.verify_unknown_media_404",lib.req('GET','/api/v1/subtitles/%s/verify-sync/99999999'%ids['srt'],None,tok)[0]==404)
s,b=up(99999999,"English","en","x.srt",b'1\n00:00:00,000 --> 00:00:01,000\nx\n'); chk("D6.unknown_media_404",s==404,[s,b])
s,b=up(M,"English","en","g.srt",os.urandom(2000)); chk("D6.garbage_422",s==422,[s,b])
chk("D7.sub_list_bad_id_no_err_echo",(lambda r:r[0]==400 and b'strconv' not in json.dumps(r[1]).encode() and b'invalid syntax' not in json.dumps(r[1]).encode())(lib.req('GET','/api/v1/subtitles/media/abc',None,tok)),lib.req('GET','/api/v1/subtitles/media/abc',None,tok))
s,b=lib.req('POST','/api/v1/subtitles/translate',{"subtitle_id":str(ids['srt']),"source_language":"en","target_language":"es"},tok)
chk("D7.translate_no_provider_503",s==503,[s,b])
# delete route
chk("SUBDEL.unauth_401",lib.req('DELETE','/api/v1/subtitles/%s'%ids['srt'],None,None)[0]==401)
dd=[]
for tag in ("srt","vtt","ass","oos"):
    a=lib.req('DELETE','/api/v1/subtitles/%s'%ids[tag],None,tok)[0]; b2=lib.req('DELETE','/api/v1/subtitles/%s'%ids[tag],None,tok)[0]; dd.append((tag,a,b2))
left=[x['id'] for x in lib.req('GET','/api/v1/subtitles/media/%d'%M,None,tok)[1]['subtitles'] if x['id'] in {str(v) for v in ids.values()}]
chk("SUBDEL.delete_204_then_404",all(a==204 and b==404 for _,a,b in dd) and not left,{"dd":dd,"left":left})
# ---- D9
s,u=lib.req('GET','/api/v1/admin/users?limit=50',None,tok)
chk("D9.list_shows_admin",any(x['username']=='qa_admin' and x['role']=='admin' for x in u),[ (x['username'],x['role']) for x in u][:5])
un="live3_r%s_%d"%(R,int(time.time())); pw="Qa!"+os.urandom(6).hex()+"Zz9"
s,b=lib.req('POST','/api/v1/auth/register',{"username":un,"email":un+"@example.test","password":pw,"first_name":"L","last_name":"T"})
uid=b.get('id') if isinstance(b,dict) else None
if uid:
    r1=lib.csrf_req('PUT','/api/v1/admin/users/%s'%uid,{"status":"active","role":"admin"},tok)
    lst=lib.req('GET','/api/v1/admin/users?limit=50',None,tok)[1]
    mine=[x for x in lst if str(x['id'])==str(uid)]
    chk("D9.put_role_admin_applied",r1[0]==200 and mine and mine[0]['role']=='admin',{"put":r1[0],"listed":mine[0]['role'] if mine else None})
    r2=lib.csrf_req('PUT','/api/v1/admin/users/%s'%uid,{"status":"active","role":"nosuchrole"},tok)
    chk("D9.unknown_role_400",r2[0]==400,r2[0])
    ls,lb=lib.req('POST','/api/v1/auth/login',{"username":un,"password":pw}); t2=lb.get('session_token') if isinstance(lb,dict) else None
    if t2:
        r3=lib.csrf_req('PUT','/api/v1/admin/users/%s'%uid,{"status":"active","role":"user"},t2)
        chk("D9.self_demotion_409",r3[0]==409,r3[0])
        # cross-user job isolation
        s,j=jb("/media/music/Misc/Plain Tone.wav","/app/data/live3_r%s_xu.mp3"%R,"wav","mp3","audio"); jid=j['id']; wait(jid)
        xs={"get":lib.req('GET','/api/v1/conversion/jobs/%d'%jid,None,t2)[0],"download":lib.req('GET','/api/v1/conversion/jobs/%d/download'%jid,None,t2,raw=True)[0],"cancel":lib.req('POST','/api/v1/conversion/jobs/%d/cancel'%jid,None,t2)[0],"delete":lib.req('DELETE','/api/v1/conversion/jobs/%d'%jid,None,t2)[0]}
        own=lib.req('GET','/api/v1/conversion/jobs/%d'%jid,None,tok)[1].get('status')
        chk("XUSER.admin2_by_design_access",xs=={"get":200,"download":200,"cancel":409,"delete":200},{"admin2_on_admin1_job":xs,"owner_status_after":own,"note":"service allows non-owners holding view:media (FU-1 documented); delete succeeded so owner row gone"})
        chk("XUSER.nonprivileged_isolation_404",False,skip="no way to grant a plain user conversion permission via API (register -> 403 on every conversion route); isolation 404 unexercised")
        lib.req('DELETE','/api/v1/conversion/jobs/%d'%jid,None,tok)
    else: chk("D9.second_admin_login",False,[ls,lb])
    chk("D9.cleanup_user",lib.csrf_req('DELETE','/api/v1/users/%s'%uid,None,tok)[0] in(200,204),uid)
else: chk("D9.register",False,[s,b])
open('results.jsonl','a').write(''.join(json.dumps(r,default=lambda o: o.decode("latin1") if isinstance(o,bytes) else str(o))+'\n' for r in res))
print("ROUND",R,"PASS",sum(r['result']=='PASS' for r in res),"FAIL",sum(r['result']=='FAIL' for r in res),"SKIP",sum(r['result']=='SKIP' for r in res))
