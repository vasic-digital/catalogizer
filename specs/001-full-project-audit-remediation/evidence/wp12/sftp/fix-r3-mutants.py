#!/usr/bin/env python3
# identity: WP-12 PA-05 fix-r3 mutation harness (fix-r2 harness, re-targeted onto the fix-r3 tree, plus the reviewer's WF24 mutants V* and mutants of every fix-r3 mechanism) for submodules/filesystem/pkg/sftp (+ pkg/factory)
# usage:   python3 -I fix-r3-mutants.py run <out-dir> [ID,ID,...]      one mutant at a time, one container each (unit suite in the pinned IMG-GO container)
#          python3 -I fix-r3-mutants.py runbatch <out-dir> <n> [ID,...]  n mutants per container (the host admits a container only when memory is free)
#          python3 -I fix-r3-mutants.py list | check              check = every pattern matches exactly `count` times in the module (no container)
# What it is: the 21 mutants the independent reviewer wrote in round 1 (R01..R21, re-targeted onto the code after the fix - the text they
# mutated no longer exists in the same form), the 14 author mutants of round 1 (M*, re-targeted), and mutants for every NEW mechanism of
# fix-r2 (N*, P*, E*, F*, K*, D*, W*, S*). Each copy gets ONE exact replacement set, asserted to match exactly `count` times, the checkout
# is never modified. Verdicts: KILLED (the suite fails or the run does not complete: a hang is a kill), SURVIVED (suite green), INVALID (does not
# compile). Negative controls: Z00 (unmutated, must PASS) and Z01 (comment-only edit, must SURVIVE). A mutant is only believed after Z00 passed in the same run.
import json, os, re, shutil, subprocess, sys, time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), *[".."] * 5))
MOD = os.environ.get("FIXR3_MOD", os.path.join(ROOT, "submodules", "filesystem"))  # a frozen copy of the module may be given: other packages of the checkout are edited concurrently by other work
SCR = os.environ.get("FIXR3_SCR", os.path.join(ROOT, ".audit", "scratch", "sftp-fixr3-mut"))
ENV = dict(os.environ, TMPDIR="/dev/shm", DISK_HEADROOM_REPO_ROOT=ROOT, LONGOPS_ALLOW_TMPFS="1")

S, H, C, FR, F = "pkg/sftp/sftp.go", "pkg/sftp/hostkey.go", "pkg/sftp/credential.go", "pkg/sftp/frame.go", "pkg/factory/factory.go"

