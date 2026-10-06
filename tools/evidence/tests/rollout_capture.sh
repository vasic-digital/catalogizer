#!/usr/bin/env bash
# T057: docs/06 section 17 rollout steps 1 to 4 run inside the pinned images, one transcript per step under $EV/wp05/rollout/ (identity header: sha256 of
# the tools and tests, the image id and its lock digests, host, UTC time; `# DONE` marker). Every leg is run through scripts/containers/run_pinned.sh with
# --network=none (the images are the lock entries; the digest lines are read from build/containers/images.lock.yaml).
#   step1  recorder + verifier: the tamper table (13.3 chain rows), duration self-test, missing-field records, 13.7 production properties  [IMG-TESTUTIL]
#   step2  runner wrappers: go (live, IMG-GO), bash (live, IMG-KCOV), vitest/gradle/cargo parsers on fixtures (IMG-TESTUTIL); live legs of the last three BLOCKED
#   step3  verdict deriver: the 18 scenario cases and the token cases [IMG-TESTUTIL]; the polarity switch on a real register defect is OWED (T069, T038, T031)
#   step4  anchors: delete-and-recompute and truncate rejected, strength probe downgrades to policy [IMG-TESTUTIL]
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd); cd "$root" || exit 1
F=specs/001-full-project-audit-remediation; D=${EVD:-$root/$F/evidence/wp05}/rollout; mkdir -p "$D"
LOCK=build/containers/images.lock.yaml
digests() { # digests IMG-ID : the lock entry's reference, tag intent, digest and platform digest
  python3 - "$LOCK" "$1" <<'P'
import sys, re
t = open(sys.argv[1]).read(); i = sys.argv[2]
m = re.search(r"^- id: %s\n(.*?)(?=^- id: |\Z)" % re.escape(i), t, re.S | re.M)
if not m: print("# image %s: NOT IN LOCK" % i); sys.exit()
for k in ("reference", "tag_intent", "digest", "platform_digest"):
    mm = re.search(r"^  %s: (\S+)" % k, m.group(1), re.M); print("# image %s %s: %s" % (i, k, mm.group(1) if mm else "-"))
P
}
hdr() { # hdr TITLE IMG...
  local t=$1; shift
  echo "# $t"; echo "# generated_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname) kernel=$(uname -r)"
  echo "# git_head=$(git rev-parse HEAD 2>/dev/null) (tree under test is uncommitted; the sha256 lines identify it)"
  local i; for i in "$@"; do digests "$i"; done
  local f; for f in tools/evidence/evrec tools/evidence/verify tools/evidence/verdict tools/evidence/evcore.py tools/evidence/evparse.py tools/evidence/evverdict.py tools/evidence/evanchor.py \
      tools/evidence/wrap-go.sh tools/evidence/wrap-bash.sh tools/evidence/wrap-vitest.sh tools/evidence/wrap-gradle.sh tools/evidence/wrap-cargo.sh tools/evidence/lib/wrap_common.sh "$F/contracts/evidence-record.schema.json"; do
    echo "# sha256 $(sha256sum "$f" | cut -d' ' -f1)  $f"; done
  for f in tools/evidence/tests/*.sh tools/evidence/tests/verdict_cases/*; do echo "# sha256 $(sha256sum "$f" | cut -d' ' -f1)  $f"; done
  echo "# sha256-tree fixtures $( (cd tools/evidence/tests/fixtures && find . -type f | LC_ALL=C sort | xargs sha256sum | sha256sum | cut -d' ' -f1) )  tools/evidence/tests/fixtures"
}
runp() { # runp IMG TEST [env...] : one container run, prints the test output and rc
  local img=$1 t=$2; shift 2
  echo "## $img: bash tools/evidence/tests/$t $*"; scripts/containers/run_pinned.sh --network=none "$img" -- env "$@" bash "tools/evidence/tests/$t" 2>&1; echo "rc=$?"
}
step=${1:-all}
s1() { { hdr "rollout step 1 (docs/06 s17): recorder and verifier in IMG-TESTUTIL: tamper table chain rows, duration self-test, five missing-field records, s13.7 production properties, the nine suites" IMG-TESTUTIL
         for t in test_evrec.sh test_evrec_more.sh test_evrec_r3.sh test_evrec_r4.sh test_evrec_r5.sh test_evrec_r6.sh test_evrec_r7.sh test_evrec_golden.sh test_evrec_hermetic.sh test_evidence_record_schema.sh; do runp IMG-TESTUTIL "$t" X=1; done; echo "# DONE"; } >"$D/step1-recorder-verifier.txt" 2>&1; }
s2() { { hdr "rollout step 2 (docs/06 s17): runner wrappers: Go live in IMG-GO, bash live in IMG-KCOV, the vitest/gradle/cargo parsers on fixtures in IMG-TESTUTIL (their live legs are blocked-unavailable until WP-11 / WP-14)" IMG-GO IMG-KCOV IMG-TESTUTIL
         runp IMG-GO test_wrap_go.sh X=1; runp IMG-KCOV test_wrap_bash.sh X=1
         for t in test_wrap_vitest.sh test_wrap_gradle.sh test_wrap_cargo.sh; do runp IMG-TESTUTIL "$t" X=1; done
         echo "blocked-unavailable: live seeded-failure legs of wrap-vitest.sh (IMG-NODE with the catalog-web toolchain, WP-11), wrap-gradle.sh (IMG-ANDROID, WP-14) and wrap-cargo.sh (IMG-RUST, WP-14); the remote-lane run of test_wrap_go.sh through scripts/build/dispatch.sh (T005b) was NOT made: dispatch.sh is another stream's uncommitted work, the live leg ran locally in IMG-GO"; echo "# DONE"; } >"$D/step2-wrappers.txt" 2>&1; }
s3() { { hdr "rollout step 3 (docs/06 s17): verdict deriver in IMG-TESTUTIL: the 18 scenario cases, token cases; the polarity switch on one real register defect is OWED" IMG-TESTUTIL
         runp IMG-TESTUTIL test_verdict.sh X=1
         echo "OWED: the polarity switch (docs/06 s17 step 3) on one real register defect reproducible inside P0 (first candidate docs/11 F-4): needs the findings register (T069) and the T038/T031 stale-tracking-ref case; if no P0 defect reproduces it is recorded in \$EV/p0-exit.json and closed by D-04 in T109."; echo "# DONE"; } >"$D/step3-verdict.txt" 2>&1; }
s4() { { hdr "rollout step 4 (docs/06 s17): anchors in IMG-TESTUTIL: delete-and-recompute and truncate rejected, anchor unreadable UNVERIFIED, strength probe downgrades to policy, rerecord anchor leg" IMG-TESTUTIL
         runp IMG-TESTUTIL test_anchors.sh X=1; echo "# DONE"; } >"$D/step4-anchors.txt" 2>&1; }
case $step in 1) s1;; 2) s2;; 3) s3;; 4) s4;; all) s1; s2; s3; s4;; esac
