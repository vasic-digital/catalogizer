#!/usr/bin/env python3
"""WF22 round-3 mutation harness (adopts the independent reviewer's round-2 mutants K01..K21 and adds author mutants X01..).

Mutates ONLY a private copy of catalog-api (default .audit/scratch/wf22r3/mut/catalog-api), never the working tree. Each run is rootless via
scripts/containers/run_pinned.sh IMG-GO, one container at a time. Verdict is fail-closed:
  KILLED   = go_rc != 0 AND (a '--- FAIL:' line OR a test-binary panic) AND no build/setup failure marker
  SURVIVED = go_rc == 0
  ERROR    = anything else (build failure of the mutant, container refusal, no verdict)
A negative control (no change) runs first for every package set and must be go_rc=0.

usage: mutate_r3.py neg | all | K01,K05,X03 ...      env: WORK (copy dir relative to repo root), OUT (results tsv)
"""
import os, re, subprocess, sys, time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
WORK = os.environ.get('WORK', '.audit/scratch/wf22r3/mut/catalog-api')
CP = os.path.join(REPO, WORK) + '/'
RT = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'run_go.sh')
RESULTS = os.environ.get('OUT', os.path.join(REPO, '.audit/scratch/wf22r3/mutation_results_r3.tsv'))
SLOW = 'TestFTPClient_Connect_WithContext|TestFTPClient_Connect_InvalidServer|TestSmbClient_Connect_InvalidServer'
SVC, FS, HD, IH, DB, MD = './internal/services/', './filesystem/', './handlers/', './internal/handlers/', './database/', './models/'
GS = 'internal/services/generic_scanner.go'; US = 'internal/services/universal_scanner.go'; SR = 'internal/services/scan_redact.go'
SH = 'handlers/scan_handler.go'; PF = 'filesystem/webdav_propfind.go'; WC = 'filesystem/webdav_client.go'; FC = 'filesystem/ftp_client.go'
ST = 'filesystem/settings.go'; FA = 'filesystem/factory.go'; EB = 'internal/handlers/eventbus_bridge.go'; V21 = 'database/migrations_v21_storage_root_allow_empty.go'
NFS = 'filesystem/nfs_client.go'; LC = 'filesystem/local_client.go'; SM = 'filesystem/smb_client.go'; CA = 'internal/services/cover_art_service.go'
MF = 'models/file.go'; CF = 'filesystem/confine.go'
FSALL = [FS, SVC, HD]
M = [
 # ---- the reviewer's round-2 mutants (K01 retargeted to the row validator that replaced the vacuous call)
 ('K01', SH, 'if err := filesystem.ValidateRoot(candidate); err != nil {', 'if err := error(nil); err != nil {\n\t\t_ = candidate', [HD]),
 ('K02', GS, '\tif r.URL != nil {\n\t\tif u, err := url.Parse(*r.URL)', '\tif false && r.URL != nil {\n\t\tif u, err := url.Parse(*r.URL)', [SVC]),
 ('K03', GS, 'rs.countable(entries) <= remaining {', 'rs.countable(entries) < remaining {', [SVC]),
 ('K04', GS, '"status 403", "status 404"}', '"status 403"}', [SVC]),
 ('K05', GS, 'return te.Code == 550 || te.Code == 553', 'return te.Code == 550', [SVC]),
 ('K06', GS, 'if r := recover(); r != nil {\n\t\t\t\tch <- answer', 'if r := interface{}(nil); r != nil {\n\t\t\t\tch <- answer', [SVC]),
 ('K07', PF, '\tif b.n >= b.max {', '\tif b.n > b.max {', FSALL),
 ('K08', PF, '\t\tp := u.Path\n\t\tif p == "" {\n\t\t\tp = href\n\t\t}\n\t\treturn strings.TrimRight(p, "/")', '\t\tp := u.Path\n\t\tif p == "" {\n\t\t\tp = href\n\t\t}\n\t\treturn p', FSALL),
 ('K09', WC, '\tif !strings.HasSuffix(fullURL, "/") {\n\t\tfullURL += "/"\n\t}', '\tif false && !strings.HasSuffix(fullURL, "/") {\n\t\tfullURL += "/"\n\t}', FSALL),
 ('K10', FC, 'if cwd, err := client.CurrentDir(); err == nil && strings.HasPrefix(cwd, "/") {', 'if cwd, err := client.CurrentDir(); err == nil && (strings.HasPrefix(cwd, "/") || true) {', FSALL),
 ('K11', FC, '\t\t\tc.base = pathpkg.Clean("/" + c.config.Path)', '\t\t\tc.base = c.config.Path', FSALL),
 ('K12', ST, '\tif len(spec.Keys) == 0 {', '\tif false && len(spec.Keys) == 0 {', FSALL),
 ('K13', ST, 'for _, k := range []string{KeyCredentialRef, KeyTLSMode, KeyHostKeyFingerprint, KeyCertSHA256} {', 'for _, k := range []string{KeyCredentialRef, KeyTLSMode, KeyHostKeyFingerprint} {', FSALL),
 ('K14', US, '\t\tpayload["reason"] = snapshot.Reason\n', '', [SVC]),
 ('K15', EB, '\t\teventbus.EventScanCancelled,\n\t\teventbus.EventScanStarted,', '\t\teventbus.EventScanStarted,', [IH]),
 ('K16', SR, 'out = append(out, s, url.QueryEscape(s), url.PathEscape(s), strings.TrimPrefix(url.UserPassword("", s).String(), ":"))', 'out = append(out, s)\n\t\t\t_ = url.QueryEscape', [SVC]),
 ('K17', SR, '`(?i)([a-z][a-z0-9+.\\-]*://)', '`([a-z][a-z0-9+.\\-]*://)', [SVC]),
 ('K18', GS, '\trs.listed++\n', '', [SVC]),
 ('K19', GS, '\t\t\trs.status.incrementCounters(1, 0, 0, 0, 0)\n', '', [SVC]),
 ('K20', V21, '\tif !has {\n\t\tif _, err := db.ExecContext(ctx, `ALTER TABLE storage_roots ADD COLUMN allow_empty', '\tif !has || true {\n\t\tif _, err := db.ExecContext(ctx, `ALTER TABLE storage_roots ADD COLUMN allow_empty', [DB]),
 ('K21', FA, 'Port:     getIntSetting(config.Settings, "port", 21),', 'Port:     getIntSetting(config.Settings, "port", 2121),', FSALL),
 # ---- author mutants of the round-3 structural work
 ('X01', FC, '\t\treturn JoinConfined(c.base, p)', '\t\treturn pathpkg.Join(c.base, p)', [FS]),
 ('X02', NFS, '\treturn filepath.Join(c.mountPoint, filepath.FromSlash(ConfinePath(path)))', '\treturn filepath.Join(c.mountPoint, strings.ReplaceAll(filepath.Clean(path), "..", ""))', [FS]),
 ('X03', LC, '\treturn filepath.Join(c.basePath, filepath.FromSlash(ConfinePath(filepath.ToSlash(path))))', '\treturn filepath.Join(c.basePath, path)', [FS]),
 ('X04', US, '\tjob.Path = ConfineScanPath(job.Path)\n', '', [SVC]),
 ('X05', GS, '\troot := filesystem.ConfinePath(job.Path)', '\troot := job.Path\n\tif root == "" {\n\t\troot = "/"\n\t}', [SVC]),
 ('X06', MF, '\tp.Password = nil\n', '', [MD, HD]),
 ('X07', MF, '\tp.URL = r.RedactedURL()\n', '', [MD, HD]),
 ('X08', SR, 'return zap.String("error", scrubSecrets(err.Error(), secrets))', 'return zap.String("error", err.Error())', [SVC]),
 ('X09', SR, '\tif _, pw, _ := ResolveSMBIdentity(root); pw != "" {\n\t\tadd(pw)\n\t}\n', '', [SVC]),
 ('X10', ST, '\tif len(unused) > 0 {\n\t\tsort.Strings(unused)', '\tif false && len(unused) > 0 {\n\t\tsort.Strings(unused)', [FS, HD]),
 ('X11', ST, '\t\tif !allowed[k] {\n\t\t\tunknown = append(unknown, k)\n\t\t\tcontinue\n\t\t}', '\t\tif !allowed[k] {\n\t\t\tcontinue\n\t\t}', [FS, HD]),
 ('X12', CA, 'filesystem.SettingsFromRoot(t.storageRoot(), ResolveSMBIdentity)', 'filesystem.SettingsFromRoot(t.storageRoot(), nil)', [SVC]),
 ('X13', NFS, '\tfor _, line := range strings.Split(mounts, "\\n") {', '\tfor _, line := range strings.Split("x x nfs\\n", "\\n") {', [FS]),
 ('X14', 'internal/services/generic_scanner.go', 'return net.JoinHostPort(h, strconv.Itoa(*r.Port))', 'return fmt.Sprintf("%s:%d", h, *r.Port)', [SVC]),
 ('X15', SM, '\tif !hasDotDotSegment(p) {\n\t\treturn p\n\t}\n', '', [FS]),
 ('X16', ST, '\t\tif !present[k] {\n\t\t\tmissing = append(missing, k)\n\t\t}', '\t\t_ = k', [FS, HD]),
]
BUILD = re.compile(r'\[build failed\]|\[setup failed\]|run_pinned: REFUSED|^\S+\.go:\d+:\d+: ', re.M)

