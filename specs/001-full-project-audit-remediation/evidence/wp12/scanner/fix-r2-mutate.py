#!/usr/bin/env python3
"""fix-r2-mutate.py - WF22 fix round 2 mutation harness (replaces mutate_unit.py / mutate_realfs.py, review finding T4).

What changed against the author's harness:
  * it mutates a PRIVATE COPY of the tree (<repo>/.audit/scratch/wf22fix/mut/{catalog-api,submodules/filesystem}), never the shared working tree
    other workers are editing (11.4.84); the sha256 of every file of catalog-api and submodules/filesystem it can read is recorded before and after: catalog-api must be unchanged, changes by other workers under submodules/filesystem during the run are counted and reported;
  * the verdict is FAIL-CLOSED: KILLED only when the build succeeded AND `go test` exited non-zero AND at least one `--- FAIL:` line exists AND no
    build/setup-failure marker is present. SURVIVED only for exit 0. Everything else (no exit-code line, a run_pinned REFUSED, a build error, a
    timeout) is ERROR and is neither killed nor survived: the mutant is re-run or reported as ERROR, never counted as a kill;
  * a negative control (no change) runs first and MUST be exit 0, else the harness stops;
  * a mutation whose `old` text is not found exactly once is NOT-APPLICABLE and aborts the run (a stale mutant list is a harness defect).
usage: fix-r2-mutate.py [--list] [--realfs] [--only ID[,ID...]] [--out FILE]
"""
import hashlib, os, re, shutil, subprocess, sys, time

REPO = '/home/milosvasic/Projects/catalogizer'
EV = REPO + '/specs/001-full-project-audit-remediation/evidence/wp12/scanner'
SCR = REPO + '/.audit/scratch/wf22fix'
MUT = SCR + '/mut'
RT = SCR + '/rt.sh'

GS = 'internal/services/generic_scanner.go'
US = 'internal/services/universal_scanner.go'
CA = 'internal/services/cover_art_service.go'
ST = 'filesystem/settings.go'
FA = 'filesystem/factory.go'
PF = 'filesystem/webdav_propfind.go'
WC = 'filesystem/webdav_client.go'
FC = 'filesystem/ftp_client.go'
SH = 'handlers/scan_handler.go'
MD = 'models/file.go'
MG = 'database/migrations.go'
# three tests of the filesystem package only wait for connect timeouts against an invalid server (20 s + 20 s + 10 s); they exercise no mutated code
SLOW = 'TestFTPClient_Connect_WithContext|TestFTPClient_Connect_InvalidServer|TestSmbClient_Connect_InvalidServer'
SVC, FS, HD, DB = './internal/services/', './filesystem/', './handlers/', './database/'

