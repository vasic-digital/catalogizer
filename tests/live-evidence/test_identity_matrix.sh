#!/usr/bin/env bash
# Table-driven identity matrix for verify_live_evidence.sh (constitution 11.4.289(D), round-3 structural fix).
# usage: test_identity_matrix.sh <verifier> <register>.  For kind {process,socket,container} x variant {native,interpreter,renamed}
# and EVERY expected key (exe_sha256, image_digest, script_sha256) the cell must be LIVE only when the answer carries the same identity.
# Any cell that stays LIVE with a mismatching/absent identity, or that is refused while consistent, FAILS the suite.
set -u
V=$1; export LIVE_EVIDENCE_DOUBLES=$2
command -v jq >/dev/null || { echo "matrix: jq missing" >&2; exit 2; }
W=$(mktemp -d "${TMPDIR:-/tmp}/lev-mx.XXXXXX"); trap 'rm -rf "${W:?}"' EXIT
EX=1111111111111111111111111111111111111111111111111111111111111111   # exe
SC=2222222222222222222222222222222222222222222222222222222222222222   # script
IM=3333333333333333333333333333333333333333333333333333333333333333   # image digest
OT=4444444444444444444444444444444444444444444444444444444444444444   # a different identity
fail=0; n=0
cell() {  # cell <kind> <variant> -> record on stdout, or nothing when the combination does not exist
  local k=$1 v=$2 ans exp bound
  case "$k:$v" in
    container:native) ans='{"kind":"container","image_digest":"sha256:'$IM'","image":"reg.example/app:1","container_id":"c1"}'; exp='{"image_digest":"sha256:'$IM'"}'; bound=$IM;;
    container:*) return 1;;
    *:native) ans='{"kind":"'$k'","pid":7,"exe_path":"/opt/app/app","exe_sha256":"'$EX'","cmdline":"/opt/app/app --port 8080"}'; exp='{"exe_sha256":"'$EX'"}'; bound=$EX;;
    *:interpreter) ans='{"kind":"'$k'","pid":7,"exe_path":"/usr/bin/python3.12","exe_sha256":"'$EX'","cmdline":"python3 /srv/app/s.py","script_path":"/srv/app/s.py","script_sha256":"'$SC'"}'; exp='{"exe_sha256":"'$EX'","script_sha256":"'$SC'"}'; bound=$SC;;
    *:renamed) ans='{"kind":"'$k'","pid":7,"exe_path":"/opt/app/server","exe_sha256":"'$EX'","cmdline":"/opt/app/server s.py","runtime_hint":"libpython3.12","script_path":"/srv/app/s.py","script_sha256":"'$SC'"}'; exp='{"exe_sha256":"'$EX'","script_sha256":"'$SC'"}'; bound=$SC;;
  esac
  jq -n --argjson a "$ans" --argjson e "$exp" --arg fp "sha256:$bound" '{schema:"catalogizer.live-evidence/1",test_id:"matrix",class:"LIVE",verdict:"PASS",timestamp:"2026-11-03T20:00:00Z",command:"x",
    artifact:{fingerprint:$fp,source:"target"},endpoint:{host:"127.0.0.1",port:8080,resolved_ip:"127.0.0.1"},answering:$a,expected:$e,
    requests:{total:3},responses:{},runs:{n:3,identical:true,signature:"sha256:"}}'
}
cls() { "$V" "$1" 2>&1 | sed -n 's/.* class=\([A-Z]*\).*/\1/p' | head -1; }
chk() { # chk <label> <want LIVE|NOTLIVE> <jq edit> <base record file>
  local label=$1 want=$2 edit=$3 base=$4 f=$W/c$n.json c; n=$((n+1))
  jq "$edit" "$base" > "$f"; c=$(cls "$f")
  if [ "$want" = LIVE ] && [ "$c" != LIVE ]; then echo "FAIL: $label want LIVE got ${c:-?}" >&2; fail=$((fail+1))
  elif [ "$want" = NOTLIVE ] && { [ "$c" = LIVE ] || [ -z "$c" ]; }; then echo "FAIL: $label must not be LIVE (got ${c:-none})" >&2; fail=$((fail+1)); fi
}
for k in process socket container; do for v in native interpreter renamed; do
  base=$W/base-$k-$v.json; cell $k $v > "$base" || continue
  chk "$k/$v baseline" LIVE '.' "$base"
  chk "$k/$v upper SHA256: prefix on every answer hash" LIVE '(.answering|=(with_entries(if (.key|test("sha256|digest")) then .value=("SHA256:"+(.value|sub("^sha256:";"")|ascii_upcase)) else . end)))' "$base"
  for key in exe_sha256 image_digest script_sha256; do
    if [ "$(jq -r --arg key "$key" '.expected[$key] // empty' "$base")" != "" ]; then
      chk "$k/$v answer lacks $key" NOTLIVE "del(.answering.$key)" "$base"
      chk "$k/$v answer $key differs" NOTLIVE ".answering.$key=\"sha256:$OT\"" "$base"
      chk "$k/$v answer $key differs (upper SHA256:)" NOTLIVE ".answering.$key=\"SHA256:$OT\"" "$base"
      chk "$k/$v expected lacks $key" NOTLIVE "del(.expected.$key)" "$base"
    else
      # expected names a key the answer cannot carry (expected.$key present, answer field absent): never LIVE
      chk "$k/$v expected adds $key the answer lacks" NOTLIVE ".expected.$key=\"sha256:$OT\"" "$base"
    fi
  done
  chk "$k/$v fingerprint differs" NOTLIVE ".artifact.fingerprint=\"sha256:$OT\"" "$base"
  chk "$k/$v fingerprint differs (upper SHA256:)" NOTLIVE ".artifact.fingerprint=\"SHA256:$OT\"" "$base"
  chk "$k/$v fingerprint equal in other case" LIVE '.artifact.fingerprint |= ("SHA256:" + (sub("^sha256:";"")|ascii_upcase))' "$base"
  [ "$k" != container ] && chk "$k/$v no cmdline" NOTLIVE 'del(.answering.cmdline)' "$base"
  [ "$k" != container ] && chk "$k/$v cmdline names a registered double (any case)" NOTLIVE '.answering.cmdline += " --mode FAKE_SERVER"' "$base"
  [ "$v" != native ] && chk "$k/$v no script_sha256" NOTLIVE 'del(.answering.script_sha256)' "$base"
  [ "$v" != native ] && chk "$k/$v no script_path" NOTLIVE 'del(.answering.script_path)' "$base"
  [ "$v" != native ] && chk "$k/$v expected lacks script" NOTLIVE 'del(.expected.script_sha256)' "$base"
  chk "$k/$v exe hash is a registered double" NOTLIVE ".answering.${v/native/XX}=null | (if .answering.kind==\"container\" then .answering.image_digest else .answering.exe_sha256 end) = \"sha256:$(printf b | sha256sum | cut -d' ' -f1)\" | .expected |= with_entries(if (.key==\"exe_sha256\" or .key==\"image_digest\") then .value=\"sha256:$(printf b | sha256sum | cut -d' ' -f1)\" else . end)" "$base"
done; done
echo "identity matrix: cells=$n fail=$fail"
[ "$n" -ge 100 ] && [ "$fail" = 0 ]