# (id, description, [(file, old, new, count), ...])
M = [
 ("Z00", "NEGATIVE CONTROL: unmutated copy (must PASS)", []),
 ("Z01", "NEGATIVE CONTROL: comment-only edit (must SURVIVE; shows the harness reports survival)", [(S, "// ErrReadOnly is returned by every mutating operation.", "// ErrReadOnly is returned by every mutating operation (edited).", 1)]),
 # ---- the reviewer's 21 mutants (WF19-REVIEW-sftp.md section 3), re-targeted
 ("R01", "pipelining off", [(S, "gosftp.MaxConcurrentRequestsPerFile(orInt(c.cfg.MaxConcurrentRequests, DefaultMaxConcurrentRequests)),", "gosftp.MaxConcurrentRequestsPerFile(1),", 1), (S, "gosftp.UseConcurrentReads(true),", "gosftp.UseConcurrentReads(false),", 1)]),
 ("R02", "file.WriteTo removed", [(S, "func (fl *file) WriteTo(w io.Writer) (n int64, err error) {", "func (fl *file) writeToDisabled(w io.Writer) (n int64, err error) {", 1)]),
 ("R03", "ListDirectory keeps '.' and '..'", [(S, 'if !plausibleName(e.Name()) {', "if false {", 1)]),
 ("R04", "context end no longer closes the open file", [(S, "fl.stop = context.AfterFunc(ctx, func() { _ = fl.Close() })", "fl.stop = func() bool { return true }", 1)]),
 ("R05", "handshake not cancellable by ctx", [(S, "stop := context.AfterFunc(ctx, func() { _ = nc.Close() })", "stop := func() bool { return true }", 1)]),
 ("R06", "handshake deadline removed", [(S, "\t_ = nc.SetDeadline(time.Now().Add(timeout))\n", "", 1)]),
 ("R07", "Disconnect closes nothing", [(S, "\tif cn != nil {\n\t\tcn.close()\n\t}\n\treturn nil\n}\n\n// IsConnected", "\t_ = cn\n\treturn nil\n}\n\n// IsConnected", 1)]),
 ("R08", "client never wipes the resolved credential", [(S, "\tdefer cred.wipe()\n", "\t_ = cred.wipe\n", 1)]),
 ("R09", "factory ignores sftp.DefaultPinStore", [(F, "PinStore:      sftppkg.DefaultPinStoreRef(),", "PinStore:      nil,", 1)]),
 ("R10", "TestConnection accepts a root that is not a directory", [(S, "\tif !fi.IsDir() {", "\tif !fi.IsDir() && false {", 1)]),
 ("R11", "ListDirectory ignores ctx (ReadDir)", [(S, "sc.ReadDirContext(ctx, remote)", "sc.ReadDir(remote)", 1)]),
 ("R12", "every net.Error is a connection loss / transient", [(S, "return errors.As(err, &ne) && ne.Timeout()", "return errors.As(err, &ne)", 1)]),
 ("R13", "ReadRange negative-offset guard removed", [(S, '\tif offset < 0 {\n\t\treturn nil, fmt.Errorf("sftp: negative offset %d", offset)\n\t}\n', "", 1)]),
 ("R14", "SANITY: a lost connection is never marked lost", [(S, "\tcase dispDropErr:\n\t\tc.drop(cn)\n\t\treturn err\n", "\tcase dispDropErr:\n\t\treturn err\n", 1)]),
 ("R15", "dial drops the captured typed host key refusal (reviewer: EQUIVALENT, x/crypto wraps with %w)", [(S, "\t\tif refusal != nil {\n\t\t\treturn nil, refusal\n\t\t}\n", "", 1)]),
 ("R16", "FileExists reports a path escape as 'does not exist'", [(S, "\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn false, nil", "\tif errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrPathEscape) {\n\t\treturn false, nil", 1)]),
 ("R17", "confine rewrites backslashes again (SFTP-07 back)", [(S, '\tvar segs []string\n\tfor _, s := range strings.Split(p, "/") {', '\tp = strings.ReplaceAll(p, "\\\\", "/")\n\tvar segs []string\n\tfor _, s := range strings.Split(p, "/") {', 1)]),
 ("R18", "backoff constant instead of exponential", [(S, "if delay *= 2; delay > maxRetryDelay {", "if delay *= 1; delay > maxRetryDelay {", 1)]),
 ("R19", "GetFileInfo('/') no longer named '/'", [(S, '\t\tif logical == "/" {\n\t\t\tout.Name = "/"\n\t\t}\n', "", 1)]),
 ("R20", "containment fails OPEN again (SFTP-06 back)", [(S, '\t\treturn fmt.Errorf("sftp: cannot verify that %q stays inside the root: %w", remote, err)\n', "\t\treturn nil\n", 1)]),
 ("R21", "io.EOF classified as connection loss again (SFTP-05 back)", [(S, "errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||", "errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) ||", 1)]),
 # ---- the author's 14 mutants of round 1 (M12 is integration-only and is run through the OpenSSH fixture, see the README), re-targeted
 ("M01", "auth failure becomes transient", [(S, "return nil, &AuthError{User: c.cfg.Username, Host: hp, Err: err}", 'return nil, fmt.Errorf("sftp: auth %s: %w", hp, io.ErrUnexpectedEOF)', 1)]),
 ("M02", "unpinned host key accepted (TOFU)", [(H, "\t\t\treturn *refusal\n\t\t}\n\t\tpinned := ", "\t\t\treturn nil\n\t\t}\n\t\tpinned := ", 1)]),
 ("M03", "a CHANGED host key accepted", [(H, "Presented: fp, Pinned: pinned}\n\t\treturn *refusal", "Presented: fp, Pinned: pinned}\n\t\treturn nil", 1)]),
 ("M04", "'..' clamps at the root instead of refusing", [(S, '\t\t\tif len(segs) == 0 {\n\t\t\t\treturn "", "", fmt.Errorf("%w: %q", ErrPathEscape, p)\n\t\t\t}\n\t\t\tsegs = segs[:len(segs)-1]', '\t\t\tif len(segs) > 0 {\n\t\t\t\tsegs = segs[:len(segs)-1]\n\t\t\t}', 1)]),
 ("M05", "WriteFile no longer refused", [(S, "func (c *Client) WriteFile(context.Context, string, io.Reader) error { return ErrReadOnly }", "func (c *Client) WriteFile(context.Context, string, io.Reader) error { return nil }", 1)]),
 ("M06", "an absent mtime is fabricated as now", [(S, "\t\tmt = time.Time{}", "\t\tmt = time.Now()", 1)]),
 ("M07", "retry budget off by one", [(S, "attempt >= c.retries()", "attempt > c.retries()", 1)]),
 ("M08", "server-side containment check disabled", [(S, "\tif c.cfg.SkipSymlinkCheck {\n\t\treturn nil\n\t}\n\trootReal := cn.rootReal", "\tif true {\n\t\treturn nil\n\t}\n\trootReal := cn.rootReal", 1)]),
 ("M09", "Credential.String leaks the password", [(C, 'return "Credential(redacted)"', 'return "Credential(" + c.Password + ")"', 1)]),
 ("M10", "Pin records whatever the server presents", [(H, "if all[i].Fingerprint == conf.Fingerprint {", "if true {", 1)]),
 ("M11", "Disconnect leaves the client re-dialable", [(S, "\tc.cur = nil\n\tc.state = stateDown\n\tc.gen++\n\tc.mu.Unlock()", "\tc.cur = nil\n\tc.state = stateLost\n\tc.gen++\n\tc.mu.Unlock()", 1)]),
 ("M13", "containment prefix accepts a sibling with the same prefix", [(S, 'strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")', "strings.HasPrefix(p, root)", 1)]),
 ("M14", "FileExists reports a missing file as an error", [(S, "\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn false, nil\n\t}\n\treturn false, err", "\treturn false, err", 1)]),
 # ---- NEW mechanisms
 ("N01", "cancellation guard never closes the connection", [(S, "tm = time.AfterFunc(g, func() { c.drop(cn) })", "tm = time.AfterFunc(g, func() {})", 1), (S, "\t\t\tgo c.drop(cn)\n", "", 1)]),
 ("N02", "no cancel grace: a cancel closes the shared connection at once", [(S, "\t\treturn DefaultCancelGrace\n", "\t\treturn 0\n", 1)]),
 ("N04", "no single-flight re-dial (SFTP-04 back)", [(S, "\t\t\tif f := c.flight; f != nil {", "\t\t\tif f := c.flight; f != nil && false {", 1)]),
 ("N05", "TestConnection disconnects the caller's client (SFTP-02 back)", [(S, "\t\treturn c.with(ctx, c.checkRoot)\n", "\t\terr := c.with(ctx, c.checkRoot)\n\t\t_ = c.Disconnect(ctx)\n\t\treturn err\n", 1)]),
 ("N06", "reply validator off (SFTP-08 back)", [(FR, "\tnames, err := validateFrame(b[4], b[5:])\n", "\tnames, err := 0, error(nil)\n\t_ = validateFrame\n", 1)]),
 ("N07", "reply validator accepts an over-long frame", [(FR, "if n < 1 || n > maxFrame {", "if n < 1 {", 1)]),
 ("N08", "no recover boundary", [(S, "if r := recover(); r != nil {", "if r := any(nil); r != nil {", 1)]),
 ("N09", "alive() always says alive", [(S, "\tcase err := <-res:\n\t\treturn err == nil\n", "\tcase err := <-res:\n\t\t_ = err\n\t\treturn true\n", 1)]),
 ("N21", "alive() treats a silent peer (timeout) as alive", [(S, "\tcase <-t.C:\n\t\treturn false\n\tcase <-cn.dead:", "\tcase <-t.C:\n\t\treturn true\n\tcase <-cn.dead:", 1)]),
 ("N10", "keepalive never runs", [(S, "\tiv, ok := c.keepaliveInterval()\n\tif !ok {", "\tiv, ok := c.keepaliveInterval()\n\tif !ok || true {", 1)]),
 ("N11", "listing limit not enforced", [(FR, "if t.count[h] > t.max {", "if false {", 1)]),
 ("N12", "ranged reads use a one-packet window (no pipelining)", [(S, "w := orInt(c.cfg.MaxPacket, DefaultMaxPacket) * orInt(c.cfg.MaxConcurrentRequests, DefaultMaxConcurrentRequests)", "w := 1024", 1)]),
 ("N13", "implicit re-dial installs over a Disconnect (SFTP-03 back)", [(S, "\t\t\tcase c.state != stateLost:", "\t\t\tcase false:", 1)]),
 ("N14", "bounded Close timer never fires", [(S, "t := time.AfterFunc(fl.c.closeTimeout(), func() { fl.c.drop(fl.cn) })", "t := time.AfterFunc(time.Hour, func() {})", 1)]),
 ("N15", "a STATUS EOF is reported as the bare io.EOF (transient, SFTP-05 back)", [(S, "\t\t\treturn &serverStatus{err: err}\n", "\t\t\treturn err\n", 1)]),
 ("N16", "a read that ends on a dead connection is an EOF", [(S, "if err == io.EOF && !fl.cn.alive() {", "if false {", 1)]),
 ("N17", "a copy that ends on a dead connection is a clean end", [(S, "if (err == nil || err == io.EOF) && !fl.cn.alive() {", "if false {", 1)]),
 ("N18", "malformed-reply fault not returned (a generic lost-connection error instead)", [(S, "\tif f := cn.faultErr(); f != nil {\n\t\tc.drop(cn)\n\t\treturn f\n\t}\n\tswitch callDisposition(ctx.Err(), err) {", "\tswitch callDisposition(ctx.Err(), err) {", 1)]),
 ("N19", "context error not mapped after a guard close", [(S, "\tcase dispDropCtx:\n\t\tc.drop(cn)\n\t\treturn ctx.Err()\n", "\tcase dispDropCtx:\n\t\tc.drop(cn)\n\t\treturn err\n", 1)]),
 ("N20", "server-side containment root not resolved at dial", [(S, "\t\tcn.rootReal = rr\n", "\t\t_ = rr\n", 1)]),
 ("L01", "a listing on a connection that ended during it is accepted", [(S, "\t\tif cn.isDead() {\n\t\t\treturn fmt.Errorf(\"%w: connection ended during the listing\"", "\t\tif false {\n\t\t\treturn fmt.Errorf(\"%w: connection ended during the listing\"", 1)]),
 ("Q01", "newFile publishes its stop function without the lock (nil-func panic / data race when the context has already ended; found while fixing, run with -race)", [(S, "\tfl.smu.Lock()\n\tfl.stop = context.AfterFunc(ctx, func() { _ = fl.Close() })\n\tfl.smu.Unlock()\n", "\tfl.stop = context.AfterFunc(ctx, func() { _ = fl.Close() })\n", 1), (S, "\t\tfl.smu.Lock()\n\t\tstop := fl.stop\n\t\tfl.smu.Unlock()\n\t\tstop()\n", "\t\tfl.stop()\n", 1)]),
 ("P01", "pin file writable by others is read", [(H, "\tif fi.Mode().Perm()&0o022 != 0 {", "\tif false {", 1)]),
 ("P02", "pin directory writable by others accepted", [(H, "\t\tif di.Mode().Perm()&0o022 != 0 && di.Mode()&os.ModeSticky == 0 {", "\t\tif false {", 1)]),
 ("P03", "no cross-process lock on the pin file", [(H, "\tif err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {", "\tif err := error(nil); err != nil {", 1)]),
 ("P04", "a non-regular pin file (symlink, directory) accepted", [(H, "\tif !fi.Mode().IsRegular() {", "\tif false {", 1)]),
 ("P05", "foreign owner accepted", [(H, "\treturn st.Uid == uint32(os.Getuid()) || st.Uid == 0", "\t_ = st\n\treturn true", 1)]),
 ("E01", "env credential refs not validated (SFTP-16 back)", [(C, "\tif !validRef(ref) {", "\tif false {", 1)]),
 ("E02", "env credential names fold case", [(C, "func envName(prefix, ref, field string) string { return prefix + \"_\" + ref + \"_\" + field }", "func envName(prefix, ref, field string) string {\n\treturn prefix + \"_\" + strings.ToUpper(ref) + \"_\" + field\n}", 1)]),
 ("F01", "factory accepts inline secrets other than 'password'", [(F, '[]string{"pass", "pwd", "secret", "token", "key", "cred", "auth"}', '[]string{"pass"}', 1)]),
 ("F02", "factory: an empty path wins over root", [(F, "\tif strings.TrimSpace(root) == \"\" {", "\tif strings.TrimSpace(root) == \"\" && false {", 1)]),
 ("F03", "factory: port not validated", [(F, "if !isNum || n < 1 || n > 65535 {", "if (!isNum || n < 1 || n > 65535) && false {", 1)]),
 ("D01", "Discover ignores ctx after the TCP connect", [(H, "stop := context.AfterFunc(ctx, func() { _ = conn.Close() })", "stop := func() bool { return true }", 1)]),
 ("K01", "no keyboard-interactive login (SFTP-12 back)", [(S, "methods = append(methods, ssh.Password(pw), ssh.KeyboardInteractive(kbdAnswer(pw)))", "methods = append(methods, ssh.Password(pw))", 1)]),
 ("K02", "keyboard-interactive failure not recognised as an auth failure", [(S, ' || strings.Contains(m, "unexpected message type 51 (expected 60)")', "", 1)]),
 ("K03", "keyboard-interactive answers echoed prompts with the password", [(S, "\t\t\tif i < len(echos) && echos[i] {", "\t\t\tif false {", 1)]),
 # ---- fix-r3: the reviewer's WF24 mutants V* (re-targeted onto the fix-r3 tree)
 ("V01", "WF24 V01: the handshake deadline is never cleared (every connection dies DialTimeout after dial)", [(S, "\t_ = nc.SetDeadline(time.Time{})\n\tc.keepalive(cn)", "\tc.keepalive(cn)", 1)]),
 ("V02", "WF24 V02: the default keepalive is disabled", [(S, "\tcase d == 0:\n\t\treturn DefaultKeepAliveInterval, true", "\tcase d == 0:\n\t\treturn 0, false", 1)]),
 ("V03", "WF24 V03: single-flight waiters inherit the dialer's context error", [(S, "if f.err != nil && !isCtxErr(f.err) {", "if f.err != nil {", 1)]),
 ("V04", "WF24 V04: TestConnection on a LOST client probes on its own connection (client not repaired)", [(S, "if st == stateUp || st == stateLost {", "if st == stateUp {", 1)]),
 ("V06b", "WF24 V06b: guard release does not stop the armed grace timer", [(S, "\t\tif tm != nil {\n\t\t\ttm.Stop()\n\t\t}\n", "\t\t_ = tm\n", 1)]),
 ("V07", "WF24 V07: negative CancelGrace no longer means 'close at once'", [(S, "\tcase d < 0:\n\t\treturn 0\n\tcase d == 0:", "\tcase d <= 0:", 1)]),
 ("V08", "WF24 V08: ranged.WriteTo no longer reports a short write", [(S, "\t\t\tif m < n {\n\t\t\t\treturn total, io.ErrShortWrite\n\t\t\t}\n", "", 1)]),
 ("V09", "WF24 V09: file.Close no longer maps a dead handle to nil", [(S, "\treturn errors.Is(err, os.ErrClosed) || errors.Is(err, gosftp.ErrSSHFxConnectionLost) || errors.Is(err, gosftp.ErrSSHFxNoConnection) ||\n\t\tfl.cn.isDead() || (errors.Is(err, io.EOF) && !fl.cn.alive())", "\treturn errors.Is(err, os.ErrClosed)", 1)]),
 ("V10", "WF24 V10: validator skips 4 instead of 8 bytes for uid/gid", [(FR, 'w.skip(8, "attribute uid/gid")', 'w.skip(4, "attribute uid/gid")', 1)]),
 ("V11", "WF24 V11: listing budget off by one (> becomes >=)", [(FR, "if t.count[h] > t.max {", "if t.count[h] >= t.max {", 1)]),
 ("V12", "WF24 V12: listing budget never reset (a CLOSE does not end it)", [(FR, "\t\tdelete(t.count, h)\n\t\tdelete(t.over, h)\n", "", 1)]),
 ("V15", "WF24 V15: 4 MiB ranged window cap removed", [(S, "\tif w > maxReadWindow {\n\t\tw = maxReadWindow\n\t}\n", "", 1)]),
 ("V16", "WF24 V16: a frame cut after its header is io.EOF (clean end) instead of io.ErrUnexpectedEOF", [(FR, "\t\tif err == io.EOF {\n\t\t\terr = io.ErrUnexpectedEOF\n\t\t}\n\t\tf.buf = b[:0]", "\t\tf.buf = b[:0]", 1)]),
 ("V18", "WF24 V18: Connect replacing a live connection no longer closes the old one (leak)", [(S, "\t\tif old != nil {\n\t\t\told.close()\n\t\t}\n\t\treturn nil", "\t\t_ = old\n\t\treturn nil", 1)]),
 ("V27b", "WF24 V27b: listing entry Path ignores the listed directory", [(S, "toInfo(path.Join(logical, e.Name()), e)", 'toInfo(path.Join(strings.TrimPrefix(logical, logical)+"/", e.Name()), e)', 1)]),
 ("V38", "WF24 V38: EnvCredentialResolver with an empty Prefix reads the wrong variables", [(C, '\tif prefix == "" {\n\t\tprefix = "SFTP_CRED"\n\t}', '\tif prefix == "" {\n\t\tprefix = "SFTP"\n\t}', 1)]),
 # ---- fix-r3: the defects of the review put back (each one must be caught by the adopted probe / new test)
 ("X01", "S01a back: a context error is a connection loss again (isConnectionLoss without the context exclusion)", [(S, "\tif isCtxErr(err) {\n\t\t// context.DeadlineExceeded is a net.Error", "\tif false && isCtxErr(err) {\n\t\t// context.DeadlineExceeded is a net.Error", 1)]),
 ("X02", "S01b back: a call that gave up by itself on the context drops the connection", [(S, "\tcase dispCtxOnly:\n\t\treturn ctx.Err()\n", "\tcase dispCtxOnly:\n\t\tc.drop(cn)\n\t\treturn ctx.Err()\n", 1)]),
 ("X03", "S01b (table): callDisposition drops on a context error by itself", [(S, "\t\tif isCtxErr(err) {\n\t\t\t// pkg/sftp's ctx-aware calls", "\t\tif false && isCtxErr(err) {\n\t\t\t// pkg/sftp's ctx-aware calls", 1)]),
 ("X04", "S02 back: AuthError no longer answers errors.Is(err, fabric.ErrAuth)", [(S, "func (e *AuthError) Is(target error) bool { return target == fabric.ErrAuth }", "func (e *AuthError) Is(target error) bool { return false && target == fabric.ErrAuth }", 1)]),
 ("X05", "S03 back: NAME replies of any request are counted (REALPATH included)", [(FR, "\th, ok := t.readdir[id]\n\tif !ok {\n\t\treturn false\n\t}\n\tdelete(t.readdir, id)", "\th, ok := t.readdir[id]\n\t_ = ok\n\tdelete(t.readdir, id)", 1)]),
 ("X06", "S03: an over-budget page is NOT replaced (the limit is never reported)", [(FR, "\t\tt.over[h] = true\n\t\treturn true\n", "\t\tt.over[h] = true\n\t\treturn false\n", 1)]),
 ("X07", "S03: a STATUS reply does not forget its READDIR id", [(FR, "\tt.mu.Lock()\n\tdelete(t.readdir, id)\n\tt.mu.Unlock()\n", "\tt.mu.Lock()\n\tt.mu.Unlock()\n", 1)]),
 ("X08", "S03: the outgoing parser mis-skips an uninteresting frame by one byte", [(FR, "t.skip = flen - 1 // not interesting", "t.skip = flen // not interesting", 1)]),
 ("X09", "S03: the limit error is a connection fault again (the whole connection ends)", [(S, "\t\t\tif cn.tr.isLimitError(err) {", "\t\t\tif cn.tr.isLimitError(err) {\n\t\t\t\tcn.setFault(ErrDirTooLarge)", 1)]),
 ("X10", "S04 back: the end-of-file liveness question has its own constant 3 s bound", [(S, "tr: newWireTracker(c.maxDirEntries()), aliveTimeout: c.livenessTimeout()}", "tr: newWireTracker(c.maxDirEntries()), aliveTimeout: 3 * time.Second}", 1)]),
 ("X11", "S04: the periodic keepalive ignores KeepAliveTimeout", [(S, "\tto := c.livenessTimeout()\n\tgo func() {", "\tto := DefaultKeepAliveTimeout\n\tgo func() {", 1)]),
 ("X13", "S07 back: no read-ahead", [(S, "\tif !fl.noRA && fl.seq >= raAfter && len(p) < win {", "\tif false && !fl.noRA && fl.seq >= raAfter && len(p) < win {", 1)]),
 ("X13b", "S07: a bounded range reads ahead beyond its end", [(S, "\tf.noRA = true\n", "", 1)]),
 ("X14", "S07: SeekCurrent is sent to the server relative to ITS offset (ahead of the logical one by the buffer)", [(S, "\t\toff, whence = abs, io.SeekStart // the sftp file", "\t\t_ = abs // the sftp file", 1)]),
 ("X15", "S07: a real Seek keeps the stale read-ahead buffer", [(S, "\tfl.ra, fl.pending, fl.seq, fl.raNext = nil, nil, 0, 0\n", "\tfl.pending, fl.seq, fl.raNext = nil, 0, 0\n", 1)]),
 ("X16", "S07: a read-ahead fill does not verify the end of the file against the connection", [(S, "\tif err == io.EOF && !fl.cn.alive() {\n\t\tfl.c.drop(fl.cn)\n\t\treturn n, fmt.Errorf(\"%w: %v\", gosftp.ErrSSHFxConnectionLost, err)\n\t}\n\treturn n, err\n}\n\nfunc (fl *file) Read", "\treturn n, err\n}\n\nfunc (fl *file) Read", 1)]),
 ("X17", "S07: WriteTo forgets the buffered read-ahead bytes (data loss in the middle of the file)", [(S, "\tif len(fl.ra) > 0 {\n\t\tm, werr := ctxWriter{ctx: fl.ctx, w: w}.Write(fl.ra)", "\tif false && len(fl.ra) > 0 {\n\t\tm, werr := ctxWriter{ctx: fl.ctx, w: w}.Write(fl.ra)", 1)]),
 ("X18", "S07: a Seek inside the buffer does not advance it (wrong bytes)", [(S, "\t\t\tfl.ra = fl.ra[abs-fl.pos:]\n", "", 1)]),
 ("X19", "S08 back: a server name '/' (or any name containing '/') is listed", [(S, ' && !strings.Contains(n, "/")', "", 1)]),
 ("X20", "S08: a NUL name is listed", [(S, " && !strings.ContainsRune(n, 0)", "", 1)]),
 ("X21", "S09 back: file.call does not map the wrapped io.EOF of a dead channel", [(S, "\tif errors.Is(err, io.EOF) && !fl.cn.alive() {\n\t\t// the send error of an already closed channel", "\tif false && errors.Is(err, io.EOF) && !fl.cn.alive() {\n\t\t// the send error of an already closed channel", 1)]),
 ("X22", "S10 back: Connect installs after a Disconnect", [(S, "\t\tif c.gen != gen {", "\t\tif false && c.gen != gen {", 1)]),
 ("X23", "S10: Disconnect does not bump the generation", [(S, "\tc.state = stateDown\n\tc.gen++\n", "\tc.state = stateDown\n", 1)]),
 ("X24", "S11: secret spellings are matched case-sensitively", [(F, '\tn := strings.ToLower(k)\n', '\tn := k\n', 1)]),
 ("X25", "S11: unknown keys are accepted", [(F, '\t\treturn nil, fmt.Errorf("sftp: unknown setting %q', '\t\tcontinue\n\t\treturn nil, fmt.Errorf("sftp: unknown setting %q', 1)]),
 ("X26", "S14 back: MaxPacket above the pkg/sftp limit is not refused up front", [(S, "\tif c.cfg.MaxPacket > MaxPacketLimit {", "\tif false && c.cfg.MaxPacket > MaxPacketLimit {", 1)]),
 ("X27", "S14 back: the factory captures sftp.DefaultPinStore when the client is created", [(F, "PinStore:      sftppkg.DefaultPinStoreRef(),", "PinStore:      sftppkg.DefaultPinStore,", 1)]),
 ("X28", "S14: the lazy pin store returns no error when no store is installed", [(H, "\treturn nil, ErrNoPinStore\n}\n\nfunc (r defaultPinStoreRef) Lookup", "\treturn NewMemPinStore(), nil\n}\n\nfunc (r defaultPinStoreRef) Lookup", 1)]),
 ("X29", "S10: ErrDisconnected is a connection loss again (transient: Connect would retry it)", [(S, 'var ErrDisconnected = errors.New("sftp: disconnected while connecting")', 'var ErrDisconnected = fmt.Errorf("sftp: disconnected while connecting: %w", gosftp.ErrSSHFxConnectionLost)', 1)]),
]