# (id, file, old, new, packages)
M = [
 # --- scanner core (author M01-M22 ported + reviewer A-set ported)
 ('M01-scanner-body-empty-again', GS, 'func (g *GenericScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {\n', 'func (g *GenericScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {\n\tif true {\n\t\treturn nil\n\t}\n', [SVC]),
 ('M02-empty-root-accepted', GS, 'if rs.total == 0 && !rs.allowEmpty {', 'if false && rs.total == 0 && !rs.allowEmpty {', [SVC]),
 ('M03-unreadable-root-swallowed', GS, 'return &ScanError{Kind: FailRootUnreadable, Path: root, Err: err}\n\t}\n\n\tnext, err', 'return nil\n\t}\n\n\tnext, err', [SVC]),
 ('M04-readonly-decorator-removed', GS, 'out := decorators.ReadOnly(c)', 'out := c\n\t_ = decorators.ReadOnly', [SVC]),
 ('M05-host-budget-not-applied', GS, 'out = fabric.Limited(out, b)', '_ = b', [SVC]),
 ('M06-every-list-error-benign', GS, 'func isBenignListError(err error) bool {\n\tif err == nil {\n\t\treturn false\n\t}', 'func isBenignListError(err error) bool {\n\tif err != nil {\n\t\treturn true\n\t}\n\tif err == nil {\n\t\treturn false\n\t}', [SVC]),
 ('M07-depth-off-by-one', GS, 'if r.t.depth > rs.maxDepth {', 'if r.t.depth > rs.maxDepth+1 {', [SVC]),
 ('M08-token-ignores-size', GS, 'fmt.Fprintf(&b, "m=%d;s=%d", t.MTimeNano, t.Size)', 'fmt.Fprintf(&b, "m=%d;s=%d", t.MTimeNano, int64(0))', [SVC]),
 ('M09-incremental-skips-everything-known', GS, 'old == tok {', 'old != "" {', [SVC]),
 ('M10-entries-unsorted', GS, 'return sorted[i].Name < sorted[j].Name', 'return false', [SVC]),
 ('M16-cancel-reported-as-failed', US, 'status.cancel(err)', 'status.fail(err)', [SVC]),
 ('M17-record-failures-never-fail-the-scan', GS, 'rs.recordFails > 0 && float64(rs.recordFails) > g.o.MaxRecordFailRatio*float64(n)', 'false', [SVC]),
 ('M18-per-dir-bound-removed', GS, 'if len(entries) > rs.g.o.MaxEntriesPerDir {', 'if false {', [SVC]),
 ('M20-registered-protocol-not-resolved-on-demand', US, 'if !exists && filesystem.IsRegisteredProtocol(job.StorageRoot.Protocol) {', 'if false {', [SVC]),
 ('M21-hostile-names-followed', GS, 'if n == "" || n == "." || n == ".." || len(n) > maxEntryNameBytes {', 'if false {', [SVC]),
 ('M22-skip-dirs-ignored', GS, 'if e.IsDir && rs.skip[e.Name] {\n\t\t\tcontinue', 'if false {\n\t\t\tcontinue', [SVC]),
 ('A01-default-depth-1', GS, 'maxDepth = defaultScanDepth', 'maxDepth = 1', [SVC]),
 ('A03-total-bound-off-by-one', GS, 'if rs.total > rs.g.o.MaxEntries {', 'if rs.total > rs.g.o.MaxEntries+1 {', [SVC]),
 ('A04-perdir-bound-ge', GS, 'if len(entries) > rs.g.o.MaxEntriesPerDir {', 'if len(entries) >= rs.g.o.MaxEntriesPerDir {', [SVC]),
 ('A05-nul-name-accepted', GS, "if r == '/' || r < 0x20 || r == 0x7f {", "if r == '/' || (r < 0x20 && r != 0) || r == 0x7f {", [SVC]),
 ('A05b-control-chars-accepted', GS, "if r == '/' || r < 0x20 || r == 0x7f {", "if r == '/' || r == 0x7f {", [SVC]),
 ('A05c-name-length-unbounded', GS, ' || len(n) > maxEntryNameBytes {', ' {', [SVC]),
 ('A06-dotdot-name-accepted', GS, 'if n == "" || n == "." || n == ".." ||', 'if n == "" || n == "." ||', [SVC]),
 ('A07-token-stored-for-failed-record', GS, '\t\t\trs.recordFails++\n', '\t\t\trs.recordFails++\n\t\t\tif rs.g.o.Tokens != nil {\n\t\t\t\trs.g.o.Tokens.Put(rs.rootKey, full, tok)\n\t\t\t}\n', [SVC]),
 ('A08-token-ignores-kind', GS, '\tif t.IsDir {\n\t\tb.WriteString(";d=1")', '\tif false && t.IsDir {\n\t\tb.WriteString(";d=1")', [SVC]),
 ('A09-token-ignores-etag', GS, '\tif t.ETag != "" {', '\tif false && t.ETag != "" {', [SVC]),
 ('A10-token-ignores-client-extras', GS, '\tif src != nil {\n\t\tt.ETag, t.Inode', '\tif false && src != nil {\n\t\tt.ETag, t.Inode', [SVC]),
 ('A11-full-scan-skips-unchanged', GS, 'rs.incremental = job.ScanType == "incremental" && g.o.Tokens != nil', 'rs.incremental = g.o.Tokens != nil', [SVC]),
 ('A13-no-per-entry-cancel-check', GS, '\t\tif err := ctx.Err(); err != nil {\n\t\t\treturn nil, &ScanError{Kind: ctxKind(ctx), Path: dir, Err: err}', '\t\tif err := ctx.Err(); err != nil && false {\n\t\t\treturn nil, &ScanError{Kind: ctxKind(ctx), Path: dir, Err: err}', [SVC]),
 ('A14-no-wave-cancel-check', GS, '\t\tif ctx.Err() != nil {\n\t\t\tresults[i].err = ctx.Err()', '\t\tif false && ctx.Err() != nil {\n\t\t\tresults[i].err = ctx.Err()', [SVC]),
 ('A15-benign-errors-is-branch-removed', GS, 'if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {\n\t\treturn true', 'if false && (errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist)) {\n\t\treturn true', [SVC]),
 ('A16-benign-textproto-branch-removed', GS, 'if errors.As(err, &te) {\n\t\treturn te.Code == 550', 'if false && errors.As(err, &te) {\n\t\treturn te.Code == 550', [SVC]),
 ('A18-workers-unbounded', GS, 'for i := 0; i < len(level); i += g.o.Workers {\n\t\t\tend := i + g.o.Workers', 'for i := 0; i < len(level); i += len(level) {\n\t\t\tend := i + len(level)', [SVC]),
 ('A20-skip-dirs-only-at-root', GS, 'if e.IsDir && rs.skip[e.Name] {\n\t\t\tcontinue', 'if e.IsDir && rs.skip[e.Name] && depth == 0 {\n\t\t\tcontinue', [SVC]),
 ('A22-production-retry-disabled', GS, 'pol := fabric.RetryPolicy{MaxAttempts: 3,', 'pol := fabric.RetryPolicy{MaxAttempts: 1,', [SVC]),
 ('A23-production-nas-budget-unbounded', GS, '\t\to.Budget = fabric.NASBudget()', '\t\to.Budget = fabric.BudgetConfig{MaxConcurrent: 1000, MaxReqPerSec: 1000000}', [SVC]),
 ('A27-tokens-shared-across-roots', GS, '\t\trootKey = job.StorageRoot.Name', '\t\trootKey = ""', [SVC]),
 ('A29-sort-reversed', GS, 'return sorted[i].Name < sorted[j].Name', 'return sorted[i].Name > sorted[j].Name', [SVC]),
 ('B03-any-ctx-error-is-cancelled', US, ' && !errors.Is(job.Context.Err(), context.DeadlineExceeded) {', ' {', [SVC]),
 ('B03r-any-failure-after-cancel-is-cancelled', US, 'if job.Context != nil && job.Context.Err() != nil && errors.Is(err, job.Context.Err())', 'if job.Context != nil && job.Context.Err() != nil && (errors.Is(err, job.Context.Err()) || true)', [SVC]),
 ('B05r-smb-join-concat', US, 'fullPath := pathpkg.Join(path, file.Name) // a remote path', 'fullPath := path + "/" + file.Name\n\t\t\t_ = pathpkg.Join // a remote path', [SVC]),
 ('B06-ftp-scanner-not-wired-to-db', US, 'NewFTPScanner(logger).WithDB(db)', 'NewFTPScanner(logger)', [SVC]),
 ('B07-webdav-scanner-not-wired-to-db', US, 'NewWebDAVScanner(logger).WithDB(db)', 'NewWebDAVScanner(logger)', [SVC]),
 ('B08-nfs-scanner-not-wired-to-db', US, 'NewNFSScanner(logger).WithDB(db)', 'NewNFSScanner(logger)', [SVC]),
 # --- fix round 2 behaviours
 ('N01-catalog-path-keeps-leading-slash', GS, 'return strings.TrimLeft(path.Clean("/"+p), "/")', 'return path.Clean("/" + p)', [SVC]),
 ('N02-db-seam-bypasses-catalog-path', GS, 'cp := catalogPath(p)', 'cp := p', [SVC]),
 ('N03-list-limit-not-passed-to-the-client', GS, 'lctx := filesystem.WithListLimit(ctx, limit)', 'lctx := ctx', [SVC]),
 ('N04-fetch-limit-ignores-remaining-budget', GS, 'if r := rs.g.o.MaxEntries - rs.total; r < lim {', 'if r := rs.g.o.MaxEntries - rs.total; false && r < lim {', [SVC]),
 ('N05-uncounted-entries-fail-a-fitting-tree', GS, '&& rs.countable(entries) <= remaining {', '&& false {', [SVC]),
 ('N07-skipped-ratio-never-fails', GS, 'if float64(k) > g.o.MaxSkippedRatio*float64(rs.listed) {', 'if false {', [SVC]),
 ('N07b-skipped-ratio-boundary', GS, 'if float64(k) > g.o.MaxSkippedRatio*float64(rs.listed) {', 'if float64(k) >= g.o.MaxSkippedRatio*float64(rs.listed) {', [SVC]),
 ('N08-skipped-dirs-not-reported', GS, 'status.setNote("completed with a smaller catalog: " + msg)', '_ = msg', [SVC]),
 ('N09-deadline-reported-as-cancelled', GS, 'if errors.Is(ctx.Err(), context.DeadlineExceeded) {', 'if false {', [SVC]),
 ('N10-incremental-claimed-without-token-store', GS, 'return g.o.Incremental && g.o.Tokens != nil', 'return g.o.Incremental', [SVC]),
 ('N11-token-etag-not-quoted', GS, 'fmt.Fprintf(&b, ";e=%q", t.ETag)', 'fmt.Fprintf(&b, ";e=%s", t.ETag)', [SVC]),
 ('N12-root-allow-empty-ignored', GS, 'rs.allowEmpty = g.o.AllowEmptyRoot || (job.StorageRoot != nil && job.StorageRoot.AllowEmpty)', 'rs.allowEmpty = g.o.AllowEmptyRoot', [SVC]),
 ('N13-dot-entries-counted-as-errors', GS, 'if e.Name == "." || e.Name == ".." {\n\t\t\tcontinue', 'if false {\n\t\t\tcontinue', [SVC]),
 ('N14-depth-probe-counts-skipped-dirs', GS, 'if e != nil && validEntryName(e.Name) && !(e.IsDir && rs.skip[e.Name]) {', 'if e != nil {', [SVC]),
 ('N15-cancel-waits-for-the-client', GS, '\tcase <-ctx.Done():\n\t\treturn nil, ctx.Err()\n\t}\n}', '\tcase <-ctx.Done():\n\t\ta := <-ch\n\t\treturn a.entries, a.err\n\t}\n}', [SVC]),
 ('N16-snapshot-dir-not-skipped-by-default', GS, 'o.SkipDirNames = []string{"@eaDir", "#recycle", "#snapshot"}', 'o.SkipDirNames = []string{"@eaDir", "#recycle"}', [SVC]),
 ('N17-skipped-ratio-default-unbounded', GS, 'o.MaxSkippedRatio = defaultMaxSkippedRatio', 'o.MaxSkippedRatio = 1.0', [SVC]),
 ('N18-depth-probe-disabled', GS, 'if r.t.depth > rs.maxDepth {', 'if false && r.t.depth > rs.maxDepth {', [SVC]),
 ('N19-context-error-not-classified-at-root', GS, '\t\tif ctx.Err() != nil {\n\t\t\treturn &ScanError{Kind: ctxKind(ctx), Path: root, Err: ctx.Err()}', '\t\tif false {\n\t\t\treturn &ScanError{Kind: ctxKind(ctx), Path: root, Err: ctx.Err()}', [SVC]),
 ('N20-rescan-inserts-instead-of-updating', US, 'if n, _ := res.RowsAffected(); n == 0 {', 'if _, _ = res.RowsAffected(); true {', [SVC]),
 ('N21-update-does-not-revive-deleted-rows', US, 'parent_id = COALESCE(?, parent_id), deleted = 0, deleted_at = NULL', 'parent_id = COALESCE(?, parent_id)', [SVC]),
 ('N22-panic-leaves-status-running', US, 'status.fail(panicErr)', '_ = panicErr', [SVC]),
 ('N23-reason-not-scrubbed', US, 's.Reason = scrubSecrets(err.Error(), s.secrets)\n\t}\n}\n\n// cancel', 's.Reason = err.Error()\n\t}\n}\n\n// cancel', [SVC]),
 ('N24-event-carries-raw-error', US, 'payload["error"] = snapshot.Reason', 'payload["error"] = scanErr.Error()', [SVC]),
 ('N25-cancelled-published-as-failed', US, 'eventType = eventbus.EventScanCancelled', 'eventType = eventbus.EventScanFailed', [SVC]),
 ('N26-secrets-of-root-not-collected', US, 'secrets:         secretsOf(job.StorageRoot),\n\t}\n\n\t// Track active scan', 'secrets:         nil,\n\t}\n\n\t// Track active scan', [SVC]),
 ('N27-cover-art-hand-built-smb-map', CA, 'settings := filesystem.SettingsFromRoot(t.storageRoot(), nil)', 'settings := map[string]interface{}{"host": t.host.String, "port": 445, "share": t.rootPath.String, "username": t.username.String, "password": t.password.String, "domain": "WORKGROUP"}', [SVC]),
 ('N28-cover-art-task-drops-url', CA, 'Domain: str(t.domain), URL: str(t.url),', 'Domain: str(t.domain), URL: nil,', [SVC]),
 # --- settings contract
 ('C02-smb-empty-domain-overrides', ST, 'if root.Domain != nil && *root.Domain != "" {', 'if root.Domain != nil {', [FS, SVC]),
 ('C05-registered-options-keys-ignored', ST, 'for _, k := range []string{KeyCredentialRef, KeyTLSMode, KeyHostKeyFingerprint, KeyCertSHA256} {', 'for _, k := range []string{} {', [FS, SVC]),
 ('C07-fractional-int-accepted', ST, 'x != math.Trunc(x) ||', '', [FS, SVC]),
 ('C07b-int-range-unchecked', ST, ' || x < math.MinInt32 || x > math.MaxInt32 {', ' {', [FS, SVC]),
 ('C07c-int64-range-unchecked', ST, '\tif n < math.MinInt32 || n > math.MaxInt32 {\n\t\treturn 0, false\n\t}\n\treturn n, true', '\treturn n, true', [FS, SVC]),
 ('C08-port-range-unchecked', ST, 'if n, _ := wholeNumber(v); n < 1 || n > 65535 {', 'if n, _ := wholeNumber(v); false && (n < 1 || n > 65535) {', [FS, SVC]),
 ('C09-hint-for-key-outside-schema', ST, '\t\t\t\tif _, hintOK := spec.Keys[hint]; hintOK {', '\t\t\t\tif _, hintOK := spec.Keys[hint]; hintOK || true {', [FS]),
 ('C10-stored-port-zero-forwarded', ST, 'if v != nil && !(k == KeyPort && *v <= 0) {', 'if v != nil {', [FS, SVC]),
 ('C11-nfs-export-key-reverted', ST, 'case "nfs":\n\t\tput(KeyHost, root.Host)\n\t\tput(KeyPath, root.Path)', 'case "nfs":\n\t\tput(KeyHost, root.Host)\n\t\tput("export_path", root.Path)', [FS, SVC]),
 ('C12-protocol-name-unvalidated', ST, 'if name == "" || name != spec.Name || strings.ContainsAny(name, " \\t/:") {', 'if name == "" {', [FS]),
 ('C13-unknown-keys-accepted', ST, '\t\tif !known {\n\t\t\tif hint', '\t\tif !known && false {\n\t\t\tif hint', [FS, SVC]),
 ('C14-ftp-path-not-forwarded', ST, 'put(KeyUsername, root.Username)\n\t\tput(KeyPassword, root.Password)\n\t\tput(KeyPath, root.Path)\n\tcase "nfs":', 'put(KeyUsername, root.Username)\n\t\tput(KeyPassword, root.Password)\n\tcase "nfs":', [FS, SVC]),
 ('C15-factory-skips-validation', FA, 'if err := ValidateSettings(config.Protocol, config.Settings); err != nil {\n\t\treturn nil, err\n\t}', '_ = ValidateSettings', [FS, SVC]),
 ('D01-supported-omits-registered', FA, 'return append(append([]string{}, builtinOrder...), RegisteredProtocols()...)', 'return append([]string{}, builtinOrder...)', [FS, HD]),
 ('D02-only-plain-int-ports-read', FA, 'if n, ok := wholeNumber(val); ok {\n\t\t\treturn int(n)', 'if n, ok := val.(int); ok {\n\t\t\treturn n', [FS]),
 # --- WebDAV
 ('E01-href-not-unescaped', PF, '\t\tp := u.Path\n', '\t\tp := u.EscapedPath()\n', [FS]),
 ('E02-propstat-without-status-rejected', PF, '\tif status == "" {\n\t\treturn true\n\t}', '\tif status == "" {\n\t\treturn false\n\t}', [FS]),
 ('E03-unparsable-mtime-is-now', PF, '\t\t\treturn t\n\t\t}\n\t}\n\treturn time.Time{}\n}', '\t\t\treturn t\n\t\t}\n\t}\n\treturn time.Now()\n}', [FS]),
 ('E05-directory-resourcetype-ignored', PF, 'IsDir:   prop.ResourceType.Collection != nil || prop.ResourceType.Directory != nil,', 'IsDir:   prop.ResourceType.Collection != nil,', [FS]),
 ('E07-name-from-presentation-string', PF, 'name := path.Base(hp)', 'name := strings.ToUpper(path.Base(hp))', [FS, SVC]),
 ('E09-foreign-hrefs-accepted', PF, 'if davNormPath(path.Dir(hp)) != selfN {', 'if false && selfN != "" {', [FS]),
 ('E10-entry-limit-ignored', PF, 'if limit > 0 && len(out) >= limit {', 'if false {', [FS]),
 ('E11-empty-body-accepted', PF, 'if !sawRoot {\n\t\treturn nil, fmt.Errorf', 'if false {\n\t\treturn nil, fmt.Errorf', [FS]),
 ('E12-too-large-answer-truncated-silently', PF, 'return 0, errPropfindTooLarge', 'return 0, io.EOF', [FS]),
 ('W01-url-path-replaced-by-root-path', WC, 'baseURL.Path = pathpkg.Join("/", baseURL.Path, config.Path)', 'baseURL.Path = config.Path', [FS, SVC]),
 ('W02-dotdot-inside-names-stripped', WC, 'cleanPath := pathpkg.Clean("/" + p)', 'cleanPath := strings.ReplaceAll(pathpkg.Clean("/"+p), "..", "")', [FS, SVC]),
 ('W03-listing-error-names-the-credential', WC, 'failed to list WebDAV directory %s: %w", redactURL(fullURL), err)', 'failed to list WebDAV directory %s: %w", fullURL, err)', [FS, HD]),
 ('W04-status-error-names-the-credential', WC, 'WebDAV server returned status %d for directory %s", resp.StatusCode, redactURL(fullURL))\n\t}\n\n\t// PA-03', 'WebDAV server returned status %d for directory %s", resp.StatusCode, fullURL)\n\t}\n\n\t// PA-03', [FS, HD]),
 ('W05-invalid-url-error-echoes-the-url', WC, 'return nil, fmt.Errorf("invalid WebDAV URL: %v", ue.Err)', 'return nil, fmt.Errorf("invalid WebDAV URL %q: %w", config.URL, err)', [FS]),
 ('W06-propfind-answer-unbounded', WC, '&boundedReader{r: resp.Body, max: MaxPropfindBytes}', 'resp.Body', [FS]),
 ('W07-list-limit-not-read', WC, 'limit, _ := ListLimit(ctx)', 'limit := 0', [FS, SVC]),
 ('W08-copy-error-names-the-credential', WC, 'redactURL(srcURL), redactURL(dstURL), err)', 'srcURL, dstURL, err)', [FS]),
 # --- FTP
 ('T01-ftp-base-not-used', FC, 'if c.base != "" {\n\t\treturn pathpkg.Join(c.base, p)', 'if false {\n\t\treturn pathpkg.Join(c.base, p)', [FS, SVC]),
 ('T02-ftp-dot-entries-listed', FC, 'if entry.Name == "." || entry.Name == ".." {\n\t\t\tcontinue', 'if false {\n\t\t\tcontinue', [SVC]),
 # --- handlers / models / database
 ('H01-reason-not-in-api-json', SH, '\t\t"reason":          s.Reason,\n', '', [HD]),
 ('H02-allowlist-is-a-fixed-list', SH, 'for _, p := range filesystem.NewDefaultClientFactory().SupportedProtocols() {', 'for _, p := range []string{"local", "smb", "ftp", "nfs", "webdav"} {', [HD]),
 ('H03-webdav-without-url-accepted', SH, 'if req.Protocol == "webdav" && (req.URL == nil || *req.URL == "") {', 'if false {', [HD]),
 ('H04-port-range-unchecked', SH, 'if req.Port != nil && (*req.Port < 1 || *req.Port > 65535) {', 'if false {', [HD]),
 ('H05-create-drops-allow-empty', SH, 'req.URL, req.MountPoint, req.Options, req.AllowEmpty, true, req.MaxDepth', 'req.URL, req.MountPoint, req.Options, false, true, req.MaxDepth', [HD]),
 ('H06-roots-listing-returns-raw-url', SH, 'r := u.Redacted()', 'r := *rawURL\n\t\t\t\t_ = u', [HD]),
 ('H07-loader-skips-allow-empty-target', SH, 'dest := append([]interface{}{&root.ID, &root.Name, &root.Enabled, &root.MaxDepth, &root.AllowEmpty}, root.ConnScanTargets()...)', 'var ignoredAE bool\n\tdest := append([]interface{}{&root.ID, &root.Name, &root.Enabled, &root.MaxDepth, &ignoredAE}, root.ConnScanTargets()...)', [HD]),
 ('H08-conn-targets-swap-username-password', MD, '&r.Username, &r.Password, &r.Domain', '&r.Password, &r.Username, &r.Domain', [HD, SVC]),
 ('H09-conn-columns-lose-url-and-mount-point', MD, 'domain, url, mount_point, options"', 'domain, options, options, options"', [HD, SVC]),
 ('V01-migration-21-not-registered', MG, '{Version: 21, Name: "storage_roots_allow_empty", Up: db.addStorageRootsAllowEmptyColumn},', '', [DB]),
]

