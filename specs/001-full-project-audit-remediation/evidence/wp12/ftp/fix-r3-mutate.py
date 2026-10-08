#!/usr/bin/env python3
# identity: WP-12 PA-04 fix round 3 (WF24 review, 11.4.276): mutation harness (r2 set re-expressed on the round-3 code, the WF24 reviewer mutants NM01-NM17, one mutant per round-3 fix class) for submodules/filesystem/pkg/ftp (+ the pkg/factory ftp wiring)
# usage:   python3 -I fix-r3-mutate.py gen <work-dir>      write one mutated copy of the module per mutant under <work-dir>/mut/<id>/filesystem
#          python3 -I fix-r3-mutate.py report <work-dir>   read <work-dir>/mut/<id>.rc|.log (written by fix-r3-mutate-run.sh inside the pinned IMG-GO
#                                                           container) and print/write the verdict table (JSON to <work-dir>/mutants.json)
# Every mutant is ONE (or two, when stated) exact text replacement on a copy of the module, each asserted to match exactly once (a pattern that matches
# zero or several times aborts the generation: no silent no-op mutants). KILLED = the suite reports at least one `--- FAIL`; INVALID = it does not
# compile; KILLED-BY-TIMEOUT-ONLY = it timed out with no FAIL line (reported separately, not counted as a clean kill); SURVIVED = the suite passed.
# Reviewer mutants MX01..MX28 and PC16 are the WF21 review's own, NM01..NM17 the WF24 review's own (re-expressed on the current code: the edit sites moved, the INTENT is the reviewer's).
import json, os, re, shutil, subprocess, sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), *[".."] * 5))
MOD = os.path.join(ROOT, "submodules", "filesystem")
F, P, M, T, S, C, FA = "pkg/ftp/ftp.go", "pkg/ftp/proto.go", "pkg/ftp/mlsx.go", "pkg/ftp/tls.go", "pkg/ftp/scan.go", "pkg/ftp/cred.go", "pkg/factory/ftp_factory.go"
S2 = "pkg/ftp/loginguard.go"