def sh(cmd, cwd=None, env=None, timeout=1500):
    p = subprocess.run(cmd, cwd=cwd, env=env or ENV, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=timeout)
    return p.returncode, p.stdout


def make_copy(cid):
    d = os.path.join(SCR, cid)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(os.path.join(d, "view"), mode=0o700)
    shutil.copytree(MOD, os.path.join(d, "view", "filesystem"), ignore=shutil.ignore_patterns(".git"))
    return d


def apply(d, f, old, new, count):
    p = os.path.join(d, "view", "filesystem", f)
    s = open(p).read()
    n = s.count(old)
    if n != count:
        raise SystemExit("mutation pattern matched %d times (expected %d) in %s: %r" % (n, count, f, old[:80]))
    open(p, "w").write(s.replace(old, new))


RACE = {"Q01"}  # mutants that need the race detector to be visible
# only these mutants need pkg/factory (it imports pkg/ftp, which another worker edits concurrently: a transient break there must not invalidate pkg/sftp mutants)
FACTORY = {"Z00", "R09", "F01", "F02", "F03", "X24", "X25", "X27", "X28"}


def run_unit(d, cid, cache):
    cmd = ["bash", os.path.join(ROOT, "scripts/containers/run_pinned.sh"), "--out", cache, "--op-id", "sftpfixr3-" + cid.lower(), "IMG-GO", "--",
           "sh", "-c", "cd /src/filesystem && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 "
           "CGO_ENABLED=1 go test -count=1 -timeout 240s " + ("-race " if cid in RACE else "") + ("./pkg/sftp/ ./pkg/factory/" if cid in FACTORY else "./pkg/sftp/")]
    # the host refuses a container when memory is short (other work is running): wait and retry, never count that as a verdict
    for _ in range(90):
        rc, o = sh(cmd, cwd=os.path.join(d, "view"))
        if "memory_budget_unavailable" in o or "disk_headroom" in o:
            time.sleep(30)
            continue
        break
    return rc, o


