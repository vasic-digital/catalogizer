#!/usr/bin/env bash
# Mutants of the guard itself (review round 3, TAURI-R3): each mutant is a one-line source transform of test_wizard_capabilities.sh;
# the paired suite (test_wizard_capabilities_mutation.sh, GUARD=<mutant>) MUST fail for every one (rc 1). Definitions are the python
# (old, new) pairs below, so they are reproducible. Exit 0 = all mutants killed, 1 = a mutant survived, 2 = setup error.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/wizcap-gm.XXXXXX")" || exit 2
trap 'chmod -R u+rwx "${TMP:?}" 2>/dev/null; rm -rf "${TMP:?}"' EXIT
bad=0
python3 -I - "$HERE/test_wizard_capabilities.sh" "$TMP" <<'PY'
import sys
src, out = sys.argv[1:3]
s = open(src).read()
M = {
 "M1_mask_comments_again": ('t1 = unescape(t0)', 't0 = re.sub(r"/\\*.*?\\*/|//[^\\n]*", "", t0, flags=re.S)\n        t1 = unescape(t0)'),
 "M2_drop_namedlist_form": ('if isinstance(d, dict) and "capabilities" in d:', 'if False:'),
 "M3_substring_window_match": ('fnmatch.fnmatchcase(target, w)', '(w == "*" or target in w)'),
 "M4_skip_test_dirs": ('    for fn in fns:\n        if not fn.endswith(EXTS):', '    dns[:] = [d for d in dns if d not in ("test", "__tests__")]\n    for fn in fns:\n        if not fn.endswith(EXTS):'),
 "M5_ts_only": ('EXTS = (".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts")', 'EXTS = (".ts", ".tsx")'),
 "M6_unterminated_comment_tolerated": ('raise ValueError("unterminated /* comment")', 'j = n - 2'),
 "M7_no_excepthook": ('sys.excepthook = _blind', 'pass'),
 "M8_no_mention_fallback": ('if not any(a <= m.start() < b for a, b in explained):', 'if False:'),
 "M9_identifier_unchecked": ('if not isinstance(c.get("identifier"), str):', 'if False:'),
 "M10_no_escape_decoding": ('t1 = unescape(t0)', 't1 = t0'),
 "M11_no_global_tauri_text": ('if "__TAURI__" in t:', 'if False:'),
 "M12_no_withGlobalTauri": ('if wg:', 'if False:'),
 "M13_identifier_before_namedlist": ('if isinstance(d, dict) and "capabilities" in d:', 'if isinstance(d, dict) and "capabilities" in d and "identifier" not in d:'),
 "M14_default_covers_all_five": ('{"dialog:default": {"dialog:allow-open", "dialog:allow-save", "dialog:allow-message"}}', '{"dialog:default": {"dialog:allow-" + c for c in CMDS}}'),
 "M15_ask_without_message": ('if need & {"dialog:allow-ask", "dialog:allow-confirm"}:', 'if False:'),
 "M16_no_followlinks": ('followlinks=True', 'followlinks=False'),
 "M17_no_symlink_loop_guard": ('if rp not in real_seen:', 'if True:'),
 "M18_walk_errors_ignored": ('onerror=walk_error', 'onerror=None'),
 "M19_mock_exemption_dropped": ('for m in mock.finditer(t):\n                explained.append((m.start(), m.end()))', 'for m in mock.finditer(t):\n                pass'),
 "M20_deny_flag_ignored": ('if target is not None and not denied_hit:', 'if target is not None:'),
 "M21_mock_exempts_everything": ('mock = re.compile(r"\\b(?:vi|jest)\\.(?:mock|doMock|unmock)\\s*\\(\\s*" + PKG)', 'mock = re.compile(r"(?s).*")'),
 "M22_unmentioned_fallback_dropped_unescaped": ('mention = re.compile(r"plugin[-:]dialog")', 'mention = re.compile(r"plugin-dialog")'),
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
