import os,sys,json,hashlib,urllib.request,urllib.error

from seclib import req as _req, login, E, B   # same helpers as the security harness
def req(m,p,body=None,tok=None,raw=False,hdr=None):
    return _req(m,p,body,tok,raw,hdr)
def sha(b): return hashlib.sha256(b).hexdigest()
def csrf_req(m,p,body=None,tok=None):
    r=urllib.request.Request(B+'/api/v1/admin/users?limit=1',headers={"Authorization":"Bearer "+tok})
    x=urllib.request.urlopen(r,timeout=30); ck=[c.split(';')[0] for c in x.headers.get_all('Set-Cookie') if c.startswith('csrf-dev=')][0]; xt=x.headers.get('X-CSRF-Token')
    return _req(m,p,body,tok,False,{"Cookie":ck,"X-CSRF-Token":xt})