def classify(rc, o):
    fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", o, re.M)))
    if rc == 0:
        return "SURVIVED", []
    if re.search(r"\[build failed\]|\.go:\d+:\d+: ", o) and not fails:
        return "INVALID", []
    if "panic: test timed out" in o:
        fails.append("(timeout: the run did not complete)")
    if "signal: killed" in o and not fails:
        fails.append("(the test binary was killed: out of memory)")
    if re.search(r"^panic: ", o, re.M) and not fails:
        fails.append("(panic: the test binary crashed)")
    return ("KILLED", fails) if fails else ("ERROR", [])


def run_batch(ids, cache, tag):
    """One container for a whole batch of mutants (the host admits a container only when memory is free; one admission per batch,
    not per mutant). Every mutant has its own copy under <batch>/<id>/filesystem; the suite of each runs in turn."""
    root = os.path.join(SCR, "batch-" + tag)
    shutil.rmtree(root, ignore_errors=True)
    os.makedirs(root, mode=0o700)
    by_id = {cid: (desc, edits) for cid, desc, edits in M}
    for cid in ids:
        shutil.copytree(MOD, os.path.join(root, cid, "filesystem"), ignore=shutil.ignore_patterns(".git"))
        for f, old, new, count in by_id[cid][1]:
            p = os.path.join(root, cid, "filesystem", f)
            s = open(p).read()
            n = s.count(old)
            if n != count:
                raise SystemExit("mutation pattern matched %d times (expected %d) in %s (%s): %r" % (n, count, f, cid, old[:80]))
            open(p, "w").write(s.replace(old, new))
    outdir = os.path.join(cache, "mutbatch-" + tag)
    shutil.rmtree(outdir, ignore_errors=True)
    os.makedirs(outdir)
    loop = ""
    for cid in ids:
        pk = "./pkg/sftp/ ./pkg/factory/" if cid in FACTORY else "./pkg/sftp/"
        loop += ("cd /src/%s/filesystem && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 CGO_ENABLED=1 "
                 "go test -count=1 -timeout 240s %s%s > /out/mutbatch-%s/%s.txt 2>&1; echo \"rc=$?\" >> /out/mutbatch-%s/%s.txt; ") % (cid, "-race " if cid in RACE else "", pk, tag, cid, tag, cid)
    cmd = ["bash", os.path.join(ROOT, "scripts/containers/run_pinned.sh"), "--out", cache, "--op-id", "sftpfixr3-batch-" + tag, "IMG-GO", "--", "sh", "-c", loop]
    for _ in range(120):
        rc, o = sh(cmd, cwd=root, timeout=6 * 3600)
        if "memory_budget_unavailable" in o or "disk_headroom" in o:
            time.sleep(30)
            continue
        break
    res = []
    for cid in ids:
        fp = os.path.join(outdir, cid + ".txt")
        txt = open(fp).read() if os.path.exists(fp) else ""
        m = re.search(r"rc=(\d+)\s*$", txt)
        mrc = int(m.group(1)) if m else 255
        verdict, fails = classify(mrc, txt) if m else ("ERROR", ["(no result: the batch container did not reach this mutant)"])
        res.append((cid, verdict, fails, txt))
    shutil.rmtree(root, ignore_errors=True)
    return res