def sha(p):
    h = hashlib.sha256()
    with open(p, 'rb') as f:
        h.update(f.read())
    return h.hexdigest()

def tree_fingerprint():
    out = {}
    for base in ('catalog-api', 'submodules/filesystem'):
        for dp, dn, fn in os.walk(os.path.join(REPO, base)):
            dn[:] = [d for d in dn if d not in ('.git', 'node_modules')]
            for f in fn:
                p = os.path.join(dp, f)
                if os.path.islink(p) or not os.path.isfile(p):
                    continue
                try:
                    out[os.path.relpath(p, REPO)] = sha(p)
                except OSError:
                    pass
    return out

def setup_copy():
    if os.path.exists(MUT):
        shutil.rmtree(MUT)
    os.makedirs(MUT + '/submodules')
    shutil.copytree(REPO + '/catalog-api', MUT + '/catalog-api', symlinks=True, ignore=shutil.ignore_patterns('node_modules', '.git', '*.test'))
    shutil.copytree(REPO + '/submodules/filesystem', MUT + '/submodules/filesystem', symlinks=True, ignore=shutil.ignore_patterns('.git'))
    for n in os.listdir(REPO + '/submodules'):
        if n == 'filesystem':
            continue
        os.symlink('../../../../../submodules/' + n, MUT + '/submodules/' + n)

