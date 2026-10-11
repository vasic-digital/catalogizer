#!/usr/bin/env bash
# Guard test (TDD, constitution 11.4.224): no shell test may write a git hook whose body runs commit/merge/rebase/checkout/pull/reset... without a recursion guard.
# Root cause (host hang 2026-10-08): test_wp04k.sh installed `post-commit` = `git commit --amend` => post-commit fires again => unbounded recursion => host OOM.
# The scanner is STATIC (scan_recursive_hook_bodies.py reads text; no hook body is ever executed). Fixtures: a golden-bad (recursive) and a golden-good (guarded) hook, plus a control needle.
# Usage: bash scripts/repo/tests/test_no_recursive_hook_bodies.sh
set -u
D0="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 2; cd "$D0" || exit 2
SC="$D0/scripts/repo/tests/scan_recursive_hook_bodies.py"; SELF="$D0/scripts/repo/tests/test_no_recursive_hook_bodies.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/norechook.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
P=0; F=0; ok() { P=$((P+1)); echo "PASS $1"; }; bad() { F=$((F+1)); echo "FAIL $1"; }
scan() { python3 -I "$SC" "$@" 2>&1; }
mkdir -p "$T/fx"
w() { printf '%s\n' "#!/bin/bash" "$2" > "$T/fx/$1"; }          # w <fixture file> <one line of the fixture script>
# ---- golden-bad: every one of these hook bodies recurses (or cannot be shown not to) and MUST be flagged ----
w bad1.sh "printf '#!/bin/sh\\n%s\\n' 'git commit -q --amend -m x' > \"\$T/hooks/post-commit\""
w bad2.sh "hook_case \"amend\" post-commit 'git commit -q --amend --no-verify -m rewritten || true'"
{ echo '#!/bin/bash'; echo 'cat > "$T/hooks/post-checkout" <<EOH'; echo '#!/bin/sh'; echo 'git checkout -q main'; echo 'EOH'; } > "$T/fx/bad3.sh"
w bad4.sh "printf '#!/bin/sh\\ngit commit --amend -m x\\n' > \"\$T/hooks/post-commit\""                      # \n escapes inside ONE printf literal
w bad5.sh "printf '#!/bin/sh\\n%s\\n' 'git commit -q --amend' > \"\$HP/ghooks/post-commit\""                # custom core.hooksPath dir
w bad6.sh "printf '#!/bin/sh\\n\"\$GIT\" commit --amend -m x\\n' > \"\$T/hooks/post-commit\""              # variable git
w bad7.sh "printf '#!/bin/sh\\n/usr/bin/git commit --amend -m x\\n' > \"\$T/hooks/post-commit\""           # absolute git
w bad8.sh "printf '#!/bin/sh\\ngit push origin main\\n' > \"\$T/hooks/pre-push\""                           # pre-push running git push
w bad9.sh "hook_case \"x\" pre-push 'git push -q origin HEAD'"
printf '%s\n' '#!/bin/sh' 'git commit --amend -m recursion' > "$T/fx/evil_hook"
w bad10.sh "cp evil_hook \"\$T/h/post-commit\""                                                              # cp of a literal source that recurses
w bad11.sh "cp \"\$SRC\" \"\$T/h/post-commit\""                                                              # unreadable source
w bad12.sh "ln -s \"\$SRC\" \"\$T/h/post-commit\""
w bad13.sh "sed -i 's/x/y/' \"\$T/h/post-commit\""                                                           # in-place edit: body unknown
w bad14.sh "cat evil_hook > \"\$T/h/post-commit\""
w bad15.sh "hook_case \"m\" post-commit \"[ -e '\$T/hook_marker' ] && exit 0; git commit -q --amend -m r\""     # marker TESTED but never CREATED
w bad16.sh "hook_case \"m\" post-commit \"[ -f /etc/hostname ] && true; git commit -q --amend -m r\""          # an unrelated test is not a guard
w bad17.sh "hook_case \"m\" post-commit \"git commit -q --amend -m r; : > '\$T/hook_marker'; [ -e '\$T/hook_marker' ] && exit 0\""   # created AFTER the recursion
w bad18.sh "hook_case \"m\" post-commit '[ -n \"\${IN_HOOK:-}\" ] && exit 0; git commit --amend -m x'"          # env var tested, never set
w bad19.sh "hook_case \"m\" post-commit '[ -n \"\${IN_HOOK:-}\" ] ; IN_HOOK=1 git commit --amend -m x'"         # test never exits
{ echo '#!/bin/bash'; echo "BODY='git commit --amend -m x'"; echo "printf '%s\n' \"\$BODY\" > \"\$T/ghooks/post-commit\""; } > "$T/fx/bad20.sh"   # body in a variable
printf '%s\n' 'import os' 'def mk(hooks):' '    open(os.path.join(hooks, "post-commit"), "w").write("#!/bin/sh\ngit commit --amend -m x\n")' > "$T/fx/bad21.py"
printf '%s\n' 'def mk(h):' '    body = """#!/bin/sh' 'git commit --amend -m x' '"""' '    (h / "post-commit").write_text(body)' > "$T/fx/bad22.py"
# ---- golden-good: a REAL guard (tested + exits + set before the risky command), and harmless hooks ----
w good1.sh "hook_case \"amend\" post-commit \"[ -e '\$T/m' ] && exit 0; : > '\$T/m'; git commit -q --amend -m x\""
w good2.sh "printf '#!/bin/sh\\necho ran >> log\\nexit 0\\n' > \"\$T/hooks/pre-commit\""
{ echo '#!/bin/bash'; echo 'cat > "$T/hooks/post-commit" <<EOH'; echo '#!/bin/sh'; echo '[ -n "${IN_HOOK:-}" ] && exit 0; IN_HOOK=1 git commit --amend -m x'; echo 'EOH'; } > "$T/fx/good3.sh"
w good4.sh "printf '#!/bin/sh\\n[ -e \"\$T/m\" ] && exit 0\\ntouch \"\$T/m\"\\ngit commit --amend -m x\\n' > \"\$T/ghooks/post-commit\""   # \n form, touch
printf '%s\n' '#!/bin/sh' '[ -e /tmp/guard_m ] && exit 0' ': > /tmp/guard_m' 'git commit --amend -m x' > "$T/fx/good_hook"
w good5.sh "cp good_hook \"\$T/h/post-commit\""                                                              # cp of a GUARDED literal source
w good6.sh "cp \"\$SRC\" \"\$T/h/post-commit\"   # hook-scan: reviewed SRC is the harmless fixture hook_noop"
w good7.sh "printf '#!/bin/sh\\necho pushed\\n' > \"\$T/hooks/pre-push\""
printf '%s\n' 'import os' 'def mk(hooks):' '    open(os.path.join(hooks, "post-commit"), "w").write("#!/bin/sh\n[ -e /tmp/m ] && exit 0\n: > /tmp/m\ngit commit --amend -m x\n")' > "$T/fx/good8.py"
{ echo '#!/bin/bash'; echo 'cat > "$T/fx_hook.sh" <<EOH'; echo '#!/bin/sh'; echo '[ -e /tmp/m ] && exit 0'; echo ': > /tmp/m'; echo 'git commit --amend -m x'; echo 'EOH'; echo 'cp "$T/fx_hook.sh" "$T/h/post-commit"'; } > "$T/fx/good9.sh"   # same-file source, guarded
nf() { scan "$T/fx" | grep -c "/$1:"; }
for f in bad1.sh bad2.sh bad3.sh bad4.sh bad5.sh bad6.sh bad7.sh bad8.sh bad9.sh bad10.sh bad11.sh bad12.sh bad13.sh bad14.sh bad15.sh bad16.sh bad17.sh bad18.sh bad19.sh bad20.sh bad21.py bad22.py; do
  o="$(nf "$f")"; [ "$o" -ge 1 ] && ok "golden-bad $f is flagged" || bad "golden-bad $f NOT flagged (scanner blind)"; done
