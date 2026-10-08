#!/usr/bin/env bash
# test_nfs_attempt.sh - WF17 fix round 5, CLASS E (RECORDS) closure test for scripts/test-infra/nfs_attempt.sh (TI-E1, TI-E2). Until now NO test drove nfs_attempt.sh end to end (only test_consumers.sh named it
# statically), which is how `nfs_attempt.sh` recorded an NFS `pass` from a STALE tracked `nfs-attempt-observed.json` without running a single round trip.
# Real rootless podman: the real nfs stack (unfs3 + rpcbind, image built by nfs_build.sh), the real libnfs round trips from IMG-INFRA-CLIENT recorded through evrec, the real terminal-state check through
# `TIC tooling unit`. Fault injection at ONE edge (11.4.85): a copy of the script directory whose roundtrip.sh fails BEFORE it runs the protocol round trip (for chosen iterations), delegating every other
# call to the real script. Oracle strategy (11.4.245): SPECIFIED (the record set is empty at the start; the observed file is written only from this run) and DERIVED (the ledger, written by evrec, is the independent
# witness of what really ran: every cited record must resolve in it).
# Rows: R1 a stale observed file claiming three ok round trips is planted, every real round trip fails -> NOT a pass, the stale file is gone, no nfs-attempt.json | R2 a clean run with the stale file planted ->
# state pass, three ledger records whose sequence numbers equal the cited ones | R3 iteration 2 fails -> the failed iteration is `stored:false` with `record:null`, the later iteration cites the sequence evrec
# REPORTED (seq 2), not the fabricated #seq3.
# Paired mutations: the stale file kept and written only when absent; the pointer fabricated again (#seq$i); identity mutant (must SURVIVE).
# Usage:  test_nfs_attempt.sh   (NFSA_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR, NFSA_ONLY (comma list of rows), NFSA_EV
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=NFSA; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/scripts/test-infra/down.sh"
for f in nfs_attempt.sh nfs_terminal_state.sh roundtrip.sh up.sh down.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
if [ -z "$(podman images -q localhost/catalogizer-infra-nfs 2>/dev/null)" ]; then blocked "the nfs server image is not built on this host (scripts/test-infra/nfs_build.sh)"; ti_summary; exit 0; fi
want() { [ -z "${NFSA_ONLY:-}" ] || [[ ",$NFSA_ONLY," == *",$1,"* ]]; }
# the SUT with the fault stub: every script is the SUT's own; only roundtrip.sh is replaced by a stub that fails the chosen iterations and delegates the others to the SUT's real roundtrip.sh
mkstub() { local d="$TI_REPO/.audit/scratch/nfsa-stub-$$"; rm -rf -- "${d:?}"; mkdir -p "$d"; cp "$TI_REPO/$SD"/*.sh "$TI_REPO/$SD"/*.py "$d/"; cp -r "$TI_REPO/$SD/client" "$TI_REPO/$SD/nfs" "$d/"
  cat >"$d/roundtrip.sh" <<EOF
#!/usr/bin/env bash
it=""; a=("\$@"); for ((i=0;i<\${#a[@]};i++)); do [ "\${a[\$i]}" = --iteration ] && it=\${a[\$((i+1))]}; done
if [ "\${FAIL_ITERS:-}" = all ] || [[ ",\${FAIL_ITERS:-}," == *",\$it,"* ]]; then echo "injected fault: the round trip did not run (iteration \$it)" >&2; exit 1; fi
exec bash "$TI_REPO/$SD/roundtrip.sh" "\$@"
EOF
  TI_FOREIGN_DIRS+=("$d"); STUB="$d"; }
mkstub
CLIENT="$TI_REPO/.audit/scratch/nfsa-client-$$.json"; echo '{"schema":"nfs-client/1","verdict":"VERIFIED"}' >"$CLIENT"; TI_FOREIGN_DIRS+=("$CLIENT")
plant_stale() { # a stale observed record claiming three ok round trips (the shape the tracked wp10 file has), plus its .rts scratch file
  mkdir -p "$1"; printf '%s\n' '{"round_trips":[{"iteration":1,"ok":true,"record":"ledger-nfs/ledger.jsonl#seq1"},{"iteration":2,"ok":true,"record":"ledger-nfs/ledger.jsonl#seq2"},{"iteration":3,"ok":true,"record":"ledger-nfs/ledger.jsonl#seq3"}]}' >"$1/nfs-attempt-observed.json"; echo '[]' >"$1/.rts"; }
run_attempt() { # run_attempt <sut dir> <ev dir> <build id> [VAR=val...]: the real nfs_attempt.sh; sets ARC, AOUT
  local sut=$1 ev=$2 id=$3; shift 3; TI_IDS+=("$id"); TI_FOREIGN_DIRS+=("$ev")
  AOUT=$(env "$@" bash "$sut/nfs_attempt.sh" --ev-dir "$ev" --client-json "$CLIENT" --build-id "$id" 2>&1); ARC=$?
}

# ---------------- R1: stale observed file + failing round trips -> never a pass ----------------
if want 1; then
EV1="$TI_REPO/.audit/scratch/nfsa-ev1-$$"; plant_stale "$EV1"
[ -s "$EV1/nfs-attempt-observed.json" ] && ok "R1 control: the stale observed file exists before the run" || bad "R1 control: the stale file was not planted"
run_attempt "$STUB" "$EV1" "$(ti_new_id)" FAIL_ITERS=all
check "R1: nfs_attempt exits 1 (no terminal state can be chosen from three failed round trips)" "$ARC" 1
check "R1: the observed file was REWRITTEN from this run (three ok:false entries), the stale pass is gone" "$(jq -r '[.round_trips[].ok] | map(tostring) | join(",")' "$EV1/nfs-attempt-observed.json" 2>/dev/null)" "false,false,false"
check "R1: no nfs-attempt.json (no terminal state) was recorded" "$([ -e "$EV1/nfs-attempt.json" ] && echo recorded || echo none)" none
check "R1: the failed round trips stored no ledger record, and cite none" "$(jq -r '[.round_trips[] | (.stored|tostring) + ":" + (.record|tostring)] | join(",")' "$EV1/nfs-attempt-observed.json" 2>/dev/null)" "false:null,false:null,false:null"
case "$AOUT" in *"no terminal state recorded"*) ok "R1: the message says no terminal state was recorded";; *) bad "R1: unexpected output: $(printf '%s' "$AOUT" | tail -2 | tr '\n' ' ' | cut -c1-200)";; esac
fi

# ---------------- R2: clean run with the stale file planted: pass with three resolvable ledger records ----------------
if want 2; then
EV2="$TI_REPO/.audit/scratch/nfsa-ev2-$$"; plant_stale "$EV2"
run_attempt "$TI_REPO/$SD" "$EV2" "$(ti_new_id)"
check "R2: nfs_attempt exits 0 on a clean run" "$ARC" 0
[ "$ARC" = 0 ] || echo "  said: $(printf '%s' "$AOUT" | tail -3 | tr '\n' ' ' | cut -c1-300)"
check "R2: the terminal state is pass" "$(jq -r '.state' "$EV2/nfs-attempt.json" 2>/dev/null)" pass
check "R2: the ledger holds exactly three records, all GREEN runtime passes" "$(jq -s '[.[] | select(.verdict=="pass" and .polarity=="GREEN" and .evidence_class=="runtime")] | length' "$EV2/ledger-nfs/ledger.jsonl" 2>/dev/null)" 3
bad_refs=0; for i in 0 1 2; do ref=$(jq -r ".round_trips[$i].record" "$EV2/nfs-attempt-observed.json"); n=${ref##*#seq}
  jq -e -s --argjson q "$n" 'any(.[]; .seq == $q and .verdict == "pass")' "$EV2/ledger-nfs/ledger.jsonl" >/dev/null 2>&1 || bad_refs=$((bad_refs+1)); done
check "R2: every cited record resolves to a passing ledger entry of that sequence" "$bad_refs" 0
check "R2: the cited sequences are the ledger's own (1,2,3)" "$(jq -r '[.round_trips[].record | sub(".*#seq"; "")] | join(",")' "$EV2/nfs-attempt-observed.json")" "1,2,3"
check "R2: the observed file carries the new schema (this run wrote it)" "$(jq -r .schema "$EV2/nfs-attempt-observed.json")" "nfs-attempt-observed/1"
check "R2: the nfs project was torn down (no container, no state)" "$(podman ps -a -q --filter "label=catalogizer.test_project=$(ti_project "${TI_IDS[-1]}")" | wc -l)$([ -e "$(ti_state "${TI_IDS[-1]}")" ] && echo left)" 0
fi

# ---------------- R3: iteration 2 fails: no fabricated pointer ----------------
if want 3; then
EV3="$TI_REPO/.audit/scratch/nfsa-ev3-$$"
run_attempt "$STUB" "$EV3" "$(ti_new_id)" FAIL_ITERS=2
check "R3: nfs_attempt exits 1 (a run with a failed round trip is inconclusive)" "$ARC" 1
check "R3: iteration 1 cites its real ledger sequence, iteration 2 is stored:false/record:null, iteration 3 cites the sequence evrec REPORTED (2), not a fabricated #seq3" "$(jq -r '[.round_trips[] | (.iteration|tostring) + "=" + (.stored|tostring) + ":" + (.record|tostring)] | join(" ")' "$EV3/nfs-attempt-observed.json" 2>/dev/null)" "1=true:ledger-nfs/ledger.jsonl#seq1 2=false:null 3=true:ledger-nfs/ledger.jsonl#seq2"
check "R3: the ledger holds two records (1 and 2): the cited sequences exist" "$(jq -s '[.[].seq] | join(",")' "$EV3/ledger-nfs/ledger.jsonl" 2>/dev/null | tr -d '"')" "1,2"
fi

# ---------------- paired mutations ----------------
if [ "${NFSA_NO_MUTATIONS:-0}" != 1 ] && [ "${NFSA_TEST_MUTANT:-0}" != 1 ]; then
  mut_batch_begin "${NFSA_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"
  NFSA_ONLY=1 mut stale_observed_kept_and_written_only_if_absent nfs_attempt.sh 'rm -rf -- "${LEDGER:?}" "$ATT" "$EVD/.rts"' 'rm -rf -- "${LEDGER:?}" "$EVD/.rts"' "    jq -n --argjson r \"\$rts\" '{schema:\"nfs-attempt-observed/1\", round_trips:\$r}' >\"\$ATT\" || ti_die \"cannot write \$ATT\" 1" "    [ -s \"\$ATT\" ] || jq -n --argjson r \"\$rts\" '{schema:\"nfs-attempt-observed/1\", round_trips:\$r}' >\"\$ATT\" || ti_die \"cannot write \$ATT\" 1"
  NFSA_ONLY=3 mut pointer_fabricated_again nfs_attempt.sh '--arg rec "ledger-nfs/ledger.jsonl#seq$seq"' '--arg rec "ledger-nfs/ledger.jsonl#seq$i"'
  NFSA_ONLY=1 mut_id identity_noop nfs_attempt.sh 'mkdir -p "$EVD"; LEDGER="$EVD/ledger-nfs"' 'mkdir -p "$EVD"; : ; LEDGER="$EVD/ledger-nfs"'
  mut_batch_end "${NFSA_EV:+$NFSA_EV/nfs-attempt-mutations.txt}"
fi
ti_summary
