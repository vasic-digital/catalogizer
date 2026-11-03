import json,glob,collections,sys
rows=[]
for f in glob.glob('../out/verdicts-*.jsonl'):
    rows+= [json.loads(l) for l in open(f)]
print('total cases',len(rows),collections.Counter(r['status'] for r in rows))
by=collections.defaultdict(lambda:collections.Counter())
for r in rows: by[(r['app'],r['browser'])][r['status']]+=1
for k,v in sorted(by.items()): print(k,dict(v))
ft=collections.defaultdict(collections.Counter)
for r in rows:
    for t in r.get('failTypes',[]): ft[(r['app'],t)][r['page']]+=1
for k,v in sorted(ft.items()): print(k,sum(v.values()),dict(v.most_common(6)))
json.dump({'total':len(rows),'status':collections.Counter(r['status'] for r in rows)},open('../out/summary.json','w'))
