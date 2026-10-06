#!/usr/bin/env bash
# mutate_dispatch_events.sh - T005a paired mutations for test_dispatch_events.sh. Each mutant is ONE textual change to a
# copy of scripts/build (the shipped files are never touched); the test must FAIL on it. Prints the unified diff, the FAIL
# lines and the RESULT line per mutant, then KILLED or SURVIVED. Usage: mutate_dispatch_events.sh list | <name>... | all
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); build=$(cd "$here/.." && pwd)
# name<TAB>file<TAB>old text (must occur exactly once)<TAB>new text
MUTANTS=$(cat <<'M'
m_noconsumedset	event_core.sh	    atomic_write "$d" "$d/consumed/$seq" "$dig $(now)" || refuse_l state_write_failed "consumed/$seq"<NL>    flock -u 9; echo "consumed seq=$seq kind=$kind"; return 0	    flock -u 9; echo "consumed seq=$seq kind=$kind"; return 0
m_acceptunsigned	lib/bev_crypto.py	    if not hmac.compare_digest(got, mac(key(statedir, b, exp_run, checkout), ev)):	    if False:
m_noseqcheck	event_core.sh	  if [ "$seq" -le "$last" ]; then refuse_l illegal_transition "seq=$seq not above last consumed $last"; fi	  :
m_filerename	event_core.sh	  if mv -T "$tmpd" "$d/terminal" 2>/dev/null; then	  if { rm -rf "$d/terminal"; mv -T "$tmpd" "$d/terminal"; } 2>/dev/null; then
m_keyinjournal	event_core.sh	  journal "$d" "$ev"	  journal "$d" "$ev $(python3 "$CRYPTO" derive "$sd" "$b" "$r" "$CHECKOUT")"
m_keyinstdout	event_core.sh	    flock -u 9; echo "consumed seq=$seq kind=$kind"; return 0	    flock -u 9; echo "consumed seq=$seq kind=$kind key=$(python3 "$CRYPTO" derive "$sd" "$b" "$r" "$CHECKOUT")"; return 0
m_stateinside	event_core.sh	  case "$sd/" in "$checkout"/*) refuse secret_dir_inside_checkout "$sd";; esac	  :
m_noreplay	lib/bev_crypto.py	        print("other_run")	        print("ok"); print(json.dumps(ev, sort_keys=True, separators=(",", ":"), ensure_ascii=False)); print(hashlib.sha256(canon(ev)).hexdigest())
m_effectnotonce	event_core.sh	      ln "$d/tmp/e.$$" "$d/effects/$key" 2>/dev/null	      cp -f "$d/tmp/e.$$" "$d/effects/$key"; printf '%s %s\n' "$key" "$(now)" >> "$d/effects.log"
m_rerundone	event_core.sh	  [ "$(cbstate "$d")" = done ] && return 0<SEP>  if [ "$(cbstate "$d")" = done ]; then flock -u 8; return 0; fi	  :<SEP>  :
m_nolock	event_core.sh	  flock -w 60 9 || refuse lock_unavailable "$b"	  true
m_locknotchecked	event_core.sh	  { exec 9>>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || refuse lock_unavailable "$b"	  { exec 9>>"$d/.lock"; } 2>/dev/null
m_unlockbeforeclaim	event_core.sh	  # ---- terminal claim, still under the lock: the rename of a prepared directory is the single serialisation point ----	  flock -u 9
m_dupbyseq	event_core.sh	    if [ "$old" = "$dig" ]; then flock -u 9;	    if true; then flock -u 9;
m_nostagecheck	event_core.sh	    if [ "$po" -lt "$lastprog" ] || [ "$st" -lt "$laststage" ]; then	    if [ "$po" -lt "$lastprog" ]; then
m_hbfields	lib/bev_crypto.py	        if not all(f in ev and _isint(ev[f], 0) for f in HB):	        if False:
m_completedimage	lib/bev_crypto.py	and _str(ev.get("image_digest"))	and True
m_completedlog	lib/bev_crypto.py	and _hex64(ev.get("remote_log_sha256")))	and True)
m_completedmanifest	lib/bev_crypto.py	_hex64(ev.get("artifact_manifest_sha256"))	True
m_exitclassenum	lib/bev_crypto.py	        if not (ev.get("exit_class") in EXIT_CLASSES and	        if not (ev.get("exit_class") and
m_extrafield	lib/bev_crypto.py	    if set(ev) - allowed:	    if False:
m_floataccepted	lib/bev_crypto.py	    return type(v) is int and lo <= v <= hi	    return isinstance(v, (int, float)) and lo <= v <= hi
m_nobound	lib/bev_crypto.py	    return type(v) is int and lo <= v <= hi	    return type(v) is int and lo <= v
m_dupkeys	lib/bev_crypto.py	        if k in d:	        if False:
m_buildidlax	lib/bev_crypto.py	BUILD_ID_RE = re.compile(r"^[A-Za-z0-9_-]{1,128}$")	BUILD_ID_RE = re.compile(r"^[A-Za-z0-9._-]{1,128}$")
m_lastbyls	event_core.sh	    n=${f##*/}; [[ $n =~ ^[1-9][0-9]{0,15}$ ]] || continue	    n=${f##*/}
m_reread	event_core.sh	  isint "$seq" || refuse event_malformed "seq"	  isint "$seq" || refuse event_malformed "seq"; ec=$(jq -r '.exit_class // "-"' "$ef")
m_evhook	event_core.sh	  # ---- terminal claim, still under the lock: the rename of a prepared directory is the single serialisation point ----	  if [ -n "${EC_TEST_HOLD_BEFORE_CLAIM:-}" ]; then : > "$EC_TEST_HOLD_BEFORE_CLAIM.reached"; while [ ! -e "$EC_TEST_HOLD_BEFORE_CLAIM.go" ]; do sleep 0.05; done; fi
m_secretmode	lib/bev_crypto.py	    if st.st_mode & 0o077:	    if False:
m_secretlink	lib/bev_crypto.py	    if stat.S_ISLNK(st.st_mode) or not stat.S_ISREG(st.st_mode):	    if False:
m_statelink	lib/bev_crypto.py	    if os.path.islink(statedir):	    if False:
m_secretmeta	lib/bev_crypto.py	    if not hmac.compare_digest(want, hashlib.sha256(raw).hexdigest()):	    if False:
m_secretnometa	lib/bev_crypto.py	    if want is None:	    if False:
m_secretlocation	lib/bev_crypto.py	        if real == co or real.startswith(co + os.sep):	        if False:
m_nodirfsync	event_core.sh	fsync_dir() { python3 -c "$FSYNC_PY" dir "$1" 2>/dev/null; }	fsync_dir() { :; }
m_logalways	event_core.sh	      if ! awk -v k="$key" '$1 == k { f = 1 } END { exit !f }' "$d/effects.log" 2>/dev/null; then	      if true; then
m_canonunsorted	lib/bev_crypto.py	    return json.dumps(e, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")	    return json.dumps(e, sort_keys=False, separators=(",", ":"), ensure_ascii=True).encode("utf-8")
m_hkdfsalt	lib/bev_crypto.py	    prk = hmac.new(salt or b"\x00" * 32, secret, hashlib.sha256).digest()	    prk = hmac.new(salt or b"\x01" * 32, secret, hashlib.sha256).digest()
m_claimfail_aslost	event_core.sh	  if terminal_ok "$d"; then return 1; fi	  return 1
m_claimfail_win	event_core.sh	  if terminal_ok "$d"; then return 1; fi	  if terminal_ok "$d"; then return 1; fi; return 0
m_cancel_damaged_superseded	event_core.sh	then if terminal_ok "$d"; then crc=1; else crc=2; fi; else	then crc=1; else
m_consume_claimfail_superseded	event_core.sh	claim_terminal "$d" completed "$seq" "$ec" "$dig"; crc=$?	claim_terminal "$d" completed "$seq" "$ec" "$dig"; crc=$?; [ $crc -eq 2 ] && crc=1
m_claim_unvalidated	event_core.sh	    && [ -s "$tmpd/state.json" ] && jq -e '.kind' "$tmpd/state.json" >/dev/null \	    && true \
m_atomic_syncunchecked	event_core.sh	[ -s "$t" ] && fsync_file "$t" && mv -fT "$t" "$f"	[ -s "$t" ] && { fsync_file "$t"; true; } && mv -fT "$t" "$f"
m_progress_unchecked	event_core.sh	|| refuse_l state_write_failed "progress.json"	|| true
m_consumedmark_unchecked	event_core.sh	if [ "$kind" != completed ]; then<NL>    atomic_write "$d" "$d/consumed/$seq" "$dig $(now)" || refuse_l state_write_failed "consumed/$seq"	if [ "$kind" != completed ]; then<NL>    atomic_write "$d" "$d/consumed/$seq" "$dig $(now)" || true
m_journal_unchecked	event_core.sh	journal "$d" "$ev" || refuse_l journal_failed "event"	journal "$d" "$ev" || true
m_journal_latechecked	event_core.sh	|| refuse_l journal_failed "late_ignored"	|| true
m_journal_multiwrite	event_core.sh	    ok = os.write(fd, d) == len(d)	    n = 0<NL>    for i in range(0, len(d), 4096): n += os.write(fd, d[i:i+4096])<NL>    ok = n == len(d)
m_terminal_journal_unchecked	event_core.sh	 '{event:"terminal_claimed",kind:$k,seq:$s}')" || return 3	 '{event:"terminal_claimed",kind:$k,seq:$s}')" || true
m_cancel_nolock	event_core.sh	  { exec 9>>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || refuse lock_unavailable "$2"<NL>  flock -w 60 9 || refuse lock_unavailable "$2"	  :
m_lockok_off	event_core.sh	  [ "$(stat -L -c %i "/proc/$$/fd/$1" 2>/dev/null)" = "$(stat -c %i "$2" 2>/dev/null)" ]; }	  true; }
m_lock_truncates	event_core.sh	  { exec 9>>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || refuse lock_unavailable "$b"	  { exec 9>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || refuse lock_unavailable "$b"
m_cblock_unchecked	event_core.sh	{ exec 8>>"$d/.cblock"; } 2>/dev/null && lock_ok 8 "$d/.cblock" ||	{ exec 8>>"$d/.cblock"; } 2>/dev/null ||
m_cb_norecheck	event_core.sh	  if [ "$(cbstate "$d")" = done ]; then flock -u 8; return 0; fi	  :
m_ln_unchecked	event_core.sh	if [ $lrc -ne 0 ] && [ ! -e "$d/effects/$key" ]; then cb_fail "$d" effect_not_applied; return 1; fi	:
m_ln_eexist_fails	event_core.sh	if [ $lrc -ne 0 ] && [ ! -e "$d/effects/$key" ]; then	if [ $lrc -ne 0 ]; then
m_failed_is_done	event_core.sh	  set_cbstate "$1" failed "$2"; flock -u 8 2>/dev/null;	  set_cbstate "$1" done "$2"; flock -u 8 2>/dev/null;
m_resume_rc	event_core.sh	  run_callback "$d" || rc=21	  run_callback "$d" || true
m_effectkey_unvalidated	event_core.sh	    [[ $key =~ ^[A-Za-z0-9_-]{1,128}$ ]] || { cb_fail "$d" effect_key_invalid; return 1; }	    :
m_effectkey_regexgrep	event_core.sh	if ! awk -v k="$key" '$1 == k { f = 1 } END { exit !f }' "$d/effects.log" 2>/dev/null; then	if ! grep -q "^$key" "$d/effects.log" 2>/dev/null; then
m_secret_ownckout	event_core.sh	  case "$sd/" in "$CHECKOUT"/*) refuse secret_dir_inside_checkout "$sd";; esac   # this script's OWN checkout, whatever the argument says	  :
m_secret_create_unverified	event_core.sh	    openssl rand -hex 32 > "$t" || exit 1<SEP>    [[ "$(cat "$t")" =~ ^[0-9a-f]{64}$ ]] || exit 1<SEP>refuse secret_create_failed "read-back"	    openssl rand -hex 32 > "$t"<SEP>    :<SEP>true
m_secret_subshell_unchecked	event_core.sh	    fsync_dir "$sd" || exit 1 ) || { rm -f "$sd/build_hmac.key" "$sd/build_hmac.key.meta" 2>/dev/null; refuse secret_create_failed "$sd"; }<SEP>refuse secret_create_failed "read-back"	    fsync_dir "$sd" || exit 1 ) || true<SEP>true
m_keylen	lib/bev_crypto.py	    if len(txt) != KEY_HEX_LEN:	    if False:
m_keyhex	lib/bev_crypto.py	    if not re.fullmatch(r"[0-9a-f]*", txt):	    if False:
m_runid_bind	lib/bev_crypto.py	    if ev["run_id"] != exp_run:	    if False:
m_variant_bind	lib/bev_crypto.py	    if ev["variant"] != exp_variant:	    if False:
m_firstseq	event_core.sh	  if [ "$last" = 0 ] && [ "$seq" != 1 ]; then refuse_l illegal_transition "first event has seq $seq, not 1"; fi	  :
m_strcap	lib/bev_crypto.py	def _str(v, lo=1, hi=STR_MAX):	def _str(v, lo=1, hi=10 ** 9):
m_capboundary	lib/bev_crypto.py	    if len(raw) > MAX_EVENT_BYTES:	    if len(raw) >= MAX_EVENT_BYTES:
m_tmpname	event_core.sh	tmpd="$d/terminal.tmp-$$-$(starttime)"	tmpd="$d/tmp/terminal.$$-$(starttime)"
X1_secondaccepted	event_core.sh	  if [ "$last" != 0 ] && [ "$kind" = accepted ]; then refuse_l illegal_transition "second accepted"; fi	  :
X2_noterminaldup	event_core.sh	  if [ "$kind" = completed ] && [ -f "$d/terminal/state.json" ]	  if false && [ -f "$d/terminal/state.json" ]
X3_novariant	lib/bev_crypto.py	    if ev["variant"] not in VARIANTS:	    if False:
X4_noschema	lib/bev_crypto.py	    if ev["schema"] != "build-event/1":	    if False:
X5_nosizecap	lib/bev_crypto.py	    if len(raw) > MAX_EVENT_BYTES:	    if False:
X6_progcorrupt	event_core.sh	      isint "$lastprog" && isint "$laststage" || refuse_l state_corrupt "progress.json"	      :
X7_nostrfields	lib/bev_crypto.py	    if not _str(ev["run_id"], 1, 128) or not _str(ev["host"]) or not _str(ev["sent_at"]):	    if False:
R3_isint_relaxed	event_core.sh	isint() { [[ ${1:-} =~ ^(0|[1-9][0-9]{0,15})$ ]]; }	isint() { [[ ${1:-} =~ ^[0-9]+$ ]]; }
R3_journal_shortwrite_ok	event_core.sh	    ok = os.write(fd, d) == len(d)	    ok = os.write(fd, d) >= 0
R3_journal_norollback	event_core.sh	    try: os.ftruncate(fd, size)<NL>    except OSError: pass<NL>    sys.exit(1)	    sys.exit(1)
R3_journal_nofsync	event_core.sh	        os.fsync(fd)<NL>except OSError:	        pass<NL>except OSError:
R3_journal_norepair	event_core.sh	    if os.pread(rfd, 1, size - 1) != b"\n":	    if False:
R3_journal_follows_symlink	event_core.sh	os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o644)	os.O_APPEND | os.O_CREAT, 0o644)
R3_journal_follows_symlink_both	event_core.sh	os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o644)<SEP>    rfd = os.open(p, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)	os.O_APPEND | os.O_CREAT, 0o644)<SEP>    rfd = os.open(p, os.O_RDONLY | os.O_NONBLOCK)
R3_journal_dirsync_ignored	event_core.sh	  fsync_dir "$1"; }	  fsync_dir "$1"; true; }
R3_atomic_dirsync_ignored	event_core.sh	  fsync_dir "$(dirname "$f")"; }	  fsync_dir "$(dirname "$f")"; true; }
R3_claim_dirsync_unchecked	event_core.sh	  fsync_dir "$tmpd" || { rm -rf "$tmpd"; return 2; }	  fsync_dir "$tmpd"
R3_fsync_file_noop	event_core.sh	    try: os.fsync(fd)<NL>    finally: os.close(fd)	    try: pass<NL>    finally: os.close(fd)
R3_fsync_errors_all_ignored	event_core.sh	    sys.exit(0 if d and e.errno in (errno.EINVAL, errno.ENOTSUP) else 1)	    sys.exit(0)
R3_fsync_dir_errors_ignored	event_core.sh	    sys.exit(0 if d and e.errno in (errno.EINVAL, errno.ENOTSUP) else 1)	    sys.exit(0 if d else 1)
R3_secret_keysync_unchecked	event_core.sh	    fsync_file "$t" || exit 1	    fsync_file "$t"
R3_secret_metasync_unchecked	event_core.sh	    fsync_file "$mt" || exit 1	    fsync_file "$mt"
R3_secret_dirsync_unchecked	event_core.sh	    fsync_dir "$sd" || exit 1 ) ||	    fsync_dir "$sd"; true ) ||
R3_secret_nonabs	event_core.sh	[[ $1 == /* ]] || refuse secret_dir_not_absolute "$1"	:
R3_cancel_noid	event_core.sh	  [[ $2 =~ ^[A-Za-z0-9_-]{1,128}$ ]] || refuse event_malformed "build_id"<NL>  [ -f "$d/submit.json" ]	  [ -f "$d/submit.json" ]
R3_resume_noid	event_core.sh	local d="$1/$2" rc=0; [[ $2 =~ ^[A-Za-z0-9_-]{1,128}$ ]] || refuse event_malformed "build_id"	local d="$1/$2" rc=0
R3_effectkey_nocap	event_core.sh	    [[ $key =~ ^[A-Za-z0-9_-]{1,128}$ ]]	    [[ $key =~ ^[A-Za-z0-9_-]+$ ]]
R3_terminaldup_noseq	event_core.sh	     && [ "$(jq -r '.seq' "$d/terminal/state.json")" = "$seq" ]; then	     && true; then
R3_b_unchecked	event_core.sh	  [[ $b =~ ^[A-Za-z0-9_-]{1,128}$ ]] || refuse event_malformed "build_id"<NL>  d="$root/$b"	  d="$root/$b"
R3_cbjson_unchecked	event_core.sh	[ -s "$d/callback.json" ] && jq -e . "$d/callback.json" >/dev/null 2>&1 || { cb_fail "$d" effect_unreadable; return 1; }	:
R3_cbkey_unchecked	event_core.sh	2>/dev/null) || { cb_fail "$d" effect_key_missing; return 1; }<SEP>  [ -n "$key" ] || { cb_fail "$d" effect_key_missing; return 1; }	2>/dev/null) || true<SEP>  :
R3_cbkey_nonstring	event_core.sh	jq -er '.effect_key | select(type == "string")'	jq -er '.effect_key'
R3_terminal_damaged_ok	event_core.sh	    terminal_ok "$d" || refuse_l state_corrupt "terminal"	    :
R3_buildid_match	lib/bev_crypto.py	BUILD_ID_RE.fullmatch(ev["build_id"])	BUILD_ID_RE.match(ev["build_id"])
R3_manifest_match	lib/bev_crypto.py	_hex64(ev.get("artifact_manifest_sha256"))	HEX64.match(str(ev.get("artifact_manifest_sha256", "")))
R3_log_match	lib/bev_crypto.py	_hex64(ev.get("remote_log_sha256"))	HEX64.match(str(ev.get("remote_log_sha256", "")))
R3_derive_checkout_optional	lib/bev_crypto.py	print(key(a[1], a[2], a[3], a[4]).hex())	print(key(a[1], a[2], a[3], a[4] if len(a) > 4 else None).hex())
R4_effects_fsync_unchecked	event_core.sh	fsync_dir "$d/effects" || { cb_fail "$d" effect_not_durable; return 1; }	fsync_dir "$d/effects"
R4_effects_dir_fsync_unchecked	event_core.sh	fsync_dir "$d" || { cb_fail "$d" effects_dir_not_durable; return 1; }	fsync_dir "$d"
W1_peak_rss_off	lib/bev_crypto.py	    if "peak_rss_bytes" in ev and not _isint(ev["peak_rss_bytes"], 0):	    if False:
W2_hb_fields_partial	lib/bev_crypto.py	        if not all(f in ev and _isint(ev[f], 0) for f in HB):	        if not all(f in ev and _isint(ev[f], 0) for f in ("progress_offset", "stage")):
W3_cancel_unknown_build	event_core.sh	  [ -f "$d/submit.json" ] || refuse event_unknown_build "$2"	  :
W4_resume_no_terminal	event_core.sh	  [ -f "$d/terminal/state.json" ] || refuse no_terminal "$2"	  :
W5_image_digest_empty_ok	lib/bev_crypto.py	_str(ev.get("image_digest"))	_str(ev.get("image_digest"), 0)
R4_sha_stringified	lib/bev_crypto.py	type(v) is str and HEX64.fullmatch(v) is not None	HEX64.fullmatch(str(v)) is not None
R4_atomic_mv_notT	event_core.sh	mv -fT "$t" "$f"	mv -f "$t" "$f"
R4_progress_isfile	event_core.sh	if [ -e "$d/progress.json" ] || [ -L "$d/progress.json" ]; then	if [ -f "$d/progress.json" ]; then
R4_dup_noredo	event_core.sh	[ "$kind" = completed ] && cb_redo "$d"; return 0; fi	return 0; fi
R4_dup_terminal_noredo	event_core.sh	dropped"; cb_redo "$d"; return 0; fi	dropped"; return 0; fi
R4_redo_retries_failed	event_core.sh	claimed|running) run_callback	claimed|running|failed) run_callback
R4_safe_dupkey_off	lib/bev_crypto.py	"duplicate key " + _safe(k)	"duplicate key " + k
R4_safe_field_off	lib/bev_crypto.py	"unexpected field " + _safe(sorted(set(ev) - allowed)[0])	"unexpected field " + sorted(set(ev) - allowed)[0]
R4_key_readerr_uncaught	lib/bev_crypto.py	raise SecretError("key_unreadable")	raise
m_r5_n1_fsync_only_on_create	event_core.sh	effects_dir_unavailable; return 1; }; fi<NL>    fsync_dir "$d" || { cb_fail "$d" effects_dir_not_durable; return 1; }	effects_dir_unavailable; return 1; }; fsync_dir "$d" || { cb_fail "$d" effects_dir_not_durable; return 1; }; fi
m_r5_n2_cat_progress	event_core.sh	pj=$(read_regular "$d/progress.json") || refuse_l state_corrupt "progress.json"	pj=$(cat "$d/progress.json" 2>/dev/null)
m_r5_n2_follow_symlink	event_core.sh	fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)<NL>st = os.fstat(fd)	fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NONBLOCK)<NL>st = os.fstat(fd)
m_r5_n3_redo_retries_failed	event_core.sh	  if [ "${2:-}" = redo ]; then case	  if false; then case
m_r6_ec1_effects_fsync_only_on_create	event_core.sh	    fsync_dir "$d/effects" || { cb_fail "$d" effect_not_durable; return 1; }	    [ -z "${lrc:-}" ] || fsync_dir "$d/effects" || { cb_fail "$d" effect_not_durable; return 1; }
m_r6_ec1_log_fsync_only_on_append	event_core.sh	        fsync_file "$d/effects.log" || { cb_fail "$d" effects_log_write_failed; return 1; }	        :
m_r6_ec2_effects_log_regular_off	event_core.sh	    regular_or_absent "$d/effects.log" || { cb_fail "$d" effects_log_not_regular; return 1; }	    :
m_r6_ec2_regular_or_absent_fstat_off	event_core.sh	sys.exit(0 if stat.S_ISREG(os.fstat(fd).st_mode) else 1)	sys.exit(0)
m_r6_ec2_regular_or_absent_enoent	event_core.sh	except OSError as e: sys.exit(0 if e.errno == errno.ENOENT else 1)	except OSError as e: sys.exit(1)
m_r6_ec2_consumed_cut	event_core.sh	    old=$(read_regular "$d/consumed/$seq") || refuse_l state_corrupt "consumed/$seq"   # round 6, EC-2: a FIFO or oversize file there is damage, never a blocked cut under the lock<NL>    old=${old%% *}	    old=$(cut -d' ' -f1 "$d/consumed/$seq" 2>/dev/null)
m_r6_ec2_journal_blocking_open	event_core.sh	os.O_WRONLY | os.O_NONBLOCK | os.O_APPEND	os.O_WRONLY | os.O_APPEND
m_r6_ec2_journal_no_regular_check	event_core.sh	if not stat.S_ISREG(st.st_mode): sys.exit(1)<NL>size = st.st_size	size = st.st_size
m_r6_ec2_cbstate_head	event_core.sh	cbstate() { read_regular "$1/terminal/callback.state" | head -c 64 | tr -d '\n'; }	cbstate() { head -c 64 "$1/terminal/callback.state" 2>/dev/null | tr -d '\n'; }
m_r6_ec3_redo_claimed_only	event_core.sh	claimed|running) ;; *) flock -u 8; return 0;;	claimed) ;; *) flock -u 8; return 0;;
m_r6_ec4_progress_unbounded	event_core.sh	 or st.st_size > 4096	
M
)
names() { printf '%s\n' "$MUTANTS" | cut -f1; }
run_one() {
  local name=$1 line file old new w
  line=$(printf '%s\n' "$MUTANTS" | grep -P "^$name\t") || { echo "no such mutant $name"; return 2; }
  file=$(cut -f2 <<<"$line"); old=$(cut -f3 <<<"$line"); new=$(cut -f4- <<<"$line")
  w=$(mktemp -d); mkdir -p "$w/co/scripts"; cp -r "$build" "$w/co/scripts/build"; rm -rf "$w/co/scripts/build/lib/__pycache__"; mb=$w/co/scripts/build
  # <NL> in a field stands for a newline; <SEP> separates several (old, new) pairs of ONE mutant (each old must occur exactly once)
  OLD=$old NEW=$new python3 - "$mb/$file" <<'PY' || { echo "== $name: PATCH FAILED (text not found exactly once)"; rm -rf "$w"; return 3; }
import os, sys
p = sys.argv[1]; s = open(p).read()
olds = os.environ["OLD"].split("<SEP>"); news = os.environ["NEW"].split("<SEP>")
if len(olds) != len(news): sys.exit(1)
for o, n in zip(olds, news):
    o = o.replace("<NL>", "\n"); n = n.replace("<NL>", "\n")
    if s.count(o) != 1: sys.exit(1)
    s = s.replace(o, n)
open(p, "w").write(s)
PY
  echo "== $name  ($file)"
  diff -u "$build/$file" "$mb/$file" | sed '1,2d'
  local out; out=$(EC_SCRIPT=$mb/event_core.sh timeout 600 bash "$here/test_dispatch_events.sh" 2>&1)
  printf '%s\n' "$out" | grep -E '^FAIL' | head -8; printf '%s\n' "$out" | grep -E '^RESULT'
  if printf '%s\n' "$out" | grep -q '^FAIL'; then echo "-> KILLED"; else echo "-> SURVIVED"; fi
  rm -rf "$w"
}
case "${1:-}" in
  list) names;;
  all) for n in $(names); do run_one "$n"; done;;
  "") echo "usage: $0 list|all|<name>..." >&2; exit 2;;
  *) for n in "$@"; do run_one "$n"; done;;
esac