# (id, description, [(file, old, new)], target package)
MUT = [
 ("N00", "NEGATIVE CONTROL: unmutated copy (must PASS = reported SURVIVED)", [], "pkg/ftp"),
 ("N00b", "NEGATIVE CONTROL: unmutated copy again (flake gauge under the same parallel load)", [], "pkg/ftp"),
 ("N00c", "NEGATIVE CONTROL: unmutated copy once more (flake gauge)", [], "pkg/ftp"),
 ("N01", "NEGATIVE CONTROL: behaviour-neutral edit, a comment (must SURVIVE)", [(F, "// TLS modes of Config.TLSMode.", "// TLS modes of Config.TLSMode (edited).")], "pkg/ftp"),
 ("PC16", "reviewer POSITIVE CONTROL: NUL no longer refused by confine (an old test covers it: must be KILLED)", [(F, 'if strings.ContainsAny(p, "\\r\\n\\x00") {', 'if strings.ContainsAny(p, "\\r\\n") {')], "pkg/ftp"),
 # ---- the 18 reviewer mutants ----
 ("MX01", "reviewer: Disconnect never aborts the open stream", [(F, "\tif abort != nil {\n\t\tabort()\n\t}\n", "\t_ = abort\n")], "pkg/ftp"),
 ("MX02", "reviewer: no ctx watcher on streams", [(F, "s.stop = context.AfterFunc(ctx, dc.abortNow)", "s.stop = func() bool { return true }")], "pkg/ftp"),
 ("MX03", "reviewer: base directory not canonicalised with PWD", [(F, "root, err = p.pwd(ctx)", "root, err = cfg.Path, error(nil)")], "pkg/ftp"),
 ("MX04", "reviewer: VerifyConnection pin re-check disabled", [(T, "\t\t\tgot := Fingerprint(cs.PeerCertificates[0])\n\t\t\tfor _, f := range fps {\n\t\t\t\tif f == NormalizeFingerprint(got) {\n\t\t\t\t\treturn nil\n\t\t\t\t}\n\t\t\t}\n\t\t\treturn &CertMismatchError{Host: host, Presented: got, Pinned: pinFingerprints(pins)}\n", "\t\t\treturn nil\n")], "pkg/ftp"),
 ('MX05', "reviewer: TLS ServerName ignores the pin's name", [(T, '\t\tif !named && p.ServerName != "" {', '\t\tif !named && p.ServerName != "" && false {')], 'pkg/ftp'),
 ("MX06", "reviewer: a network failure during login is classified AUTH", [(P, "\tif isNetErr(err) {\n\t\treturn fabric.MarkTransient(fmt.Errorf(\"ftp: %s: %w\", phase, err))\n\t}", "\tif isNetErr(err) {\n\t\treturn fabric.MarkAuth(fmt.Errorf(\"ftp: %s: %w\", phase, err))\n\t}")], "pkg/ftp"),
 ('MX07', 'reviewer: stream Close swallows a failing final reply after a FULL read', [(P, '\t\tif _, _, err := p.reply(ctx, "transfer", []int{226, 250}); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn p.trailing(ctx)', '\t\t_, _, _ = p.reply(ctx, "transfer", []int{226, 250})\n\t\treturn p.trailing(ctx)')], 'pkg/ftp'),
 ("MX08", "reviewer: server entry names containing '/' or NUL are not filtered", [(F, 'return n != "" && n != "." && n != ".." && !strings.ContainsAny(n, "/\\x00")', 'return n != "" && n != "." && n != ".."')], "pkg/ftp"),
 ("MX09", "reviewer: no QUIT / close after a failed login", [(F, "\tfail := func(err error) error {\n\t\tp.quit()\n\t\treturn err\n\t}", "\tfail := func(err error) error {\n\t\treturn err\n\t}")], "pkg/ftp"),
 ('MX10', 'reviewer: IOTimeout not applied', [(P, 'd := start.Add(budget)', 'd := start.Add(24 * time.Hour)')], 'pkg/ftp'),
 ("MX11", "reviewer: DiscoverCert without a deadline", [(T, "protoOpts{dialTimeout: timeout, ioTimeout: timeout, maxReply: DefaultMaxReplyBytes}", "protoOpts{ioTimeout: 24 * time.Hour, maxReply: DefaultMaxReplyBytes}")], "pkg/ftp"),
 ("MX12", "reviewer: Pin records ServerName = host", [(T, "name, err := serverNameFor(d.Leaf, host)", "name, err := host, error(nil)")], "pkg/ftp"),
 ("MX13", "reviewer: Degraded() not set at connect", [(F, "\tc.degraded = !p.mlstOK()\n", "")], "pkg/ftp"),
 ("MX14", "reviewer: FileInfo.Mode never set", [(F, "\t\tMode:  e.mode(),\n", "")], "pkg/ftp"),
 ("MX17", "reviewer: default scan retry policy = 1 attempt", [(S, "MaxAttempts: 3, BaseDelay: 200 * time.Millisecond", "MaxAttempts: 1, BaseDelay: 200 * time.Millisecond")], "pkg/ftp"),
 ("MX18", "reviewer: a lost connection is never closed", [(F, "\tgo p.quit()\n", "")], "pkg/ftp"),
 ("MX19", "reviewer: Disconnect drops the connection without QUIT/close", [(F, "\t\tp.quit() // a connection that is already gone cannot say goodbye: that is not a failure of Disconnect", "\t\t_ = p")], "pkg/ftp"),
 ("MX28", "reviewer: owner confirmation accepted when it is only a PREFIX of the fingerprint", [(T, "if NormalizeFingerprint(d.Fingerprint) != NormalizeFingerprint(conf.Fingerprint) {", "if !strings.HasPrefix(NormalizeFingerprint(d.Fingerprint), NormalizeFingerprint(conf.Fingerprint)) {")], "pkg/ftp"),
 # ---- one mutant per fix class (author-written) ----
 ('F1a', 'control reply reads carry no deadline', [(P, '\t\t_ = p.ctl.SetReadDeadline(p.deadlineFor(ctx, start, budget))\n', '\t\t_, _ = start, budget\n')], 'pkg/ftp'),
 ('F1b', 'data reads carry no deadline', [(P, '\t_ = d.Conn.SetReadDeadline(dl)\n', '\t_ = dl\n')], 'pkg/ftp'),
 ("F1c", "the connect-phase budget (DialTimeout) is not applied", [(P, "\tif !pe.IsZero() && pe.Before(d) {\n\t\td = pe\n\t}\n", "\t_ = pe\n")], "pkg/ftp"),
 ("F1d", "ctx cancellation does not interrupt a command", [(P, "\tstop := context.AfterFunc(ctx, p.interrupt)\n\tdefer stop()\n\tline := fmt.Sprintf(format, a...)", "\tstop := func() bool { return true }\n\tdefer stop()\n\tline := fmt.Sprintf(format, a...)")], "pkg/ftp"),
 ("F1e", "control writes carry no deadline", [(P, "\t_ = p.ctl.SetWriteDeadline(p.deadline(ctx))\n\tif p.aborted.Load() {\n\t\treturn 0, nil, p.fail(ctx, ErrAborted)\n\t}", "\tif p.aborted.Load() {\n\t\treturn 0, nil, p.fail(ctx, ErrAborted)\n\t}")], "pkg/ftp"),
 ('F1f', 'a control reply gets a fresh IOTimeout per read (a server dripping bytes is never timed out)', [(P, '_ = p.ctl.SetReadDeadline(p.deadlineFor(ctx, start, budget))', '_ = start\n\t\t_ = p.ctl.SetReadDeadline(p.deadlineFor(ctx, time.Now(), budget))')], 'pkg/ftp'),
 ("F2a", "an unexpected positive reply is accepted", [(P, "\tcase expect == nil || contains(expect, code):", "\tcase true:")], "pkg/ftp"),
 ("F2b", "a 421 does not make the connection unusable", [(P, "\t\tif code == 421 {\n\t\t\tp.markBroken() // the server is closing the control connection\n\t\t}\n", "")], "pkg/ftp"),
 ("F2c", "an early close is not checked for lock-step with NOOP", [(P, "\tif _, _, err = p.cmd(ctx, []int{200}, \"NOOP\"); err != nil {\n\t\tp.markBroken()\n\t}\n\treturn nil\n}", "\t_ = err\n\treturn nil\n}")], "pkg/ftp"),
 ('F2d', 'bytes nobody asked for are not detected before a command (the whole drain check, buffer AND kernel)', [(P, '\tif err := p.drain(ctx, "before "+verb, drainWait); err != nil {\n\t\treturn 0, nil, err\n\t}\n', '')], 'pkg/ftp'),
 ("F2e", "a 4yz/5yz reply to an early-close NOOP is not checked (any NOOP answer is taken)", [(P, "\tif _, _, err = p.cmd(ctx, []int{200}, \"NOOP\"); err != nil {", "\tif _, _, err = p.cmd(ctx, nil, \"NOOP\"); err != nil {")], "pkg/ftp"),
 ('F3a', 'a cancelled/aborted stream is still readable (abort checks removed from dconn.Read)', [(P, 'func (d *dconn) Read(b []byte) (int, error) {\n\tif d.aborted.Load() {\n\t\treturn 0, d.abortErr()\n\t}\n', 'func (d *dconn) Read(b []byte) (int, error) {\n'), (P, '\t_ = d.Conn.SetReadDeadline(dl)\n\tif d.aborted.Load() { // abort between the first check and arming: arming must not undo it\n\t\treturn 0, d.abortErr()\n\t}', '\t_ = d.Conn.SetReadDeadline(dl)')], 'pkg/ftp'),
 ("F4a", "OpenSeekable returns a short transfer as complete", [(F, "\t\tcase s.off < s.size:\n", "\t\tcase false:\n")], "pkg/ftp"),
 ("F4b", "OpenSeekable discards the final-reply error at EOF", [(F, "\t\tcase cerr != nil:\n\t\t\treturn n, cerr\n", "\t\tcase false:\n\t\t\treturn n, cerr\n")], "pkg/ftp"),
 ('F5a', 'no reply to PASS is retryable', [(P, '\treturn fmt.Errorf("ftp: connection failed after the password was sent (%v); not retried, a retry would send it again: %w", err, errPassUnknown)', '\treturn fabric.MarkTransient(fmt.Errorf("ftp: connection failed after the password was sent (%w)", err))')], 'pkg/ftp'),
 ("F5b", "421 at USER is an authentication failure", [(P, "te.Code == 530 || te.Code == 532 || te.Code == 332", "te.Code == 421 || te.Code == 530 || te.Code == 532 || te.Code == 332")], "pkg/ftp"),
 ('F5c', 'garbled names after a refused OPTS UTF8 ON are catalogued', [(F, '\tif !p.utf8On {\n\t\t// the session is not known', '\tif false {\n\t\t// the session is not known')], 'pkg/ftp'),
 ("F5d", "a PBSZ refusal is an authentication failure", [(P, "\t\t\treturn p.stepErr(ctx, \"PBSZ\", err)", "\t\t\treturn fabric.MarkAuth(err)")], "pkg/ftp"),
 ('F5e', 'a PASS rejection with an unlisted code is an authentication failure', [(P, '\t\treturn fmt.Errorf("ftp: PASS rejected: %d %s (not retried: a retry would send the password again): %w", te.Code, flat(te.Msg), errPassRejected)', '\t\treturn fabric.MarkAuth(fmt.Errorf("ftp: PASS rejected: %d %s", te.Code, flat(te.Msg)))')], 'pkg/ftp'),
 ("F5f", "a failed FEAT is fatal (a server without FEAT cannot log in)", [(P, "\t\tlines = nil\n\t} else if fcode != 211 {", "\t\treturn p.stepErr(ctx, \"FEAT\", err)\n\t} else if fcode != 211 {")], "pkg/ftp"),
 ("F6a", "MLSD type values are case-sensitive", [(M, "switch l := strings.ToLower(v); {", "switch l := v; {")], "pkg/ftp"),
 ("F6b", "MLSD sizes are parsed with base prefixes (010 is octal)", [(M, "u, err := strconv.ParseUint(v, 10, 64)", "u, err := strconv.ParseUint(v, 0, 64)")], "pkg/ftp"),
 ("F6c", "MLSD facts end at the first space (a space in a fact value breaks the entry)", [(M, "\t\ti := strings.Index(line, \"; \")\n\t\tif i < 0 {", "\t\ti := strings.Index(line, \" \") - 1\n\t\tif i < 0 {")], "pkg/ftp"),
 ("F6d", "cdir/pdir entries are listed as children", [(F, "if !visibleName(en.name) || en.kind == kindSelf || en.kind == kindParent {", "if !visibleName(en.name) {")], "pkg/ftp"),
 ('F6e', 'a listing line that cannot be parsed is silently dropped (a first line / a damaged entry too)', [(F, '\t\tif !a.sawEntry || (!degraded && strings.Contains(line, ";")) {\n\t\t\treturn perr\n\t\t}', '')], 'pkg/ftp'),
 ("F6f", "symlinks are not recognised", [(M, "\tcase strings.HasPrefix(l, \"os.unix=slink\"), strings.HasPrefix(l, \"os.unix=symlink\"):\n\t\treturn kindLink\n", "")], "pkg/ftp"),
 ('F6g', 'MLST accepts several entries silently', [(F, '\t\t\tif len(ents) != 1 {\n\t\t\t\tpr.markBroken()\n\t\t\t\treturn listViolation("MLST returned %d entries for one path", len(ents))\n\t\t\t}', '\t\t\tif len(ents) == 0 {\n\t\t\t\treturn perr\n\t\t\t}')], 'pkg/ftp'),
 ("F7a", "every 550 is reported as non-existence", [(M, "\treturn k550Unknown\n}", "\treturn k550Absent\n}")], "pkg/ftp"),
 ("F7b", "permission text is not recognised (denied reads as absent)", [(M, "\tfor _, s := range []string{\"permission\", \"denied\", \"not permitted\", \"forbidden\", \"not allowed\", \"privilege\", \"read-only\", \"read only\"} {", "\tfor _, s := range []string{} {")], "pkg/ftp"),
 ("F7c", "an ambiguous 550 is not settled by the parent listing", [(F, "classify550(te.Msg) == k550Unknown {", "classify550(te.Msg) == k550Unknown && false {")], "pkg/ftp"),
 ('F8a', 'degraded stat of a top-level entry lists with no argument', [(F, '\tentries, err := c.listEntries(ctx, p, path.Dir(logical), path.Dir(remote), degraded)', '\tentries, err := c.listEntries(ctx, p, path.Dir(logical), strings.TrimSuffix(path.Dir(remote), "/"), degraded)')], 'pkg/ftp'),
 ("F8b", "degraded stat of the root itself is a parent listing", [(F, "\t\tif logical == \"/\" { // the root itself: there is no parent to list", "\t\tif false { // the root itself: there is no parent to list")], "pkg/ftp"),
 ("F10a", "Credential's String has a pointer receiver again", [(C, "func (c Credential) String() string { return \"Credential(redacted)\" }", "func (c *Credential) String() string { return \"Credential(redacted)\" }"), (C, "func (c Credential) GoString() string { return c.String() }", "func (c *Credential) GoString() string { return c.String() }")], "pkg/ftp"),
 ("F11a", "a timed-out Disconnect leaves the connection to whoever comes next", [(F, "\tif stale != nil {\n\t\tstale.quit()\n\t}\n\t<-c.sem", "\t_ = stale\n\t<-c.sem")], "pkg/ftp"),
 ("F11b", "a re-dial does not close the connection it replaces", [(F, "\tif old != nil {\n\t\told.quit() // a connection left by a timed-out Disconnect must not leak\n\t}", "\t_ = old")], "pkg/ftp"),
 ("F12", "a 0xFF byte in a path is sent raw", [(P, "func escapeIAC(s string) string { return strings.ReplaceAll(s, \"\\xff\", \"\\xff\\xff\") }", "func escapeIAC(s string) string { return s }")], "pkg/ftp"),
 ("F13a", "the control reply size is not bounded", [(P, "\t\tif total > p.o.maxReply {", "\t\tif false {")], "pkg/ftp"),
 ('F13b', 'the listing size is not bounded', [(P, '\t\t\tif n > maxEntries || total > maxBytes {', '\t\t\tif false && (n > maxEntries || total > maxBytes) {')], 'pkg/ftp'),
 ("F14", "Pin/DiscoverCert port 0 is dialled as port 0", [(T, "\tif port == 0 {\n\t\tport = DefaultPort\n\t}\n\t// An empty trust pool", "\t// An empty trust pool")], "pkg/ftp"),
 ("F15", "a second server name for the same host is accepted by Pin", [(T, "if e.ServerName != \"\" && e.ServerName != name &&", "if false && e.ServerName != name &&")], "pkg/ftp"),
 ("F2f", "a stream failure is waited for instead of dropping the connection", [(F, "\t\tif s.rerr != nil || s.dc.aborted.Load() || s.ctx.Err() != nil {\n\t\t\ts.p.markBroken()\n\t\t\t_ = s.dc.Close()\n\t\t} else if", "\t\tif false {\n\t\t} else if")], "pkg/ftp"),
 # ---- the factory wiring (F16) ----
 ('W1', 'factory: the process-wide pin store is not passed', [(FA, '\t\tPinStore:          ftp.DeferredDefaultPinStore,\n', '')], 'pkg/factory'),
 ("W2", "factory: credential_ref is not passed", [(FA, "CredentialRef:     GetStringSetting(config.Settings, \"credential_ref\", \"\"),", "CredentialRef:     \"\",")], "pkg/factory"),
 ("W3", "factory: tls_mode is not passed", [(FA, "TLSMode:           GetStringSetting(config.Settings, \"tls_mode\", \"\"),", "TLSMode:           \"\",")], "pkg/factory"),
 ("W4", "factory: trusted_lan is always true", [(FA, "TrustedLAN:        trusted,", "TrustedLAN:        trusted || true,")], "pkg/factory"),
 ("W5", "factory: allow_degraded_list is not passed", [(FA, "AllowDegradedList: degraded,", "AllowDegradedList: degraded && false,")], "pkg/factory"),
 ("W6", "factory: an inline password is accepted again", [(FA, "[]string{\"password\", \"passphrase\", \"private_key\", \"privatekey\", \"key\", \"secret\", \"token\"}", "[]string{\"passphrase\"}")], "pkg/factory"),
 ("W7", "factory: the port is not validated", [(FA, "if !isNum || n < 1 || n > 65535 {", "if !isNum {")], "pkg/factory"),
 ("W8", "factory: the scan client is replaced by the raw client", [(FA, "\treturn ftp.NewScanClient(&ftp.Config{", "\treturn rawFTP(&ftp.Config{"), (FA, "// boolSetting reads a boolean setting", "func rawFTP(c *ftp.Config, _ ftp.ScanOptions) (client.Client, error) { return ftp.NewFTPClient(c), nil }\n\n// boolSetting reads a boolean setting")], "pkg/factory"),
 ("W9", "factory: a quoted boolean is accepted", [(FA, "\tb, isBool := v.(bool)\n\tif !isBool {\n\t\treturn false, fmt.Errorf(\"ftp: setting %q must be a boolean\", key)\n\t}\n\treturn b, nil", "\tb, _ := v.(bool)\n\treturn b, nil")], "pkg/factory"),
 # ---- the WF24 reviewer's own mutants NM01-NM17 (his positive control PC01 is F12), re-expressed on the round-3 code ----
 ("NM01", "WF24: cmd() no longer refuses CR/LF/NUL (Config.Path reaches the wire unchecked)", [(P, "\tif strings.ContainsAny(line, \"\\r\\n\\x00\") {\n\t\treturn 0, nil, fmt.Errorf(\"%w: control characters in a %s command\", ErrPathEscape, verb)\n\t}\n", "")], "pkg/ftp"),
 ("NM02", "WF24: data-channel TLS does not verify the pinned certificate (the control channel still does)", [(P, "\t\tnc = tls.Client(c, p.tlsCfg) // the handshake runs on the first Read/Write, i.e. after the transfer command", "\t\tdcfg := p.tlsCfg.Clone()\n\t\tdcfg.VerifyConnection = nil\n\t\tdcfg.InsecureSkipVerify = true\n\t\tnc = tls.Client(c, dcfg)")], "pkg/ftp"),
 ("NM03", "WF24: a multi-line reply ends at ANY 'NNN ' line, not only at its own code", [(P, "\t\tif len(line) >= 3 && line[:3] == code && (len(line) == 3 || line[3] == ' ') {", "\t\tif len(line) >= 3 && isDigits(line[:3]) && (len(line) == 3 || line[3] == ' ') {")], "pkg/ftp"),
 ("NM04", "WF24: a TLS data stream without close_notify is an error (truncation mapping removed)", [(P, "\t\tif d.tls && errors.Is(err, io.ErrUnexpectedEOF) {", "\t\tif false && d.tls && errors.Is(err, io.ErrUnexpectedEOF) {")], "pkg/ftp"),
 ("NM05", "WF24: visibleName no longer hides '.' and '..'", [(F, "\treturn n != \"\" && n != \".\" && n != \"..\" && !strings.ContainsAny(n, \"/\\x00\")", "\treturn n != \"\" && !strings.ContainsAny(n, \"/\\x00\")")], "pkg/ftp"),
 ("NM06", "WF24: the factory drops disable_epsv", [(FA, "\t\tDisableEPSV:       noEPSV,", "\t\tDisableEPSV:       false && noEPSV,")], "pkg/factory"),
 ("NM07", "WF24: an empty password from the resolver is accepted", [(F, "\t\tif cred.Password == \"\" {", "\t\tif false && cred.Password == \"\" {")], "pkg/ftp"),
 ("NM08", "WF24: FEAT keys are not upper-cased", [(P, "\t\t\tp.feats[strings.ToUpper(k)] = v", "\t\t\tp.feats[k] = v")], "pkg/ftp"),
 ("NM09", "WF24: PASS is sent even when USER already answered 230", [(P, "\tif code == 331 {", "\tif code == 331 || code == 230 {")], "pkg/ftp"),
 ("NM10", "WF24: startTLS no longer refuses bytes buffered before the handshake (STARTTLS injection guard)", [(P, "\tif p.br.Buffered() > 0 {\n\t\tp.markBroken()\n\t\treturn fmt.Errorf(\"%w: data received before the TLS handshake\", ErrProtocol)\n\t}\n", "")], "pkg/ftp"),
 ("NM11", "WF24: GetFileInfo keeps the server's MLST pathname as the entry name", [(F, "\t\t\te0.name = path.Base(logical)\n", "")], "pkg/ftp"),
 ("NM12", "WF24: when the names are checked only 0x7f is caught, invalid UTF-8 passes", [(F, "\t\t\tif !utf8.ValidString(e.name) || strings.ContainsRune(e.name, 0x7f) {", "\t\t\tif (false && !utf8.ValidString(e.name)) || strings.ContainsRune(e.name, 0x7f) {")], "pkg/ftp"),
 ("NM13", "WF24: the seeker keeps the stream (and the session) after the announced size was read", [(F, "\tcase s.off >= s.size:\n\t\t// everything announced", "\tcase false && s.off >= s.size:\n\t\t// everything announced")], "pkg/ftp"),
 ("NM14", "WF24: reply() is not interrupted by ctx", [(P, "func (p *proto) replyBudget(ctx context.Context, verb string, expect []int, budget time.Duration) (int, []string, error) {\n\tstop := context.AfterFunc(ctx, p.interrupt)\n", "func (p *proto) replyBudget(ctx context.Context, verb string, expect []int, budget time.Duration) (int, []string, error) {\n\tstop := func() bool { return false }\n")], "pkg/ftp"),
 ("NM15", "WF24: connect() never ends the connect phase (every later I/O capped at connect time + DialTimeout)", [(F, "\tp.endPhase()\n", "")], "pkg/ftp"),
 ("NM16", "WF24: DiscoverCert leaks its connection (hardClose removed)", [(T, "\tdefer p.hardClose()\n", "\t_ = p.hardClose\n")], "pkg/ftp"),
 ("NM17", "WF24: DefaultMaxListEntries 1<<20 -> 1<<10 (a 1025-entry directory fails by default)", [(P, "DefaultMaxListEntries = 1 << 20 ", "DefaultMaxListEntries = 1 << 10 ")], "pkg/ftp"),
 # ---- one mutant per round-3 fix class (author-written) ----
 ("K1b", "MLST reply of the wrong shape (no entry) does not drop the connection", [(F, "\t\t\t\t// reply could be one behind - the connection is dropped (WF24 G1(b))\n\t\t\t\tpr.markBroken()\n\t\t\t\treturn perr", "\t\t\t\treturn perr")], "pkg/ftp"),
 ("K1c", "MLST with several entries does not drop the connection", [(F, "\t\t\tif len(ents) != 1 {\n\t\t\t\tpr.markBroken()\n", "\t\t\tif len(ents) != 1 {\n")], "pkg/ftp"),
 ("K1d", "the MLST pathname is not compared with the requested path", [(F, "\t\t\tif logical != \"/\" && !mlstNameMatches(e0.name, remote) {", "\t\t\tif false && logical != \"/\" && !mlstNameMatches(e0.name, remote) {")], "pkg/ftp"),
 ("K1d2", "mlstNameMatches is case-sensitive (a case-folding server is refused)", [(M, "return strings.EqualFold(base, path.Base(remote))", "return base == path.Base(remote)")], "pkg/ftp"),
 ("K1e", "an unreadable SIZE reply does not drop the connection", [(F, "\t\t\t// a 213 whose text is not a size is not the answer to our SIZE: the lock-step is in doubt (WF24 K1.e)\n\t\t\tpr.markBroken()\n", "")], "pkg/ftp"),
 ("K1f", "an unreadable PWD reply does not drop the connection", [(P, "\t\tp.markBroken()\n\t\treturn \"\", protoViolation(\"unsupported PWD reply %q\", flat(s))\n\t}\n\tvar b strings.Builder", "\t\treturn \"\", protoViolation(\"unsupported PWD reply %q\", flat(s))\n\t}\n\tvar b strings.Builder")], "pkg/ftp"),
 ("K1g", "an unreadable EPSV reply falls back to PASV instead of dropping the connection", [(P, "\t\t\tp.markBroken()\n\t\t\treturn 0, protoViolation(\"unusable EPSV reply\")\n", "\t\t\tp.skipEPSV = true\n\t\t\t_ = lines\n")], "pkg/ftp"),
 ("K1h", "EPSV delimiters other than '|' are refused", [(P, "if i, j := strings.Index(line, \"(\"), strings.LastIndex(line, \")\"); i >= 0 && j > i+4 {", "if i, j := strings.Index(line, \"(\"), strings.LastIndex(line, \")\"); false && i >= 0 && j > i+4 {")], "pkg/ftp"),
 ("K1i", "a reply behind the final reply of a complete transfer is not looked for", [(P, "\t\treturn p.trailing(ctx)", "\t\treturn nil")], "pkg/ftp"),
 ("K1i2", "a negative reply behind the final reply is not returned as the verdict", [(P, "\tif err == nil && code >= 400 {\n\t\treturn &textproto.Error{Code: code, Msg: strings.Join(lines, \"\\n\")}\n\t}\n\treturn nil\n}", "\t_, _, _ = code, lines, err\n\treturn nil\n}")], "pkg/ftp"),
 ("K1j", "a lock-step violation is permanent again (never retried)", [(P, "\treturn fabric.MarkTransient(fmt.Errorf(\"%w: \"+format, append([]any{ErrProtocol}, a...)...))", "\treturn fmt.Errorf(\"%w: \"+format, append([]any{ErrProtocol}, a...)...)")], "pkg/ftp"),
 ("K1k", "the drain check cannot see the kernel buffer (only the bufio buffer)", [(P, "\t_, err := p.br.Peek(1)\n\tif p.aborted.Load() {", "\tvar err error = context.DeadlineExceeded\n\tif p.aborted.Load() {")], "pkg/ftp"),
 ("K2a", "the server's text sits in the LEAF of a 4yz phase error (Classify reads it)", [(P, "return fabric.MarkTransient(fmt.Errorf(\"ftp: %s: %d %s: %w\", phase, te.Code, flat(te.Msg), errPhaseTransient))", "return fabric.MarkTransient(fmt.Errorf(\"ftp: %s: %d %s\", phase, te.Code, flat(te.Msg)))")], "pkg/ftp"),
 ("K2b", "the server's text sits in the LEAF of a 5yz phase error", [(P, "return fmt.Errorf(\"ftp: %s refused by the server: %d %s: %w\", phase, te.Code, flat(te.Msg), errPhaseRefused)", "return fmt.Errorf(\"ftp: %s refused by the server: %d %s\", phase, te.Code, flat(te.Msg))")], "pkg/ftp"),
 ("K2c", "a USER-phase 530 is an authentication failure again", [(P, "\t\treturn fmt.Errorf(\"ftp: login refused at USER (no password was sent): %d %s: %w\", te.Code, flat(te.Msg), errUserRefused)", "\t\treturn fabric.MarkAuth(fmt.Errorf(\"ftp: login refused at USER: %d %s\", te.Code, flat(te.Msg)))")], "pkg/ftp"),
 ("K2d", "AUTH TLS refusal carries the server text in the leaf", [(P, "return fmt.Errorf(\"ftp: AUTH TLS refused: %d %s: %w\", te.Code, flat(te.Msg), ErrTLSNotOffered)", "return fmt.Errorf(\"ftp: AUTH TLS refused: %d %s\", te.Code, flat(te.Msg))")], "pkg/ftp"),
 ("K2e", "a PASS rejected with a non-credential code starts no back-off", [(P, "\t\tnoteAmbiguousLogin(p.loginKey, p.o.loginBackoff)\n\t\treturn fmt.Errorf(\"ftp: PASS rejected", "\t\treturn fmt.Errorf(\"ftp: PASS rejected")], "pkg/ftp"),
 ("K2f", "an unanswered PASS starts no back-off", [(P, "\tif written {\n\t\tnoteAmbiguousLogin(p.loginKey, p.o.loginBackoff)\n\t}\n\treturn fmt.Errorf(\"ftp: connection failed after", "\treturn fmt.Errorf(\"ftp: connection failed after")], "pkg/ftp"),
 ("K2g", "a PASS interrupted by the context starts no back-off", [(P, "\t\tif written {\n\t\t\tnoteAmbiguousLogin(p.loginKey, p.o.loginBackoff)\n\t\t}\n\t\treturn fmt.Errorf(\"ftp: login: %w\", ce)", "\t\t_ = written\n\t\treturn fmt.Errorf(\"ftp: login: %w\", ce)")], "pkg/ftp"),
 ("K2h", "connect does not consult the login back-off", [(F, "\tif rem := loginBackoffs.remaining(key); rem > 0 {", "\tif rem := loginBackoffs.remaining(key); false && rem > 0 {")], "pkg/ftp"),
 ("K2i", "a good login does not end the back-off", [(F, "\tloginBackoffs.clear(key) // a credential verdict was reached: any earlier doubt is over\n", "")], "pkg/ftp"),
 ("K2j", "the back-off ignores its TTL (a thousand hours)", [(S2, "\tg.until[key] = now.Add(d)", "\tg.until[key] = now.Add(1000 * time.Hour)")], "pkg/ftp"),
 ("K2k", "the default back-off (LoginBackoff 0) is zero", [(S2, "\tif d == 0 {\n\t\td = DefaultLoginBackoff\n\t}", "")], "pkg/ftp"),
 ("K2l", "ClearLoginBackoff does nothing", [(S2, "\tloginBackoffs.clear(loginKeyFor(host, port, username))", "\t_ = loginKeyFor")], "pkg/ftp"),
 ("K3a", "the seeker at SIZE drops the stream through the early-close path (any final reply accepted)", [(F, "\t\tif cerr := s.finishAtEnd(); cerr != nil {", "\t\tif cerr := s.dropStream(); cerr != nil {")], "pkg/ftp"),
 ("K3b", "an early close waits a whole IOTimeout again", [(P, "\twait := p.o.ioTimeout\n\tif wait > earlyCloseWait {\n\t\twait = earlyCloseWait\n\t}\n", "\twait := p.o.ioTimeout\n")], "pkg/ftp"),
 ("K3c", "the end-of-data wait of the seeker is a whole IOTimeout", [(F, "\t\tst.dc.maxWait.Store(int64(earlyCloseWait))\n", "")], "pkg/ftp"),
 ("K4a", "the listing has no byte budget", [(P, "\t\t\tif n > maxEntries || total > maxBytes {", "\t\t\tif n > maxEntries {")], "pkg/ftp"),
 ("K4b", "the control line bound is back to 4096 (a listed long name cannot be stat'ed)", [(P, "\tcontrolLineBytes = 2 * maxLineBytes", "\tcontrolLineBytes = 4096")], "pkg/ftp"),
 ("K4c", "the connect budget starts after the dial", [(P, "\tp.phaseEnd = phaseEnd\n", "\t_ = phaseEnd\n\tif o.dialTimeout > 0 {\n\t\tp.phaseEnd = time.Now().Add(o.dialTimeout)\n\t}\n")], "pkg/ftp"),
 ("K5", "connect publishes its connection even after a Disconnect ran", [(F, "\tif c.gen != gen {", "\tif false && c.gen != gen {")], "pkg/ftp"),
 ("K5b", "Disconnect does not bump the generation", [(F, "\tc.state = stateDown\n\tc.gen++\n", "\tc.state = stateDown\n")], "pkg/ftp"),
 ("K6a", "the entry in front of a fragment is kept (a truncated name is listed)", [(F, "\t\tif a.lastAppended {\n\t\t\ta.entries = a.entries[:len(a.entries)-1]\n\t\t\ta.truncated++\n\t\t\ta.lastAppended = false\n\t\t}\n", "")], "pkg/ftp"),
 ("K6b", "a listing with too many fragments is accepted", [(F, "\tif a.fragments > 8 && a.fragments*10 > a.lines {", "\tif false && a.fragments > 8 && a.fragments*10 > a.lines {")], "pkg/ftp"),
 ("K6c", "a name with CR/LF/NUL is listed", [(F, "\tif strings.ContainsAny(e.name, \"\\r\\n\\x00\") {", "\tif false && strings.ContainsAny(e.name, \"\\r\\n\\x00\") {")], "pkg/ftp"),
 ("K6d", "a fragment ends the listing with an error again (the NAS defect)", [(F, "\t\ta.fragments++\n", "\t\treturn perr\n")], "pkg/ftp"),
 ("K6e", "no warning is delivered for skipped entries", [(F, "\tif w := acc.warning(logical); w != nil {\n\t\tc.noteWarning(*w)\n\t}\n", "")], "pkg/ftp"),
 ("K6f", "names are checked only after a refused OPTS (the utf8On flag is never set but also never checked)", [(F, "\tif !p.utf8On {\n\t\t// the session is not known", "\tif !p.utf8On && p.feats[\"UTF8\"] != \"\" && false {\n\t\t// the session is not known")], "pkg/ftp"),
 ("K6g", "the second pin's name wins again", [(T, "\t\tif !named && p.ServerName != \"\" {", "\t\tif (!named || serverName == host) && p.ServerName != \"\" {")], "pkg/ftp"),
 ("K6h", "MemPinStore.Record accepts a second name", [(T, "\tif err := checkPinName(s.pins[hostport], pin); err != nil {\n\t\treturn err\n\t}\n", "")], "pkg/ftp"),
 ("K6i", "FilePinStore.Record accepts a second name", [(T, "\tif err := checkPinName(m[hostport], pin); err != nil {\n\t\treturn err\n\t}\n", "")], "pkg/ftp"),
 ("K6j", "the deferred store captures DefaultPinStore at init", [(T, "func (deferredPinStore) get() (PinStore, error) {\n\tif s := DefaultPinStore; s != nil {", "var capturedAtInit = DefaultPinStore\n\nfunc (deferredPinStore) get() (PinStore, error) {\n\tif s := capturedAtInit; s != nil {")], "pkg/ftp"),
 ("K6k", "PublicConfig does not show disable_epsv", [(F, " DisableEPSV: c.config.DisableEPSV,", "")], "pkg/ftp"),
 ("W10", "factory: the pin store is captured at creation again", [(FA, "\t\tPinStore:          ftp.DeferredDefaultPinStore,\n", "\t\tPinStore:          ftp.DefaultPinStore,\n")], "pkg/factory"),
]