def wait_for_memory(min_kb=5 * 1024 * 1024, max_wait=300):
    # run_pinned gives the container MemAvailable - max(4 GiB, 15% of MemTotal) (12.6): below ~9 GiB available the budget is too small to LINK the
    # handlers test binary (the linker is killed), which is no verdict about a mutant. Wait for the other workers to release memory.
    t0 = time.time()
    while time.time() - t0 < max_wait:
        try:
            avail = [int(l.split()[1]) for l in open('/proc/meminfo') if l.startswith('MemAvailable:')][0]
        except (OSError, IndexError, ValueError):
            return
        if avail >= min_kb:
            return
        time.sleep(30)

def run_go(tag, pkgs):
    log = '/dev/shm/fixr2_mut_%s.log' % tag
    env = dict(os.environ, WORK='.audit/scratch/wf22fix/mut/catalog-api')
    cmd = [RT, log, "test -count=1 -failfast -skip '" + SLOW + "' " + ' '.join(pkgs), '3']
    t0 = time.time()
    for attempt in range(40):
        wait_for_memory()
        try:
            subprocess.run(cmd, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=1500)
        except subprocess.TimeoutExpired:
            return None, 'TIMEOUT', time.time() - t0
        out = open(log, errors='replace').read() if os.path.exists(log) else ''
        # the launcher REFUSES (it never starts the container) when the host has no memory / disk headroom (12.6, 12.9): that is no verdict about the mutant, wait and retry
        if 'run_pinned: REFUSED reason=memory_budget_unavailable' in out or 'REFUSED reason=disk' in out or ': signal: killed' in out:
            time.sleep(60)
            continue
        break
    out = open(log, errors='replace').read() if os.path.exists(log) else ''
    m = re.findall(r'^go_rc=(\d+)\s*$', out, re.M)
    return (m[-1] if m else None), out, time.time() - t0