def main():
    mode = sys.argv[1]
    if mode == "runbatch":  # runbatch <out-dir> <batch-size> [ID,ID,...]
        out = os.path.abspath(sys.argv[2])
        size = int(sys.argv[3])
        only = sys.argv[4].split(",") if len(sys.argv) > 4 else None
        cache = os.environ.get("FIXR3_CACHE", os.path.join(ROOT, ".audit", "out", "sftp-fix-r3"))
        os.makedirs(out, exist_ok=True)
        ids = [cid for cid, desc, edits in M if not only or cid in only]
        desc_of = {cid: desc for cid, desc, edits in M}
        results = []
        for i in range(0, len(ids), size):
            batch = ids[i:i + size]
            t0 = time.time()
            for cid, verdict, fails, txt in run_batch(batch, cache, str(i // size)):
                open(os.path.join(out, cid + ".txt"), "w").write(txt)
                results.append({"id": cid, "desc": desc_of[cid], "verdict": verdict, "failing": fails[:12], "n_failing": len(fails)})
                print(cid, verdict, len(fails), "failing", flush=True)
            json.dump(results, open(os.path.join(out, "results.json"), "w"), indent=1)
            print("batch", i // size, "done in", round(time.time() - t0), "s", flush=True)
        print("TOTAL", len(results), "KILLED", len([r for r in results if r["verdict"] == "KILLED"]), "SURVIVED", [r["id"] for r in results if r["verdict"] == "SURVIVED"],
              "INVALID/ERROR", [r["id"] for r in results if r["verdict"] in ("INVALID", "ERROR")])
        return
    if mode == "check":  # every pattern matches exactly `count` times in the checkout (no container, nothing written)
        bad = 0
        for cid, desc, edits in M:
            for f, old, new, count in edits:
                n = open(os.path.join(MOD, f)).read().count(old)
                if n != count:
                    bad += 1
                    print("MISMATCH", cid, f, n, "!=", count, repr(old[:70]))
        print("checked", len(M), "mutants,", bad, "mismatches")
        sys.exit(1 if bad else 0)
    if mode == "list":
        for cid, desc, edits in M:
            print(cid, len(edits), desc)
        return
    out = os.path.abspath(sys.argv[2])
    only = sys.argv[3].split(",") if len(sys.argv) > 3 else None
    cache = os.environ.get("FIXR3_CACHE", os.path.join(ROOT, ".audit", "out", "sftp-fix-r3"))
    os.makedirs(out, exist_ok=True)
    res = []
    for cid, desc, edits in M:
        if only and cid not in only:
            continue
        t0 = time.time()
        d = make_copy(cid)
        for f, old, new, count in edits:
            apply(d, f, old, new, count)
        rc, o = run_unit(d, cid, cache)
        verdict, fails = classify(rc, o)
        open(os.path.join(out, cid + ".txt"), "w").write(o)
        res.append({"id": cid, "desc": desc, "verdict": verdict, "failing": fails[:12], "n_failing": len(fails), "seconds": round(time.time() - t0)})
        print(cid, verdict, len(fails), "failing", flush=True)
        shutil.rmtree(d, ignore_errors=True)
        json.dump(res, open(os.path.join(out, "results.json"), "w"), indent=1)
    k = [r["id"] for r in res if r["verdict"] == "KILLED"]
    print("TOTAL", len(res), "KILLED", len(k), "SURVIVED", [r["id"] for r in res if r["verdict"] == "SURVIVED"], "INVALID/ERROR", [r["id"] for r in res if r["verdict"] in ("INVALID", "ERROR")])


main()
