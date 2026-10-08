#!/usr/bin/env python3
# identity: WP-12 PA-04 fix round 2 (WF21 review, 11.4.276): mutation harness for submodules/filesystem/pkg/ftp (+ the pkg/factory ftp wiring)
# usage:   python3 -I fix-r2-mutate.py gen <work-dir>      write one mutated copy of the module per mutant under <work-dir>/mut/<id>/filesystem
#          python3 -I fix-r2-mutate.py report <work-dir>   read <work-dir>/mut/<id>.rc|.log (written by fix-r2-mutate-run.sh inside the pinned IMG-GO
#                                                           container) and print/write the verdict table (JSON to <work-dir>/mutants.json)
# Every mutant is ONE (or two, when stated) exact text replacement on a copy of the module, each asserted to match exactly once (a pattern that matches
# zero or several times aborts the generation: no silent no-op mutants). KILLED = the suite reports at least one `--- FAIL`; INVALID = it does not
# compile; KILLED-BY-TIMEOUT-ONLY = it timed out with no FAIL line (reported separately, not counted as a clean kill); SURVIVED = the suite passed.
# Reviewer mutants MX01..MX28 and PC16 are the WF21 review's own (re-expressed on the rewritten code: the edit sites moved, the INTENT is the reviewer's).
import json, os, re, shutil, subprocess, sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), *[".."] * 5))
MOD = os.path.join(ROOT, "submodules", "filesystem")
F, P, M, T, S, C, FA = "pkg/ftp/ftp.go", "pkg/ftp/proto.go", "pkg/ftp/mlsx.go", "pkg/ftp/tls.go", "pkg/ftp/scan.go", "pkg/ftp/cred.go", "pkg/factory/ftp_factory.go"