# compiler / go tool markers only: a test may print 'syntax error' (an XML parser's) or 'undefined' in its own failure message and still be a real kill
BUILD_MARK = re.compile(r'\[build failed\]|\[setup failed\]|^REFUSED|run_pinned: REFUSED|^\S+\.go:\d+:\d+: (?:cannot use|undefined:|.* declared and not used|.* imported and not used|syntax error)', re.M)

# mutants (ids of M) that are also run against the REAL FTP / WebDAV servers (--realfs); the stack of scripts/test-infra/up.sh --build-id wf22fix must be up
REAL = ['N01-catalog-path-keeps-leading-slash', 'N02-db-seam-bypasses-catalog-path', 'N13-dot-entries-counted-as-errors', 'N20-rescan-inserts-instead-of-updating',
        'N12-root-allow-empty-ignored', 'T01-ftp-base-not-used', 'W01-url-path-replaced-by-root-path', 'W03-listing-error-names-the-credential',
        'N23-reason-not-scrubbed', 'M01-scanner-body-empty-again', 'N18-depth-probe-disabled']

def run_real(tag):
    wait_for_memory()
    log = tag + '.log'
    env = dict(os.environ, WORK='.audit/scratch/wf22fix/mut/catalog-api')
    t0 = time.time()
    try:
        subprocess.run(['bash', EV + '/fix-r2-run_realfs.sh', log], env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=1800)
    except subprocess.TimeoutExpired:
        return None, 'TIMEOUT', time.time() - t0
    p = EV + '/' + log
    out = open(p, errors='replace').read() if os.path.exists(p) else ''
    m = re.findall(r'^realfs_rc=(\d+)', out, re.M)
    return (m[-1] if m else None), out, time.time() - t0

