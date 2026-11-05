import sys,json,time; sys.path.insert(0,'.')
import lib
tok=lib.login()[1]['session_token']
s,b=lib.req('POST','/api/v1/scans',{"storage_root_id":int(sys.argv[1])},tok); jid=b['job_id']
for i in range(60):
    s,b=lib.req('GET','/api/v1/scans/'+jid,None,tok)
    if b.get('status') in('completed','failed'): print(b.get('status'),b.get('files_found'),b.get('error_count')); break
    time.sleep(3)
