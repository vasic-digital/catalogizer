# DEPRECATED (WF22 review T4, 2026-10-08): this round-1 harness writes mutants INTO THE SHARED working tree while other workers edit it (11.4.84) and counts a missing exit
# code ('none', e.g. a run_pinned REFUSED) as KILLED. Do not run it again. Use fix-r2-mutate.py (private copy, fail-closed verdict, negative control first). Kept unchanged
# below as the record of how the round-1 numbers (28/28) were produced; those numbers hold only for the author's own mutants.
import sys, json, subprocess, hashlib, shutil, os, re
ROOT='/home/milosvasic/Projects/catalogizer/catalog-api'
EV='/home/milosvasic/Projects/catalogizer/specs/001-full-project-audit-remediation/evidence/wp12/scanner'
GS='internal/services/generic_scanner.go'
ST='filesystem/settings.go'
PF='filesystem/webdav_propfind.go'
US='internal/services/universal_scanner.go'
FA='filesystem/factory.go'
M=[
 ('M01-scanner-body-empty-again (the original stub)', GS, 'func (g *GenericScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {\n', 'func (g *GenericScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {\n\tif true {\n\t\treturn nil\n\t}\n'),
 ('M02-empty-root-accepted', GS, 'if rs.total == 0 && !g.o.AllowEmptyRoot {', 'if false && rs.total == 0 && !g.o.AllowEmptyRoot {'),
 ('M03-unreadable-root-swallowed', GS, 'return &ScanError{Kind: FailRootUnreadable, Path: root, Err: err}\n\t}\n\n\tnext, err', 'return nil\n\t}\n\n\tnext, err'),
 ('M04-readonly-decorator-removed', GS, 'out := decorators.ReadOnly(c)', 'out := c\n	_ = decorators.ReadOnly'),
 ('M05-host-budget-not-applied', GS, 'out = fabric.Limited(out, b)', '_ = b'),
 ('M06-every-list-error-benign', GS, 'func isBenignListError(err error) bool {\n\tif err == nil {\n\t\treturn false\n\t}', 'func isBenignListError(err error) bool {\n\tif err != nil {\n\t\treturn true\n\t}\n\tif err == nil {\n\t\treturn false\n\t}'),
 ('M07-depth-off-by-one', GS, 'if e.IsDir && depth+1 <= rs.maxDepth {', 'if e.IsDir && depth+1 <= rs.maxDepth+1 {'),
 ('M08-token-ignores-size', GS, 'fmt.Fprintf(&b, "m=%d;s=%d", t.MTimeNano, t.Size)', 'fmt.Fprintf(&b, "m=%d;s=%d", t.MTimeNano, int64(0))'),
 ('M09-incremental-skips-everything-known', GS, 'old == tok {', 'old != "" {'),
 ('M10-entries-unsorted', GS, 'return sorted[i].Name < sorted[j].Name', 'return false'),
 ('M11-nfs-export-key-reverted', ST, 'case "nfs":\n\t\tput(KeyHost, root.Host)\n\t\tput(KeyPath, root.Path)', 'case "nfs":\n\t\tput(KeyHost, root.Host)\n\t\tput("export_path", root.Path)'),
 ('M12-unknown-keys-accepted', ST, 'if !known {\n\t\t\tif hint', 'if !known && false {\n\t\t\tif hint'),
 ('M13-ftp-path-not-forwarded', ST, 'put(KeyUsername, root.Username)\n\t\tput(KeyPassword, root.Password)\n\t\tput(KeyPath, root.Path)\n\tcase "nfs":', 'put(KeyUsername, root.Username)\n\t\tput(KeyPassword, root.Password)\n\tcase "nfs":'),
 ('M14-propfind-self-not-skipped', PF, 'if hp == self {', 'if hp == self && false {'),
 ('M15-failed-propstat-trusted', PF, 'if !davStatusOK(ps.Status) {', 'if false {'),
 ('M16-cancel-reported-as-failed', US, 'status.cancel(err)', 'status.fail(err)'),
 ('M17-all-records-failing-still-completes', GS, 'if rs.recordedOK == 0 && rs.recordFails > 0 {', 'if false {'),
 ('M18-per-dir-bound-removed', GS, 'if len(entries) > rs.g.o.MaxEntriesPerDir {', 'if false {'),
 ('M19-factory-skips-validation', FA, 'if err := ValidateSettings(config.Protocol, config.Settings); err != nil {\n\t\treturn nil, err\n\t}', '_ = ValidateSettings'),
 ('M20-registered-protocol-not-resolved-on-demand', US, 'if !exists && filesystem.IsRegisteredProtocol(job.StorageRoot.Protocol) {', 'if false {'),
 ('M21-hostile-names-followed', GS, 'return n != "" && n != "." && n != ".." && !strings.ContainsAny(n, "/\\x00")', 'return true'),
 ('M22-skip-dirs-ignored', GS, 'if e.IsDir && rs.skip[e.Name] {', 'if false {'),
]
M.append(('M00-negative-control-no-change', GS, 'package services', 'package services'))
only=sys.argv[1:] 
RUN=r"""./internal/services/ ./filesystem/ -run 'TestSettingsContract|TestValidateSettings|TestFactory_|TestRegisterProtocol|TestSettingsFromRoot|TestStorageRootToSettings|GenericScanner|ChangeToken|HostKey|IsBenign|ScanFailureKinds|PluggableScanner|StubScanners|TestScanJob_|ParsePropfind|DavStatus|ListDirectory' -count=1"""
def sha(p): return hashlib.sha256(open(p,'rb').read()).hexdigest()
def run(tag):
    log=f'/dev/shm/mut_{tag}.log'
    cmd=f"cd /home/milosvasic/Projects/catalogizer; export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1; bash scripts/containers/run_pinned.sh IMG-GO -- bash -c \"cd catalog-api && set -o pipefail; env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1 go test {RUN} 2>&1 | grep -v '^go: downloading'; echo go_test_rc=\\${{PIPESTATUS[0]}}\" > {log} 2>&1"
    subprocess.run(['bash','-c',cmd])
    out=open(log).read()
    m=re.search(r'go_test_rc=(\d+)',out)
    return (m.group(1) if m else 'none'), out
res=[]
for name,f,old,new in M:
    tag=name.split('-')[0]
    if only and tag not in only: continue
    p=os.path.join(ROOT,f)
    orig=open(p).read()
    if orig.count(old)!=1:
        res.append((name,'NOT-APPLICABLE count=%d'%orig.count(old),'')); continue
    h0=sha(p)
    try:
        open(p,'w').write(orig.replace(old,new))
        rc,out=run(tag)
        failing=sorted(set(re.findall(r'^--- FAIL: (\S+)',out,re.M)))[:4]
        verdict='KILLED' if rc not in ('0',) else 'SURVIVED'
        if 'build failed' in out or '[build failed]' in out: verdict='BUILD-BROKEN(not counted)'
        res.append((name,verdict+' rc='+rc,', '.join(failing)))
    finally:
        open(p,'w').write(orig)
        assert sha(p)==h0, 'restore mismatch '+p
    print(res[-1],flush=True)
with open(EV+'/mutation_results_'+('_'.join(only) if only else 'all')+'.txt','w') as fh:
    for r in res: fh.write('\t'.join(r)+'\n')
