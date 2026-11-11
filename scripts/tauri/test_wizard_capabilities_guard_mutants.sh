#!/usr/bin/env bash
# Mutants of the guard itself (review round 3, TAURI-R3): each mutant is a one-line source transform of test_wizard_capabilities.sh;
# the paired suite (test_wizard_capabilities_mutation.sh, GUARD=<mutant>) MUST fail for every one (rc 1). Definitions are the python
# (old, new) pairs below, so they are reproducible. Exit 0 = all mutants killed, 1 = a mutant survived, 2 = setup error.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/wizcap-gm.XXXXXX")" || exit 2
trap 'rm -rf "${TMP:?}"' EXIT
bad=0
python3 -I - "$HERE/test_wizard_capabilities.sh" "$TMP" <<'PY'
import sys
src, out = sys.argv[1:3]
s = open(src).read()
M = {
 "M1_mask_comments_again": ('errors="replace").read()\n        covered', 'errors="replace").read()\n        t = re.sub(r"/\\*.*?\\*/|//[^\\n]*", "", t, flags=re.S)\n        covered'),
 "M2_drop_namedlist_form": ('elif isinstance(d, dict) and isinstance(d.get("capabilities"), list):', 'elif False:'),
 "M3_substring_window_match": ('fnmatch.fnmatchcase(target, w)', '(w == "*" or target in w)'),
 "M4_skip_test_dirs": ('    for fn in fns:\n        if not fn.endswith(EXTS):', '    dns[:] = [d for d in dns if d not in ("test", "__tests__")]\n    for fn in fns:\n        if not fn.endswith(EXTS):'),
 "M5_ts_only": ('EXTS = (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts")', 'EXTS = (".ts", ".tsx")'),
 "M6_unterminated_comment_tolerated": ('raise ValueError("unterminated /* comment")', 'j = n - 2'),
 "M7_no_excepthook": ('sys.excepthook = _blind', 'pass'),
 "M8_no_loose_fallback": ('if m.end() not in covered:', 'if False:'),
 "M9_identifier_unchecked": ('if not isinstance(c.get("identifier"), str):', 'if False:'),
}
for k, (a, b) in M.items():
    assert s.count(a) == 1, (k, s.count(a))
    open("%s/%s.sh" % (out, k), "w").write(s.replace(a, b))
PY
for m in "$TMP"/M*.sh; do
  rc=0; GUARD="$m" bash "$HERE/test_wizard_capabilities_mutation.sh" >/dev/null 2>&1 || rc=$?
  if [ "$rc" = 1 ]; then echo "killed   $(basename "$m" .sh)"; else echo "SURVIVED $(basename "$m" .sh) (suite rc=$rc)"; bad=1; fi
done
exit "$bad"
