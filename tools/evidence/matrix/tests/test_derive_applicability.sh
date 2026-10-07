#!/usr/bin/env bash
# test_derive_applicability.sh - WF11 review I1, I2, I7, I11, m1, m2, m11 (RED first against the T196 deriver). Oracle for tools/evidence/matrix/derive_applicability.py.
# The deriver reads a repository, so every leg builds a SYNTHETIC repository in a temp directory (a doc05 4.4 table with only P/A cells, a .gitmodules listing the 30 modules the
# deriver names plus two governance modules, one submodule directory per module) and compares the deriver's cells with HAND-WRITTEN expectations (counted here from the
# fixture files, never taken from the deriver). Legs: determinism; an uninitialised submodule is refused; path-TOKEN markers (e2e is not e2ee, load is not loader, fault is not
# defaults, perf is not perfetto); first-party scripts/build and scripts/coverage are counted; the A9 unit cell is re-read from tests/; the bank count leaves out MANIFEST.yaml; the
# Website DDoS cell flips to A when a hosting config appears; the unit P threshold (10 test files) is exact. Paired mutations: copies of the deriver with ONE expression changed
# must each make this body FAIL, preceded by a sandbox control (the unmutated copy must pass).
# Run through `scripts/test-in-container.sh tooling unit -- bash tools/evidence/matrix/tests/test_derive_applicability.sh` (IMG-TESTUTIL has python3 + PyYAML).
# Usage: test_derive_applicability.sh [--no-mutations]    Env: DERIVE (deriver under test), MUTATION_RECORD
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DERIVE="${DERIVE:-$HERE/../derive_applicability.py}"
PASSES=0; FAILS=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
T="$(mktemp -d "${TMPDIR:-/tmp}/derive-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -f "$DERIVE" ] || { bad "deriver not found at $DERIVE"; echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"; exit 1; }
A10="assets auth cache concurrency config database discovery entities event_bus filesystem lazy media memory middleware observability rate_limiter recovery security storage streaming watcher"
A11="auth_context_react catalogizer_api_client_ts collection_manager_react dashboard_analytics_react media_browser_react media_player_react media_types_ts ui_components_react websocket_client_ts"
A12="constitution helix_qa"

mkrepo() { # mkrepo DIR : the fixture repository with every module populated by one placeholder file
  local r="$1"; rm -rf "$r"; mkdir -p "$r/specs/001-full-project-audit-remediation/docs" "$r/submodules" "$r/Build/lib" "$r/Website" "$r/tests" "$r/scripts"
  python3 -I - "$r/specs/001-full-project-audit-remediation/docs/05-test-strategy-and-coverage-matrix.md" <<'PY'
import sys
types=["unit","integration","e2e","full_automation","security","ddos","scaling","chaos","stress","performance","benchmarking","ui","ux","challenges","helixqa"]
cols=["A1 api","A2 web","A3 desktop","A4 installer","A5 android","A6 tv","A7 client","A8 Website","A9 Build","A10 Go mods","A11 TS mods"]
out=["### 4.4 The matrix (applications x fifteen types)","","Legend: fixture.","","| Type | "+" | ".join(cols)+" |","|---|"+"---|"*len(cols)]
for i,t in enumerate(types,1):
    out.append("| %d %s | "%(i,t)+" | ".join("A" for _ in cols)+" |")
out.append("")
open(sys.argv[1],"w").write("\n".join(out)+"\n")
PY
  : >"$r/.gitmodules"
  for m in $A10 $A11 $A12; do
    printf '[submodule "%s"]\n\tpath = submodules/%s\n\turl = x\n' "$m" "$m" >>"$r/.gitmodules"
    mkdir -p "$r/submodules/$m"; echo "placeholder" >"$r/submodules/$m/README.md"
  done
  printf '{"name":"w"}\n' >"$r/Website/package.json"
}
derive() { python3 -I "$DERIVE" --repo "$1" --out "${2:--}" >"$T/stdout" 2>"$T/stderr"; RC=$?; }
cellf() { python3 -I - "$1" "$2" "$3" "$4" <<'PY'
import sys,yaml
d=yaml.safe_load(open(sys.argv[1])); c=d["components"][sys.argv[2]]["cells"][sys.argv[3]]
print(c["state"] if sys.argv[4]=="state" else c["reason"])
PY
}

R="$T/repo"; mkrepo "$R"
# -- determinism (the generator claims byte-identical output; the old test did not run the deriver at all)
derive "$R" "$T/a1.yaml"; check "golden: the deriver exits 0 on a populated fixture" "$RC" 0
derive "$R" "$T/a2.yaml"; check "golden: two runs over the same tree are byte-identical" "$(sha256sum <"$T/a1.yaml")" "$(sha256sum <"$T/a2.yaml")"
check "golden: 30 modules + 2 governance modules + 9 applications + A13 = 42 components" "$(python3 -I -c "import yaml;print(len(yaml.safe_load(open('$T/a1.yaml'))['components']))")" 42
grep -q '^# enumeration: walk; files_fingerprint: [0-9a-f]\{32\} ' "$T/a1.yaml" && ok "m1: the header records the enumeration mode and a files_fingerprint (provenance of what was counted)" || bad "m1: provenance header missing: $(sed -n 5p "$T/a1.yaml")"
# the fingerprint (the hex only, not the count in the same header line) moves when a counted file is ADDED and when one is merely RENAMED (same count)
fp() { sed -n 's/^# enumeration: [^;]*; files_fingerprint: \([0-9a-f]*\) .*/\1/p' "$1"; }
[ -n "$(fp "$T/a1.yaml")" ] && [ "$(fp "$T/a1.yaml")" != "00000000000000000000000000000000" ] && ok "m1: the fingerprint is a real digest (not empty, not constant zeros)" || bad "m1: fingerprint is '$(fp "$T/a1.yaml")'"
echo more >"$R/submodules/auth/extra.txt"; derive "$R" "$T/a3.yaml"
[ "$(fp "$T/a1.yaml")" != "$(fp "$T/a3.yaml")" ] && ok "m1: the fingerprint changes when a counted file is added" || bad "m1: the fingerprint did not change on an added file"
rm "$R/submodules/auth/extra.txt"; mv "$R/submodules/auth/README.md" "$R/submodules/auth/README2.md"; derive "$R" "$T/a4.yaml"
[ "$(fp "$T/a1.yaml")" != "$(fp "$T/a4.yaml")" ] && ok "m1: the fingerprint changes when a file is only RENAMED (same file count)" || bad "m1: the fingerprint did not change on a rename"
mv "$R/submodules/auth/README2.md" "$R/submodules/auth/README.md"; derive "$R" "$T/a5.yaml"
check "m1 control: restoring the name restores the fingerprint (it is a function of the counted files)" "$(fp "$T/a5.yaml")" "$(fp "$T/a1.yaml")"

# -- I1: an uninitialised (empty) submodule is refused, never read as "no tests"
mkrepo "$R"; rm -rf "$R/submodules/auth"; mkdir -p "$R/submodules/auth"
derive "$R" "$T/e1.yaml"
check "I1: an empty submodule directory is refused (exit 3)" "$RC" 3
grep -q 'submodule_uninitialised' "$T/stderr" && grep -q 'auth' "$T/stderr" && ok "I1: the refusal names submodule_uninitialised and the module" || bad "I1: refusal text: $(cat "$T/stderr")"
[ ! -e "$T/e1.yaml" ] && ok "I1: a refused derivation writes no applicability file" || bad "I1: output written despite the refusal"
mkrepo "$R"; rm -rf "$R/submodules/cache" "$R/submodules/media"
derive "$R" "$T/e2.yaml"; [ "$RC" = 3 ] && grep -q 'cache' "$T/stderr" && grep -q 'media' "$T/stderr" && ok "I1: a MISSING directory is refused too, every empty module is named (2 listed)" || bad "I1: missing dirs not both named (rc $RC): $(cat "$T/stderr")"
mkrepo "$R"; derive "$R" "$T/e3.yaml"; check "I1 control: the same fixture populated again derives (exit 0)" "$RC" 0

# -- I2: markers are path TOKENS
mkrepo "$R"; S="$R/submodules/assets"
mkdir -p "$S/pkg/defaults" "$S/pkg/e2ee" "$S/pkg/loader" "$S/pkg/perfetto" "$S/pkg/injector" "$S/tests/e2e" "$S/pkg/stress"
printf 'package x\nimport "net/http"\nvar _ = http.Get\n' >"$S/server.go"; echo 'module m' >"$S/go.mod"
for f in pkg/defaults/defaults_test.go pkg/e2ee/crypto_test.go pkg/loader/loader_test.go pkg/perfetto/perfetto_test.go pkg/injector/injector_test.go; do printf 'package x\nfunc TestA(t *testing.T){}\n' >"$S/$f"; done
derive "$R" "$T/t1.yaml"
check "I2: pkg/defaults/defaults_test.go does not make chaos partial (fault is not defaults): the cell stays A" "$(cellf "$T/t1.yaml" 'A10:assets' chaos state)" "A"
check "I2: pkg/e2ee/* is not e2e (e2ee is encryption): the e2e cell stays A" "$(cellf "$T/t1.yaml" 'A10:assets' e2e state)" "A"
check "I2: loader_test.go does not make stress partial (load is not loader)" "$(cellf "$T/t1.yaml" 'A10:assets' stress state)" "A"
check "I2: perfetto_test.go does not make performance partial (perf is not perfetto)" "$(cellf "$T/t1.yaml" 'A10:assets' performance state)" "A"
check "I2: injector_test.go does not make security partial (inject is not injector)" "$(cellf "$T/t1.yaml" 'A10:assets' security state)" "A"
printf 'package x\nfunc TestE(t *testing.T){}\n' >"$S/tests/e2e/flow_test.go"; printf 'package x\nfunc TestS(t *testing.T){}\n' >"$S/pkg/stress/stress_test.go"
derive "$R" "$T/t2.yaml"
check "I2 golden-false: tests/e2e/flow_test.go IS e2e: the cell becomes ~ (1 file)" "$(cellf "$T/t2.yaml" 'A10:assets' e2e state)" "~"
cellf "$T/t2.yaml" 'A10:assets' e2e reason | grep -q '1 test file' && ok "I2: the e2e reason counts exactly the 1 real file (not the e2ee ones)" || bad "I2: e2e reason: $(cellf "$T/t2.yaml" 'A10:assets' e2e reason)"
check "I2 golden-false: pkg/stress/stress_test.go IS a stress marker: the cell becomes ~" "$(cellf "$T/t2.yaml" 'A10:assets' stress state)" "~"
# -- I2 (the same class, two more sites): a Go build tag and a render( call are matched as tokens too
mkrepo "$R"; S="$R/submodules/auth"; echo 'module m' >"$S/go.mod"; printf '//go:build disintegration\n\npackage x\nfunc TestA(t *testing.T){}\n' >"$S/a_test.go"
derive "$R" "$T/g1.yaml"; check "I2: a build tag that merely CONTAINS integration (disintegration) does not make the integration cell partial" "$(cellf "$T/g1.yaml" 'A10:auth' integration state)" "A"
printf '//go:build integration\n\npackage x\nfunc TestB(t *testing.T){}\n' >"$S/b_test.go"
derive "$R" "$T/g2.yaml"; check "I2 golden-false: the build tag integration makes the cell partial" "$(cellf "$T/g2.yaml" 'A10:auth' integration state)" "~"
mkrepo "$R"; S="$R/submodules/ui_components_react"; printf '{"dependencies":{"react":"18"}}\n' >"$S/package.json"; printf 'it("a", () => { prerender(x); });\n' >"$S/a.test.tsx"
derive "$R" "$T/r1.yaml"; check "I2: prerender( is not a render( call: the ui cell stays A" "$(cellf "$T/r1.yaml" 'A11:ui_components_react' ui state)" "A"
printf 'it("b", () => { render(x); });\n' >"$S/b.test.tsx"
derive "$R" "$T/r2.yaml"; check "I2 golden-false: a real render( call makes the ui cell partial" "$(cellf "$T/r2.yaml" 'A11:ui_components_react' ui state)" "~"

# -- I2: first-party directories that happen to be called build / coverage are COUNTED
mkdir -p "$R/scripts/build/tests" "$R/scripts/coverage/tests" "$R/scripts/other" "$R/node_modules/pkg" "$R/tools/opensource/x"
for f in scripts/build/tests/test_a.sh scripts/coverage/tests/test_b.sh scripts/other/test_c.sh; do echo '#!/bin/sh' >"$R/$f"; done
echo '#!/bin/sh' >"$R/node_modules/pkg/test_vendored.sh"; echo '#!/bin/sh' >"$R/tools/opensource/x/test_third_party.sh"
derive "$R" "$T/t3.yaml"
cellf "$T/t3.yaml" A13 unit reason | grep -q '3 `test_\*.sh`' && ok "I2: scripts/build, scripts/coverage and scripts/other tests are counted (3); node_modules and tools/opensource are not" || bad "I2: A13 unit reason: $(cellf "$T/t3.yaml" A13 unit reason)"

# -- I11: the A9 build unit cell is re-read from tests/
mkrepo "$R"; derive "$R" "$T/b0.yaml"
check "I11: without a build test the unit cell keeps docs/05's reading (A)" "$(cellf "$T/b0.yaml" A9 unit state)" "A"
printf '#!/usr/bin/env bash\nsource "$P/Build/lib/common.sh"\nsource "$P/Build/lib/hash.sh"\ntest_one() { :; }\ntest_two() { :; }\n' >"$R/tests/test_build_system.sh"
derive "$R" "$T/b1.yaml"
check "I11: tests/test_build_system.sh that sources Build/lib makes the unit cell ~" "$(cellf "$T/b1.yaml" A9 unit state)" "~"
cellf "$T/b1.yaml" A9 unit reason | grep -q 'sources 2 Build/lib script(s) (common.sh, hash.sh)' && cellf "$T/b1.yaml" A9 unit reason | grep -q 'defines 2 test function' && ok "I11: the reason names the 2 libraries and the 2 test functions it counted" || bad "I11: reason: $(cellf "$T/b1.yaml" A9 unit reason)"

# -- m2: MANIFEST.yaml is not a bank
mkdir -p "$R/challenges/helixqa-banks"; for i in 1 2 3; do echo "b: $i" >"$R/challenges/helixqa-banks/bank$i.yaml"; done; echo "m: 1" >"$R/challenges/helixqa-banks/MANIFEST.yaml"
derive "$R" "$T/m1.yaml"; cellf "$T/m1.yaml" A13 helixqa reason | grep -q '^TS-00 read: 3 bank files' && ok "m2: 3 banks + MANIFEST.yaml count as 3 banks" || bad "m2: helixqa reason: $(cellf "$T/m1.yaml" A13 helixqa reason)"

# -- m11: the Website DDoS cell is re-measured
derive "$R" "$T/w1.yaml"; check "m11: Website with no hosting config: DDoS n/a" "$(cellf "$T/w1.yaml" A8 ddos state)" "n/a"
echo 'server {}' >"$R/Website/nginx.conf"; derive "$R" "$T/w2.yaml"
check "m11: a Website/nginx.conf flips the DDoS cell to A (there is now server config of ours)" "$(cellf "$T/w2.yaml" A8 ddos state)" "A"
cellf "$T/w2.yaml" A8 ddos reason | grep -q 'nginx.conf' && ok "m11: the reason names the file found" || bad "m11: reason: $(cellf "$T/w2.yaml" A8 ddos reason)"
rm "$R/Website/nginx.conf"

# -- RM5: the unit P threshold is exactly 10 test files for a Go module
mkrepo "$R"; S="$R/submodules/auth"; echo 'module m' >"$S/go.mod"
for i in 1 2 3 4 5 6 7 8 9; do printf 'package x\nfunc TestA(t *testing.T){}\n' >"$S/a${i}_test.go"; done
derive "$R" "$T/p1.yaml"; check "RM5: 9 Go test files: unit is ~" "$(cellf "$T/p1.yaml" 'A10:auth' unit state)" "~"
printf 'package x\nfunc TestA(t *testing.T){}\n' >"$S/a10_test.go"
derive "$R" "$T/p2.yaml"; check "RM5: 10 Go test files: unit is P" "$(cellf "$T/p2.yaml" 'A10:auth' unit state)" "P"
rm -f "$S"/a*_test.go; derive "$R" "$T/p3.yaml"; check "RM5: no Go test file: unit is A" "$(cellf "$T/p3.yaml" 'A10:auth' unit state)" "A"

# -- the REAL repository: the derivation is deterministic and the committed file is what the tracked tree derives (a drift is a finding)
REPO="$(cd "$HERE/../../../.." && pwd)"
if [ -d "$REPO/submodules/auth" ] && [ -n "$(ls -A "$REPO/submodules/auth" 2>/dev/null)" ]; then
  python3 -I "$DERIVE" --repo "$REPO" --out "$T/real1.yaml" >/dev/null 2>&1; r1=$?
  python3 -I "$DERIVE" --repo "$REPO" --out "$T/real2.yaml" >/dev/null 2>&1
  check "real repository: derives (exit 0)" "$r1" 0
  check "real repository: two runs are byte-identical" "$(sha256sum <"$T/real1.yaml")" "$(sha256sum <"$T/real2.yaml")"
  grep -q 'enumeration: git-tracked' "$T/real1.yaml" && ok "real repository: counted from git-tracked files" || bad "real repository: not git-tracked: $(sed -n 5p "$T/real1.yaml")"
else echo "SKIP: real-repository legs (submodules not checked out here)"; fi

if [ "${1:-}" != --no-mutations ] && [ -z "${DERIVE_MUTANT:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  mut() { # mut NAME 'old' 'new'
    local name="$1" old="$2" new="$3" d="$T/mut-$1"; rm -rf "$d"; mkdir -p "$d"; cp "$DERIVE" "$d/derive_applicability.py"
    python3 -I - "$d/derive_applicability.py" "$old" "$new" <<'PY' || { bad "mutation $name: anchor not unique"; return; }
import sys
s=open(sys.argv[1]).read()
if s.count(sys.argv[2])!=1: sys.exit(1)
open(sys.argv[1],"w").write(s.replace(sys.argv[2],sys.argv[3]))
PY
    if DERIVE="$d/derive_applicability.py" DERIVE_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-$name.out" 2>&1; then bad "mutation $name SURVIVED"; echo "SURVIVED $name" >>"$REC"
    else ok "mutation $name caught ($(grep -c '^FAIL:' "$T/mut-$name.out") failing legs)"; echo "CAUGHT $name: $(grep '^FAIL:' "$T/mut-$name.out" | head -2 | cut -c1-110 | tr '\n' '|')" >>"$REC"; fi
  }
  d="$T/mut-ctl"; rm -rf "$d"; mkdir -p "$d"; cp "$DERIVE" "$d/derive_applicability.py"
  if DERIVE="$d/derive_applicability.py" DERIVE_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-ctl.out" 2>&1; then ok "mutation sandbox control: an UNMUTATED copy passes this body"; echo "CONTROL PASS" >>"$REC"
  else bad "mutation sandbox control FAILED ($(grep -c '^FAIL:' "$T/mut-ctl.out") failing legs): every CAUGHT below is suspect"; echo "CONTROL FAIL" >>"$REC"; fi
  mut tag_token_off '_tok(("integration", "integrations")).search(t)]   # MUT:tag_token' '"integration" in t]   # MUT:tag_token'
  mut render_call_off 're.search(r"(?<![A-Za-z0-9_])render\(", t): f["a11y"] += 1   # MUT:render_call' '"render(" in t: f["a11y"] += 1   # MUT:render_call'
  mut RM5_unit_threshold 'c["unit"] = ("P" if tf >= 10 else "~" if tf >= 1 else "A"' 'c["unit"] = ("P" if tf >= 0 else "~" if tf >= 1 else "A"'
  mut empty_submodule_guard 'if empty:' 'if False:'
  mut token_markers_off 'return any(("_" + w + "_") in joined for w in words)' 'return any(w in rel.lower() for w in words)'
  mut vendored_only 'SKIP_DIRS = {".git", "node_modules", "vendor", "opensource"}' 'SKIP_DIRS = {".git", "node_modules", "vendor", "opensource", "build", "coverage"}'
  mut manifest_counted 'os.path.basename(p) != "MANIFEST.yaml"' 'True'
  mut website_frozen 'if hits:' 'if False:'
  mut build_reread_off 'if libs:' 'if False:'
  mut fingerprint_const 'hashlib.sha256("\n".join("%s %s %d %s" % ((k,) + v) for k, v in sorted(_FPR.items())).encode()).hexdigest()[:32]' '"0" * 32'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=0"
[ "$FAILS" = 0 ]