def verdict(rc, out):
    if rc is None:
        return 'ERROR(no go_rc line)'
    if BUILD_MARK.search(out):
        return 'ERROR(build/setup failure marker)'
    fails = re.findall(r'^\s*--- FAIL: (\S+)', out, re.M)
    if rc == '0':
        return 'SURVIVED' if not fails else 'ERROR(rc 0 with FAIL lines)'
    if fails:
        return 'KILLED'
    return 'ERROR(rc %s without any --- FAIL line)' % rc

def main():
    args = sys.argv[1:]
    only, outfile = None, EV + '/fix-r2-mutation_results.tsv'
    if '--list' in args:
        for m in M:
            print(m[0])
        return
    real = '--realfs' in args
    if '--only' in args:
        only = set(args[args.index('--only') + 1].split(','))
    if '--out' in args:
        outfile = args[args.index('--out') + 1]
    if '--realfs' in args and '--out' not in args:
        outfile = EV + '/fix-r2-mutation_results_realfs.tsv'
    before = tree_fingerprint()
    setup_copy()
    results = []
    live = open(outfile + '.partial', 'w')  # one line per finished mutant, so a long run that is interrupted keeps what it has
    def emit(r):
        results.append(r)
        live.write('\t'.join(r) + '\n')
        live.flush()
        print(r, flush=True)
    # negative control
    if real:
        rc, out, dt = run_real('fix-r2-realfs_mut_M00')
    else:
        ctl = sorted({q for (mid_, f_, o_, n_, ps_) in M if (not only or mid_ in only) for q in ps_}) or [SVC, FS, HD]
        rc, out, dt = run_go('M00', ctl)
    v = verdict(rc, out)
    emit(('M00-negative-control-no-change', 'CONTROL-OK' if v == 'SURVIVED' else 'CONTROL-BROKEN:' + v, '', '%.0fs' % dt))
    if v != 'SURVIVED':
        print('negative control failed: the unmutated copy does not pass; harness stops', flush=True)
        sys.exit(3)
    for mid, f, old, new, pkgs in M:
        if only and mid not in only:
            continue
        if real and mid not in REAL:
            continue
        p = os.path.join(MUT, 'catalog-api', f)
        orig = open(p).read()
        h0 = sha(p)
        n = orig.count(old)
        if n != 1:
            emit((mid, 'NOT-APPLICABLE(old text found %d times)' % n, '', ''))
            continue
        try:
            open(p, 'w').write(orig.replace(old, new))
            if real:
                rc, out, dt = run_real('fix-r2-realfs_mut_' + mid.split('-')[0])
                if rc is not None and rc != '0' and re.search(r'^--- FAIL', out, re.M) and 'BUILD_FAILED' not in out:
                    v = 'KILLED'
                elif rc == '0':
                    v = 'SURVIVED'
                else:
                    v = 'ERROR(realfs rc %s)' % rc
            else:
                rc, out, dt = run_go(mid.split('-')[0], pkgs)
                v = verdict(rc, out)
            fails = ', '.join(sorted(set(re.findall(r'^\s*--- FAIL: (\S+)', out, re.M)))[:4])
        finally:
            open(p, 'w').write(orig)
            if sha(p) != h0:
                raise SystemExit('restore mismatch ' + p)
        emit((mid, v, fails, '%.0fs' % dt))
    after = tree_fingerprint()
    changed = sorted(k for k in set(before) | set(after) if before.get(k) != after.get(k))
    mine = [k for k in changed if k.startswith('catalog-api/')]
    same = not mine
    killed = sum(1 for r in results if r[1] == 'KILLED')
    survived = [r[0] for r in results if r[1] == 'SURVIVED']
    other = [r for r in results[1:] if r[1] not in ('KILLED', 'SURVIVED')]
    live.close()
    os.remove(outfile + '.partial')
    with open(outfile, 'w') as fh:
        fh.write('# fix-r2 mutation run; copy of the tree under .audit/scratch/wf22fix/mut; catalog-api files of the shared tree changed during the run: %d %s (%d files fingerprinted before and after; changes under submodules/filesystem by other workers during the run: %d)\n' % (len(mine), mine[:5], len(before), len(changed) - len(mine)))
        for r in results:
            fh.write('\t'.join(r) + '\n')
        fh.write('# mutants=%d killed=%d survived=%d other=%d control=%s\n' % (len(results) - 1, killed, len(survived), len(other), results[0][1]))
        if survived:
            fh.write('# SURVIVED: %s\n' % ', '.join(survived))
    print('done: killed=%d survived=%s other=%d catalog_api_unchanged=%s' % (killed, survived, len(other), same))

if __name__ == '__main__':
    main()