# (id, description, [(file, old, new)], target package)
MUT = [
 ("N00", "NEGATIVE CONTROL: unmutated copy (must PASS = reported SURVIVED)", [], "pkg/ftp"),
 ("N01", "NEGATIVE CONTROL: behaviour-neutral edit, a comment (must SURVIVE)", [(F, "// TLS modes of Config.TLSMode.", "// TLS modes of Config.TLSMode (edited).")], "pkg/ftp"),
 ("PC16", "reviewer POSITIVE CONTROL: NUL no longer refused by confine (an old test covers it: must be KILLED)", [(F, 'if strings.ContainsAny(p, "\\r\\n\\x00") {', 'if strings.ContainsAny(p, "\\r\\n") {')], "pkg/ftp"),
 # ---- the 18 reviewer mutants ----
 ("MX01", "reviewer: Disconnect never aborts the open stream", [(F, "\tif abort != nil {\n\t\tabort()\n\t}\n", "\t_ = abort\n")], "pkg/ftp"),
 ("MX02", "reviewer: no ctx watcher on streams", [(F, "s.stop = context.AfterFunc(ctx, dc.abortNow)", "s.stop = func() bool { return true }")], "pkg/ftp"),
 ("MX03", "reviewer: base directory not canonicalised with PWD", [(F, "root, err = p.pwd(ctx)", "root, err = cfg.Path, error(nil)")], "pkg/ftp"),
 ("MX04", "reviewer: VerifyConnection pin re-check disabled", [(T, "\t\t\tgot := Fingerprint(cs.PeerCertificates[0])\n\t\t\tfor _, f := range fps {\n\t\t\t\tif f == NormalizeFingerprint(got) {\n\t\t\t\t\treturn nil\n\t\t\t\t}\n\t\t\t}\n\t\t\treturn &CertMismatchError{Host: host, Presented: got, Pinned: pinFingerprints(pins)}\n", "\t\t\treturn nil\n")], "pkg/ftp"),
 ("MX05", "reviewer: TLS ServerName ignores the pin's name", [(T, "\t\tif serverName == host && p.ServerName != \"\" {\n\t\t\tserverName = p.ServerName\n\t\t}\n", "\t\t_ = p\n")], "pkg/ftp"),
 ("MX06", "reviewer: a network failure during login is classified AUTH", [(P, "\tif isNetErr(err) {\n\t\treturn fabric.MarkTransient(fmt.Errorf(\"ftp: %s: %w\", phase, err))\n\t}", "\tif isNetErr(err) {\n\t\treturn fabric.MarkAuth(fmt.Errorf(\"ftp: %s: %w\", phase, err))\n\t}")], "pkg/ftp"),
 ("MX07", "reviewer: stream Close swallows a failing final reply after a FULL read", [(P, "\t\t_, _, err := p.reply(ctx, \"transfer\", []int{226, 250})\n\t\treturn err\n", "\t\t_, _, _ = p.reply(ctx, \"transfer\", []int{226, 250})\n\t\treturn nil\n")], "pkg/ftp"),
 ("MX08", "reviewer: server entry names containing '/' or NUL are not filtered", [(F, 'return n != "" && n != "." && n != ".." && !strings.ContainsAny(n, "/\\x00")', 'return n != "" && n != "." && n != ".."')], "pkg/ftp"),
 ("MX09", "reviewer: no QUIT / close after a failed login", [(F, "\tfail := func(err error) error {\n\t\tp.quit()\n\t\treturn err\n\t}", "\tfail := func(err error) error {\n\t\treturn err\n\t}")], "pkg/ftp"),
 ("MX10", "reviewer: IOTimeout not applied", [(P, "d := start.Add(p.o.ioTimeout)", "d := start.Add(24 * time.Hour)")], "pkg/ftp"),
 ("MX11", "reviewer: DiscoverCert without a deadline", [(T, "protoOpts{dialTimeout: timeout, ioTimeout: timeout, maxReply: DefaultMaxReplyBytes}", "protoOpts{ioTimeout: 24 * time.Hour, maxReply: DefaultMaxReplyBytes}")], "pkg/ftp"),
 ("MX12", "reviewer: Pin records ServerName = host", [(T, "name, err := serverNameFor(d.Leaf, host)", "name, err := host, error(nil)")], "pkg/ftp"),
 ("MX13", "reviewer: Degraded() not set at connect", [(F, "\tc.degraded = !p.mlstOK()\n", "")], "pkg/ftp"),
 ("MX14", "reviewer: FileInfo.Mode never set", [(F, "\t\tMode:  e.mode(),\n", "")], "pkg/ftp"),
 ("MX17", "reviewer: default scan retry policy = 1 attempt", [(S, "MaxAttempts: 3, BaseDelay: 200 * time.Millisecond", "MaxAttempts: 1, BaseDelay: 200 * time.Millisecond")], "pkg/ftp"),
 ("MX18", "reviewer: a lost connection is never closed", [(F, "\tgo p.quit()\n", "")], "pkg/ftp"),
 ("MX19", "reviewer: Disconnect drops the connection without QUIT/close", [(F, "\t\tp.quit() // a connection that is already gone cannot say goodbye: that is not a failure of Disconnect", "\t\t_ = p")], "pkg/ftp"),
 ("MX28", "reviewer: owner confirmation accepted when it is only a PREFIX of the fingerprint", [(T, "if NormalizeFingerprint(d.Fingerprint) != NormalizeFingerprint(conf.Fingerprint) {", "if !strings.HasPrefix(NormalizeFingerprint(d.Fingerprint), NormalizeFingerprint(conf.Fingerprint)) {")], "pkg/ftp"),
 # ---- one mutant per fix class (author-written) ----
 ("F1a", "control reply reads carry no deadline", [(P, "\t\t_ = p.ctl.SetReadDeadline(p.deadlineFrom(ctx, start))\n\t\tif p.aborted.Load() {\n\t\t\treturn 0, nil, ErrAborted\n\t\t}", "\t\t_ = start\n\t\tif p.aborted.Load() {\n\t\t\treturn 0, nil, ErrAborted\n\t\t}")], "pkg/ftp"),
 ("F1b", "data reads carry no deadline", [(P, "\t_ = d.Conn.SetReadDeadline(d.p.deadline(d.ctx))\n", "")], "pkg/ftp"),
 ("F1c", "the connect-phase budget (DialTimeout) is not applied", [(P, "\tif !pe.IsZero() && pe.Before(d) {\n\t\td = pe\n\t}\n", "\t_ = pe\n")], "pkg/ftp"),
 ("F1d", "ctx cancellation does not interrupt a command", [(P, "\tstop := context.AfterFunc(ctx, p.interrupt)\n\tdefer stop()\n\tline := fmt.Sprintf(format, a...)", "\tstop := func() bool { return true }\n\tdefer stop()\n\tline := fmt.Sprintf(format, a...)")], "pkg/ftp"),
 ("F1e", "control writes carry no deadline", [(P, "\t_ = p.ctl.SetWriteDeadline(p.deadline(ctx))\n\tif p.aborted.Load() {\n\t\treturn 0, nil, p.fail(ctx, ErrAborted)\n\t}", "\tif p.aborted.Load() {\n\t\treturn 0, nil, p.fail(ctx, ErrAborted)\n\t}")], "pkg/ftp"),
 ("F1f", "a control reply gets a fresh IOTimeout per read (a server dripping bytes is never timed out)", [(P, "_ = p.ctl.SetReadDeadline(p.deadlineFrom(ctx, start))", "_ = start\n\t\t_ = p.ctl.SetReadDeadline(p.deadline(ctx))")], "pkg/ftp"),
 ("F2a", "an unexpected positive reply is accepted", [(P, "\tcase expect == nil || contains(expect, code):", "\tcase true:")], "pkg/ftp"),
 ("F2b", "a 421 does not make the connection unusable", [(P, "\t\tif code == 421 {\n\t\t\tp.markBroken() // the server is closing the control connection\n\t\t}\n", "")], "pkg/ftp"),
 ("F2c", "an early close is not checked for lock-step with NOOP", [(P, "\tif _, _, err = p.cmd(ctx, []int{200}, \"NOOP\"); err != nil {\n\t\tp.markBroken()\n\t}\n\treturn nil\n}", "\t_ = err\n\treturn nil\n}")], "pkg/ftp"),
 ("F2d", "bytes nobody asked for are not detected before a command", [(P, "\tif p.br.Buffered() > 0 {\n\t\t// bytes nobody asked for", "\tif false {\n\t\t// bytes nobody asked for")], "pkg/ftp"),
 ("F2e", "a 4yz/5yz reply to an early-close NOOP is not checked (any NOOP answer is taken)", [(P, "\tif _, _, err = p.cmd(ctx, []int{200}, \"NOOP\"); err != nil {", "\tif _, _, err = p.cmd(ctx, nil, \"NOOP\"); err != nil {")], "pkg/ftp"),
 ("F3a", "a cancelled/aborted stream is still readable (abort checks removed from dconn.Read)", [(P, "func (d *dconn) Read(b []byte) (int, error) {\n\tif d.aborted.Load() {\n\t\treturn 0, d.abortErr()\n\t}\n\t_ = d.Conn.SetReadDeadline(d.p.deadline(d.ctx))\n\tif d.aborted.Load() { // abort between the first check and arming: arming must not undo it\n\t\treturn 0, d.abortErr()\n\t}", "func (d *dconn) Read(b []byte) (int, error) {\n\t_ = d.Conn.SetReadDeadline(d.p.deadline(d.ctx))")], "pkg/ftp"),
 ("F4a", "OpenSeekable returns a short transfer as complete", [(F, "\t\tcase s.off < s.size:\n", "\t\tcase false:\n")], "pkg/ftp"),
 ("F4b", "OpenSeekable discards the final-reply error at EOF", [(F, "\t\tcase cerr != nil:\n\t\t\treturn n, cerr\n", "\t\tcase false:\n\t\t\treturn n, cerr\n")], "pkg/ftp"),
 ("F5a", "no reply to PASS is retryable", [(P, "\treturn fmt.Errorf(\"ftp: connection failed after the password was sent (%v); not retried, a retry would send it again\", err)", "\treturn fabric.MarkTransient(fmt.Errorf(\"ftp: connection failed after the password was sent (%w)\", err))")], "pkg/ftp"),
 ("F5b", "421 at USER is an authentication failure", [(P, "te.Code == 530 || te.Code == 532 || te.Code == 332", "te.Code == 421 || te.Code == 530 || te.Code == 532 || te.Code == 332")], "pkg/ftp"),
 ("F5c", "garbled names after a refused OPTS UTF8 ON are catalogued", [(F, "\tif p.utf8Refused {\n\t\tfor _, e := range out {", "\tif false {\n\t\tfor _, e := range out {")], "pkg/ftp"),
 ("F5d", "a PBSZ refusal is an authentication failure", [(P, "\t\t\treturn p.stepErr(ctx, \"PBSZ\", err)", "\t\t\treturn fabric.MarkAuth(err)")], "pkg/ftp"),
 ("F5e", "a PASS rejection with an unlisted code is an authentication failure", [(P, "\treturn fmt.Errorf(\"ftp: PASS rejected: %d %s (not retried: a retry would send the password again)\", te.Code, flat(te.Msg))", "\treturn fabric.MarkAuth(fmt.Errorf(\"ftp: PASS rejected: %d %s\", te.Code, flat(te.Msg)))")], "pkg/ftp"),
 ("F5f", "a failed FEAT is fatal (a server without FEAT cannot log in)", [(P, "\t\tlines = nil\n\t} else if fcode != 211 {", "\t\treturn p.stepErr(ctx, \"FEAT\", err)\n\t} else if fcode != 211 {")], "pkg/ftp"),
 ("F6a", "MLSD type values are case-sensitive", [(M, "switch l := strings.ToLower(v); {", "switch l := v; {")], "pkg/ftp"),
 ("F6b", "MLSD sizes are parsed with base prefixes (010 is octal)", [(M, "u, err := strconv.ParseUint(v, 10, 64)", "u, err := strconv.ParseUint(v, 0, 64)")], "pkg/ftp"),
 ("F6c", "MLSD facts end at the first space (a space in a fact value breaks the entry)", [(M, "\t\ti := strings.Index(line, \"; \")\n\t\tif i < 0 {", "\t\ti := strings.Index(line, \" \") - 1\n\t\tif i < 0 {")], "pkg/ftp"),
 ("F6d", "cdir/pdir entries are listed as children", [(F, "if !visibleName(en.name) || en.kind == kindSelf || en.kind == kindParent {", "if !visibleName(en.name) {")], "pkg/ftp"),
 ("F6e", "a listing line that cannot be parsed is silently dropped", [(F, "\t\t\te, perr := parseMLEntry(line)\n\t\t\tif perr == nil {\n\t\t\t\tout = append(out, e)\n\t\t\t}\n\t\t\treturn perr", "\t\t\te, perr := parseMLEntry(line)\n\t\t\tif perr == nil {\n\t\t\t\tout = append(out, e)\n\t\t\t}\n\t\t\treturn nil")], "pkg/ftp"),
 ("F6f", "symlinks are not recognised", [(M, "\tcase strings.HasPrefix(l, \"os.unix=slink\"), strings.HasPrefix(l, \"os.unix=symlink\"):\n\t\treturn kindLink\n", "")], "pkg/ftp"),
 ("F6g", "MLST accepts several entries / no entry silently", [(F, "\t\t\tif len(ents) != 1 {\n\t\t\t\treturn fmt.Errorf(\"%w: MLST returned %d entries for one path\", ErrListingIncomplete, len(ents))\n\t\t\t}", "\t\t\tif len(ents) == 0 {\n\t\t\t\treturn perr\n\t\t\t}")], "pkg/ftp"),
 ("F7a", "every 550 is reported as non-existence", [(M, "\treturn k550Unknown\n}", "\treturn k550Absent\n}")], "pkg/ftp"),
 ("F7b", "permission text is not recognised (denied reads as absent)", [(M, "\tfor _, s := range []string{\"permission\", \"denied\", \"not permitted\", \"forbidden\", \"not allowed\", \"privilege\", \"read-only\", \"read only\"} {", "\tfor _, s := range []string{} {")], "pkg/ftp"),
 ("F7c", "an ambiguous 550 is not settled by the parent listing", [(F, "classify550(te.Msg) == k550Unknown {", "classify550(te.Msg) == k550Unknown && false {")], "pkg/ftp"),
 ("F8a", "degraded stat of a top-level entry lists with no argument", [(F, "\tentries, err := c.listEntries(ctx, p, path.Dir(remote), degraded)", "\tentries, err := c.listEntries(ctx, p, strings.TrimSuffix(path.Dir(remote), \"/\"), degraded)")], "pkg/ftp"),
 ("F8b", "degraded stat of the root itself is a parent listing", [(F, "\t\tif logical == \"/\" { // the root itself: there is no parent to list", "\t\tif false { // the root itself: there is no parent to list")], "pkg/ftp"),
 ("F10a", "Credential's String has a pointer receiver again", [(C, "func (c Credential) String() string { return \"Credential(redacted)\" }", "func (c *Credential) String() string { return \"Credential(redacted)\" }"), (C, "func (c Credential) GoString() string { return c.String() }", "func (c *Credential) GoString() string { return c.String() }")], "pkg/ftp"),
 ("F11a", "a timed-out Disconnect leaves the connection to whoever comes next", [(F, "\tif stale != nil {\n\t\tstale.quit()\n\t}\n\t<-c.sem", "\t_ = stale\n\t<-c.sem")], "pkg/ftp"),
 ("F11b", "a re-dial does not close the connection it replaces", [(F, "\tif old != nil {\n\t\told.quit() // a connection left by a timed-out Disconnect must not leak\n\t}", "\t_ = old")], "pkg/ftp"),
 ("F12", "a 0xFF byte in a path is sent raw", [(P, "func escapeIAC(s string) string { return strings.ReplaceAll(s, \"\\xff\", \"\\xff\\xff\") }", "func escapeIAC(s string) string { return s }")], "pkg/ftp"),
 ("F13a", "the control reply size is not bounded", [(P, "\t\tif total > p.o.maxReply {", "\t\tif false {")], "pkg/ftp"),
 ("F13b", "the listing size is not bounded", [(P, "\t\t\tif n > maxEntries {", "\t\t\tif false {")], "pkg/ftp"),
 ("F14", "Pin/DiscoverCert port 0 is dialled as port 0", [(T, "\tif port == 0 {\n\t\tport = DefaultPort\n\t}\n\t// An empty trust pool", "\t// An empty trust pool")], "pkg/ftp"),
 ("F15", "a second server name for the same host is accepted by Pin", [(T, "if e.ServerName != \"\" && e.ServerName != name &&", "if false && e.ServerName != name &&")], "pkg/ftp"),
 ("F2f", "a stream failure is waited for instead of dropping the connection", [(F, "\t\tif s.rerr != nil || s.dc.aborted.Load() || s.ctx.Err() != nil {\n\t\t\ts.p.markBroken()\n\t\t\t_ = s.dc.Close()\n\t\t} else if", "\t\tif false {\n\t\t} else if")], "pkg/ftp"),
 # ---- the factory wiring (F16) ----
 ("W1", "factory: the process-wide pin store is not passed", [(FA, "\t\tPinStore:          ftp.DefaultPinStore,\n", "")], "pkg/factory"),
 ("W2", "factory: credential_ref is not passed", [(FA, "CredentialRef:     GetStringSetting(config.Settings, \"credential_ref\", \"\"),", "CredentialRef:     \"\",")], "pkg/factory"),
 ("W3", "factory: tls_mode is not passed", [(FA, "TLSMode:           GetStringSetting(config.Settings, \"tls_mode\", \"\"),", "TLSMode:           \"\",")], "pkg/factory"),
 ("W4", "factory: trusted_lan is always true", [(FA, "TrustedLAN:        trusted,", "TrustedLAN:        trusted || true,")], "pkg/factory"),
 ("W5", "factory: allow_degraded_list is not passed", [(FA, "AllowDegradedList: degraded,", "AllowDegradedList: degraded && false,")], "pkg/factory"),
 ("W6", "factory: an inline password is accepted again", [(FA, "[]string{\"password\", \"passphrase\", \"private_key\", \"privatekey\", \"key\", \"secret\", \"token\"}", "[]string{\"passphrase\"}")], "pkg/factory"),
 ("W7", "factory: the port is not validated", [(FA, "if !isNum || n < 1 || n > 65535 {", "if !isNum {")], "pkg/factory"),
 ("W8", "factory: the scan client is replaced by the raw client", [(FA, "\treturn ftp.NewScanClient(&ftp.Config{", "\treturn rawFTP(&ftp.Config{"), (FA, "// boolSetting reads a boolean setting", "func rawFTP(c *ftp.Config, _ ftp.ScanOptions) (client.Client, error) { return ftp.NewFTPClient(c), nil }\n\n// boolSetting reads a boolean setting")], "pkg/factory"),
 ("W9", "factory: a quoted boolean is accepted", [(FA, "\tb, isBool := v.(bool)\n\tif !isBool {\n\t\treturn false, fmt.Errorf(\"ftp: setting %q must be a boolean\", key)\n\t}\n\treturn b, nil", "\tb, _ := v.(bool)\n\treturn b, nil")], "pkg/factory"),
]


def gen(work):
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
                sys.exit(1)
            open(p, "w").write(s.replace(old, new))
        open(os.path.join(work, "mut", mid, "target"), "w").write(target + "\n")
    print("generated", len(MUT))


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
