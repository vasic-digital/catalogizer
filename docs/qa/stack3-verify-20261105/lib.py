import os,json,urllib.request,urllib.error
QA=os.path.expanduser("~/.cache/claude-tmp/catqa3")
def envf(p):
    d={}
    for l in open(p):
        if "=" in l and not l.startswith("#"):
            k,v=l.rstrip("\n").split("=",1); d[k]=v
    return d
E=envf(QA+"/qa.env"); B=os.environ.get("API","http://127.0.0.1:38082")
def req(m,p,body=None,tok=None,raw=False,hdr=None,base=None):
    h={"Content-Type":"application/json"}; h.update(hdr or {})
    if tok: h["Authorization"]="Bearer "+tok
    r=urllib.request.Request((base or B)+p,data=((body if isinstance(body,bytes) else json.dumps(body).encode()) if body is not None else None),method=m,headers=h)
    try:
        x=urllib.request.urlopen(r,timeout=60); s=x.status; b=x.read(); hd=dict(x.headers)
    except urllib.error.HTTPError as e: s=e.code; b=e.read(); hd=dict(e.headers)
    if raw: return s,b
    try: return s,json.loads(b)
    except Exception: return s,b.decode('utf8','replace')
def login(u=None,p=None):
    return req("POST","/api/v1/auth/login",{"username":u or E["ADMIN_USERNAME"],"password":p or E["ADMIN_PASSWORD"]})