def gen(work):
    bad = []
    shutil.rmtree(os.path.join(work, "mut"), ignore_errors=True)
    for mid, desc, edits, target in MUT:
        d = os.path.join(work, "mut", mid, "filesystem")
        os.makedirs(d)
        subprocess.run("tar --exclude=.git -C %s -cf - . | tar -C %s -xf -" % (MOD, d), shell=True, check=True)
        for f, old, new in edits:
            p = os.path.join(d, f)
            s = open(p).read()
            n = s.count(old)
            if n != 1:
                print("BAD PATTERN", mid, f, "matches", n)
                bad.append(mid)
                continue
            open(p, "w").write(s.replace(old, new))
        open(os.path.join(work, "mut", mid, "target"), "w").write(target + "\n")
    print("generated", len(MUT), "bad", len(bad))
    if bad:
        sys.exit(1)


def report(work):
    rows = []
    for mid, desc, edits, target in MUT:
        rc_f, log_f = (os.path.join(work, "mut", mid + ext) for ext in (".rc", ".log"))
        if not os.path.exists(rc_f):
            rows.append({"id": mid, "verdict": "NOT-RUN", "desc": desc})
            continue
        rc = open(rc_f).read().strip()
        log = open(log_f, errors="replace").read()
        fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", log, re.M)))
        if "[build failed]" in log or re.search(r"^# digital\.vasic", log, re.M):
            v = "INVALID"
        elif rc == "0":
            v = "SURVIVED"
        elif fails:
            v = "KILLED"
        elif "panic: test timed out" in log:
            v = "KILLED-BY-TIMEOUT-ONLY"
        else:
            v = "KILLED-NO-FAIL-LINE"
        rows.append({"id": mid, "verdict": v, "desc": desc, "target": target, "fails": fails[:6], "n_fail": len(fails)})
    json.dump(rows, open(os.path.join(work, "mutants.json"), "w"), indent=1)
    for r in rows:
        print("%-5s %-24s %2s  %s  %s" % (r["id"], r["verdict"], r.get("n_fail", ""), r["desc"][:80], ",".join(r.get("fails", [])[:2])))
    from collections import Counter
    print(Counter(r["verdict"] for r in rows))


if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[1] not in ("gen", "report"):
        sys.exit(__doc__)
    {"gen": gen, "report": report}[sys.argv[1]](os.path.abspath(sys.argv[2]))