def run(tag, pkgs):
    log = '/dev/shm/r3mut_%s.log' % tag
    skip = SLOW + '|TestConfigurationWizardService_FinalizeConfiguration'
    out = ''
    for attempt in range(6):
        subprocess.run([RT, log, "test -p 1 -count=1 -skip '%s' %s" % (skip, ' '.join(pkgs)), '2'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       timeout=3000, env=dict(os.environ, WORK=WORK))
        out = open(log, errors='replace').read()
        if 'REFUSED reason=memory' in out or 'REFUSED reason=disk' in out or 'signal: killed' in out:
            time.sleep(60); continue
        break
    m = re.findall(r'^go_rc=(\d+)\s*$', out, re.M)
    return (m[-1] if m else None), out

def verdict(rc, out):
    if rc == '0': return 'SURVIVED'
    if rc is not None and rc != '0' and not BUILD.search(out) and (re.search(r'^\s*--- FAIL:', out, re.M) or re.search(r'^panic: ', out, re.M)):
        return 'KILLED'
    return 'ERROR'

def main():
    arg = sys.argv[1] if len(sys.argv) > 1 else 'all'
    res = open(RESULTS, 'a')
    if arg == 'neg':
        for pk in [[FS], [SVC], [HD], [IH], [DB], [MD]]:
            rc, out = run('NEG_' + pk[0].strip('./').replace('/', '_'), pk)
            res.write('NEG\t%s\trc=%s\n' % (pk[0], rc)); res.flush(); print('NEG', pk, rc, flush=True)
        return
    only = None if arg == 'all' else arg.split(',')
    for mid, f, old, new, pkgs in M:
        if only and mid not in only: continue
        p = CP + f
        src = open(p).read()
        n = src.count(old)
        if n != 1:
            res.write('%s\tNOT-APPLICABLE(count=%d)\n' % (mid, n)); res.flush(); print(mid, 'NOT-APPLICABLE', n, flush=True); continue
        open(p, 'w').write(src.replace(old, new, 1))
        try:
            t = time.time(); rc, out = run(mid, pkgs)
        finally:
            open(p, 'w').write(src)
        v = verdict(rc, out)
        fails = re.findall(r'^\s*--- FAIL: (\S+)', out, re.M)[:4]
        res.write('%s\t%s\trc=%s\t%ds\t%s\t%s\n' % (mid, v, rc, time.time() - t, ' '.join(pkgs), ','.join(fails))); res.flush()
        print(mid, v, rc, fails, flush=True)

main()