for f in good1.sh good2.sh good3.sh good4.sh good5.sh good6.sh good7.sh good8.py good9.sh; do
  o="$(nf "$f")"; [ "$o" -eq 0 ] && ok "golden-good $f passes" || { bad "golden-good $f falsely flagged"; scan "$T/fx" | grep "/$f:"; }; done
scan "$T/fx" >/dev/null; [ $? -eq 1 ] && ok "scanner exit 1 on fixtures with findings" || bad "scanner exit code on bad fixtures"
# control needles on the REAL test_wp04k.sh hook writer: its guard is `[ -e M ] && exit 0; : > M; git commit ...`
mkdir -p "$T/needle1" "$T/needle2" "$T/needle3"
cp scripts/repo/tests/test_wp04k.sh "$T/needle1/n.sh"; sed -i "s/\[ -e '[^']*hook_marker' \] \&\& exit 0; : > '[^']*hook_marker'; //" "$T/needle1/n.sh"
scan "$T/needle1" >/dev/null; [ $? -eq 1 ] && ok "control needle: test_wp04k.sh with its WHOLE guard stripped IS flagged" || bad "control needle: stripped-guard copy not flagged"
cp scripts/repo/tests/test_wp04k.sh "$T/needle2/n.sh"; sed -i "s/ : > '[^']*hook_marker';//" "$T/needle2/n.sh"      # reviewer mutation: remove ONLY the marker creation
cmp -s scripts/repo/tests/test_wp04k.sh "$T/needle2/n.sh" && bad "mutation (remove marker creation) did not change the copy - the needle is blind"
scan "$T/needle2" >/dev/null; [ $? -eq 1 ] && ok "reviewer mutation: marker TESTED but its creation removed => flagged" || bad "reviewer mutation (creation removed) NOT flagged"
cp scripts/repo/tests/test_wp04k.sh "$T/needle3/n.sh"; scan "$T/needle3" >/dev/null; [ $? -eq 0 ] && ok "control: the unmodified test_wp04k.sh is clean" || bad "control: unmodified test_wp04k.sh falsely flagged"
# hook_case's timeout is exercised for real: the copy of test_wp04k.sh up to the post-commit case, whose hook body is replaced by one that never returns
# (a non-returning hook is exactly what the timeout exists to bound; plain `git commit --amend` in post-commit was measured NOT to re-fire here - see docs), run INSIDE a tiny scope
if command -v systemd-run >/dev/null 2>&1 && systemctl --user show -p Version >/dev/null 2>&1; then
  L="$(grep -n '^hook_case "post-commit hook amends the message"' scripts/repo/tests/test_wp04k.sh | cut -d: -f1)"
  U="scripts/repo/tests/.hang_wp04k.$$.sh"
  { sed -n "1,$((L-1))p" scripts/repo/tests/test_wp04k.sh; echo 'hook_case "post-commit hook amends the message" post-commit "sleep 60"'; } > "$U"
  st=$SECONDS; out="$(HOOK_CASE_TIMEOUT=3 systemd-run --user --scope -q -p TasksMax=100 -p MemoryMax=1G timeout -k 5 150 bash "$U" 2>&1)"; took=$((SECONDS - st)); rm -f "$U"
  case "$out" in *"FAIL hook_case [post-commit hook amends the message]: the hook-driven commit did not finish in 3s"*) ok "hook_case timeout fires (3s) on a hook that never returns, inside a TasksMax=100 scope (whole run ${took}s)";; *) bad "hook_case timeout did NOT fire on a non-returning hook"; printf '%s\n' "$out" | tail -5;; esac
else echo "SKIP hook_case timeout exercise: no systemd --user scope available"; fi
# the real repo: every shell test under scripts/ and tests/ (this file excluded: it contains the fixtures as text)
out="$(scan "$D0/scripts" --exclude "$SELF"; [ -d "$D0/tests" ] && scan "$D0/tests" --exclude "$SELF")"
[ -z "$out" ] && ok "repo scan: no unguarded recursive hook body" || { bad "repo scan found unguarded hook bodies:"; echo "$out"; }
echo "RESULT pass=$P fail=$F"; [ "$F" -eq 0 ]
