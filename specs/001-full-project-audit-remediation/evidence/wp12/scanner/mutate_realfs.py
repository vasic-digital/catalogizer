# DEPRECATED (WF22 review T4, 2026-10-08): this round-1 harness writes mutants INTO THE SHARED working tree while other workers edit it (11.4.84) and counts a missing exit
# code ('none', e.g. a run_pinned REFUSED) as KILLED. Do not run it again. Use fix-r2-mutate.py (private copy, fail-closed verdict, negative control first). Kept unchanged
# below as the record of how the round-1 numbers (28/28) were produced; those numbers hold only for the author's own mutants.
import subprocess, hashlib, os, re
ROOT='/home/milosvasic/Projects/catalogizer/catalog-api'
EV='/home/milosvasic/Projects/catalogizer/specs/001-full-project-audit-remediation/evidence/wp12/scanner'
S='/tmp/claude-1000/-home-milosvasic-Projects-catalogizer/80fd5f96-ce40-f6fc-6eb6-74c496826149/scratchpad'
M=[

 ('R5-empty-root-accepted', 'internal/services/generic_scanner.go', 'if rs.total == 0 && !g.o.AllowEmptyRoot {', 'if false && rs.total == 0 && !g.o.AllowEmptyRoot {'),
]
def sha(p): return hashlib.sha256(open(p,'rb').read()).hexdigest()
res=[]
for name,f,old,new in M:
    p=os.path.join(ROOT,f); orig=open(p).read()
    if orig.count(old)!=1: res.append((name,'NOT-APPLICABLE',''));continue
    h0=sha(p)
    try:
        open(p,'w').write(orig.replace(old,new))
        subprocess.run(['bash',S+'/realfs.sh','realfs_mut_'+name.split('-')[0]+'.log'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        out=open(EV+'/realfs_mut_'+name.split('-')[0]+'.log').read()
        rc=re.search(r'realfs_rc=(\d+)',out)
        failing=sorted(set(re.findall(r'^--- FAIL: (\S+)',out,re.M)))
        verdict='KILLED' if (rc and rc.group(1)!='0') else ('BUILD-FAILED' if 'BUILD_FAILED' in out else 'SURVIVED')
        res.append((name,verdict,', '.join(failing)))
    finally:
        open(p,'w').write(orig); assert sha(p)==h0
    print(res[-1],flush=True)
open(EV+'/mutation_results_realfs.txt','w').write('\n'.join('\t'.join(r) for r in res)+'\n')
