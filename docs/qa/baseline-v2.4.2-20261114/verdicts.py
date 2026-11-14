#!/usr/bin/env python3
# usage: verdicts.py report.json  -> TSV test_id<TAB>verdict on stdout (harness-derived from Playwright JSON)
import json,sys,re
r=json.load(open(sys.argv[1])); rows=[]
M={'passed':'PASS','failed':'FAIL','timedOut':'FAIL','skipped':'SKIPPED','interrupted':'ERROR'}
def walk(s,path):
    p=path+([s['title']] if s.get('title') and not s['title'].endswith('.ts') else [])
    for x in s.get('suites',[]): yield from walk(x,p)
    for sp in s.get('specs',[]):
        for t in sp['tests']:
            res=t['results'][-1] if t['results'] else None
            st=M.get(res['status'],'ERROR') if res else 'ERROR'
            if t.get('status')=='flaky': st='ERROR'
            tid=t['projectName']+'::'+sp['file']+'::'+' > '.join(p)+' > '+sp['title']
            yield re.sub(r'[^A-Za-z0-9._:/>-]+','_',tid),st
for s in r['suites']: rows+=list(walk(s,[]))
ids=[a for a,_ in rows]; assert len(ids)==len(set(ids)),"duplicate ids"
for a,b in sorted(rows): print(f"{a}\t{b}")
