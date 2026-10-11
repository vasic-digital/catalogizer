#!/usr/bin/env bash
# fix-r6-probe.sh - WP-03 round 11 ground truth: what do git 2.53.0 and bash do with the surfaces the verifier relies on? Plain git vs the isolation.
# Output: TSV rows  ID<TAB>surface<TAB>plain<TAB>isolated<TAB>fools?<TAB>note   (plain = what an unprotected verifier would see)
set -u
export LC_ALL=C GIT_CONFIG_NOSYSTEM=1
HERE="$(cd "$(dirname "$0")" && pwd)"; FORGE="${FORGE:-$HERE/fix-r6-forge_cg.py}"
T="$(mktemp -d "${TMPDIR:-/tmp}/probe6.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
export HOME="$T/home"; mkdir -p "$HOME"
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
G="git -c user.email=t@t -c user.name=t"
row() { printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$@"; }
ANC() { local rc; "$@" >/dev/null 2>&1; rc=$?; echo "rc=$rc"; }
# diverged fixture: c1 <- c3 (local main) ; c1 <- c2a <- c2 (branch rem). TRUE: c3 is NOT an ancestor of c2 (rc=1)
mkd() { local d="$1"; git init -q -b main "$d"; ( cd "$d" || exit; export GIT_COMMITTER_DATE="2020-01-01T00:00:10Z" GIT_AUTHOR_DATE="2020-01-01T00:00:10Z"
  echo 1 > f; git add f; $G commit -qm c1
  git checkout -q -b rem; echo a > a; git add a; GIT_COMMITTER_DATE="2020-01-01T00:00:20Z" GIT_AUTHOR_DATE="2020-01-01T00:00:20Z" $G commit -qm c2a
  echo b > b; git add b; GIT_COMMITTER_DATE="2020-01-01T00:00:30Z" GIT_AUTHOR_DATE="2020-01-01T00:00:30Z" $G commit -qm c2
  git checkout -q main; echo 3 > c; git add c; GIT_COMMITTER_DATE="2020-01-01T00:00:40Z" GIT_AUTHOR_DATE="2020-01-01T00:00:40Z" $G commit -qm c3 ); }
# ---- replace refs
mkd "$T/rp"; C3=$(git -C "$T/rp" rev-parse main); C2=$(git -C "$T/rp" rev-parse rem)
FAKE=$(git -C "$T/rp" commit-tree -p "$C3" -m fake "$(git -C "$T/rp" rev-parse "$C2^{tree}")")
git -C "$T/rp" replace "$C2" "$FAKE"
row P01 "refs/replace/<c2> -> fake child of c3: is-ancestor c3 c2 (true: 1)" "$(ANC git -C "$T/rp" merge-base --is-ancestor $C3 $C2)" "$(ANC git -C "$T/rp" --no-replace-objects merge-base --is-ancestor $C3 $C2)" YES "isolated = --no-replace-objects"
row P02 "same, only GIT_NO_REPLACE_OBJECTS=1 in env" "$(ANC git -C "$T/rp" merge-base --is-ancestor $C3 $C2)" "$(GIT_NO_REPLACE_OBJECTS=1 ANC git -C "$T/rp" merge-base --is-ancestor $C3 $C2)" YES "env var works too"
git -C "$T/rp" config core.useReplaceRefs true
row P03 "repo core.useReplaceRefs=true with GIT_NO_REPLACE_OBJECTS=1" "n/a" "$(GIT_NO_REPLACE_OBJECTS=1 ANC git -C "$T/rp" merge-base --is-ancestor $C3 $C2)" no "env wins over repo config"
row P04 "repo core.useReplaceRefs=true with --no-replace-objects" "n/a" "$(ANC git -C "$T/rp" --no-replace-objects merge-base --is-ancestor $C3 $C2)" no "option wins over repo config"
# ---- grafts
mkd "$T/gr"; C3=$(git -C "$T/gr" rev-parse main); C2=$(git -C "$T/gr" rev-parse rem); printf '%s %s\n' "$C2" "$C3" > "$T/gr/.git/info/grafts"
row P05 "info/grafts '<c2> <c3>' (git 2.53)" "$(ANC git -C "$T/gr" merge-base --is-ancestor $C3 $C2)" "$(ANC git -C "$T/gr" --no-replace-objects merge-base --is-ancestor $C3 $C2)" YES "--no-replace-objects does NOT disable grafts in 2.53 (see P06)"
row P06 "same with GIT_GRAFT_FILE=/nonexistent" "n/a" "$(GIT_GRAFT_FILE=/nonexistent ANC git -C "$T/gr" merge-base --is-ancestor $C3 $C2)" no "GIT_GRAFT_FILE=<absent> is REQUIRED (the only switch)"
# ---- commit-graph, interior and tip forge, generation v1 and v2
for GV in 1 2; do
  mkd "$T/cg$GV"; C3=$(git -C "$T/cg$GV" rev-parse main); C2=$(git -C "$T/cg$GV" rev-parse rem); C2A=$(git -C "$T/cg$GV" rev-parse rem~1)
  git -C "$T/cg$GV" -c commitGraph.generationVersion=$GV commit-graph write --reachable --no-changed-paths
  python3 "$FORGE" "$T/cg$GV" "$C2A" "$C3" || echo "FORGE FAILED"
  row P07.$GV "forged commit-graph (generation v$GV): INTERIOR commit c2a's parent -> c3; is-ancestor c3 c2 (true: 1)" "$(ANC git -C "$T/cg$GV" merge-base --is-ancestor $C3 $C2)" "$(ANC git -C "$T/cg$GV" -c core.commitGraph=false merge-base --is-ancestor $C3 $C2)" YES "isolated = -c core.commitGraph=false"
  row P08.$GV "same forge: git log --format=%P -1 c2a (true: c1)" "$(git -C "$T/cg$GV" log --format=%P -1 $C2A | cut -c1-7)" "$(git -C "$T/cg$GV" -c core.commitGraph=false log --format=%P -1 $C2A | cut -c1-7)" YES "log/rev-list read the forged graph too"
done
mkd "$T/cgt"; C3=$(git -C "$T/cgt" rev-parse main); C2=$(git -C "$T/cgt" rev-parse rem)
git -C "$T/cgt" commit-graph write --reachable --no-changed-paths; python3 "$FORGE" "$T/cgt" "$C2" "$C3"
row P09 "forged commit-graph, TIP commit c2 parent -> c3" "$(ANC git -C "$T/cgt" merge-base --is-ancestor $C3 $C2)" "$(ANC git -C "$T/cgt" -c core.commitGraph=false merge-base --is-ancestor $C3 $C2)" "see value" "r2 reviewer's variant"
mkd "$T/cgc"; C3=$(git -C "$T/cgc" rev-parse main); C2=$(git -C "$T/cgc" rev-parse rem); C2A=$(git -C "$T/cgc" rev-parse rem~1)
git -C "$T/cgc" commit-graph write --reachable --no-changed-paths; python3 "$FORGE" "$T/cgc" "$C2A" "$C3"; git -C "$T/cgc" config core.commitGraph true
row P10 "repo core.commitGraph=true + forged graph, command-line -c core.commitGraph=false" "$(ANC git -C "$T/cgc" merge-base --is-ancestor $C3 $C2)" "$(ANC git -C "$T/cgc" -c core.commitGraph=false merge-base --is-ancestor $C3 $C2)" YES "command line wins over repo config"
row P11 "env GIT_COMMIT_GRAPH-like switch" "n/a" "none exists" no "core.commitGraph is config only"
# does a fetch write a commit-graph when the repo sets fetch.writeCommitGraph
mkd "$T/wcg"; git init -q --bare "$T/wcg.bare"; git -C "$T/wcg" push -q "$T/wcg.bare" main rem 2>/dev/null; git clone -q "$T/wcg.bare" "$T/wcg2" 2>/dev/null; git -C "$T/wcg2" config fetch.writeCommitGraph true; git -C "$T/wcg2" config fetch.writeCommitGraph true
git -C "$T/wcg" checkout -q main; echo n > "$T/wcg/n"; git -C "$T/wcg" add n; git -C "$T/wcg" commit -qm n; git -C "$T/wcg" push -q "$T/wcg.bare" main 2>/dev/null
rm -rf "$T/wcg2/.git/objects/info/commit-graph" "$T/wcg2/.git/objects/info/commit-graphs"; git -C "$T/wcg2" fetch -q origin 2>/dev/null; B="$(find "$T/wcg2/.git/objects/info" -name "commit-graph*" -type f | wc -l)"
git -C "$T/wcg" commit -q --allow-empty -m m; git -C "$T/wcg" push -q "$T/wcg.bare" main 2>/dev/null; rm -rf "$T/wcg2/.git/objects/info/commit-graph" "$T/wcg2/.git/objects/info/commit-graphs"
git -C "$T/wcg2" -c fetch.writeCommitGraph=false fetch -q origin 2>/dev/null; A="$(find "$T/wcg2/.git/objects/info" -name "commit-graph*" -type f | wc -l)"
row P12 "fetch with repo fetch.writeCommitGraph=true: commit-graph files written (1=written)" "$B" "$A" "writes" "-c fetch.writeCommitGraph=false stops it (read-only claim)"
git -C "$T/wcg2" commit -q --allow-empty -m x 2>/dev/null; git -C "$T/wcg" commit -q --allow-empty -m m2; git -C "$T/wcg" push -q "$T/wcg.bare" main 2>/dev/null; rm -rf "$T/wcg2/.git/objects/info/commit-graph" "$T/wcg2/.git/objects/info/commit-graphs"
git -C "$T/wcg2" -c core.commitGraph=false fetch -q origin 2>/dev/null; C="$(find "$T/wcg2/.git/objects/info" -name "commit-graph*" -type f | wc -l)"
row P12b "fetch with repo fetch.writeCommitGraph=true AND -c core.commitGraph=false (files written)" "n/a" "$C" "no" "core.commitGraph=false alone stops the write; -c fetch.writeCommitGraph=false is a redundant belt (mutant MR4 is EQUIVALENT)"
# ---- shallow
mkd "$T/sh"; C3=$(git -C "$T/sh" rev-parse main); C2=$(git -C "$T/sh" rev-parse rem); C2A=$(git -C "$T/sh" rev-parse rem~1); C1=$(git -C "$T/sh" rev-parse main~1)
echo "$C2A" > "$T/sh/.git/shallow"
row P13 "shallow file cutting c2a's parents: is-ancestor c1 c2 (true: 0)" "$(ANC git -C "$T/sh" merge-base --is-ancestor $C1 $C2)" "n/a" "no (fails closed)" "shallow only REMOVES ancestry: ahead/behind turn into diverged, never into clean"
echo "$C2" > "$T/sh/.git/shallow"
row P14 "shallow cutting c2 itself: is-ancestor c3 c3's own history unaffected; is-ancestor c1 c2" "$(ANC git -C "$T/sh" merge-base --is-ancestor $C1 $C2)" "n/a" "no (fails closed)" "same"
# ---- alternates
mkd "$T/al"; C3=$(git -C "$T/al" rev-parse main); C2=$(git -C "$T/al" rev-parse rem)
git clone -q --no-hardlinks "$T/al" "$T/al2" 2>/dev/null; git -C "$T/al2" checkout -q main 2>/dev/null
rm -rf "$T/al2/.git/objects/pack"/* ; echo "$T/al/.git/objects" > "$T/al2/.git/objects/info/alternates"
row P15 "objects only in an alternate store: cat-file -e c2 in a repo that lacks them locally" "$(ANC git -C "$T/al2" cat-file -e "$C2^{commit}")" "n/a" "no" "alternates add genuinely held objects (content-addressed); ancestry answered from them is true"
# ---- excludes
untr() { git -C "$1" status --porcelain --untracked-files=all 2>/dev/null | wc -l; }
mkr() { git init -q -b main "$1"; ( cd "$1" || exit; echo 1 > f; git add f; $G commit -qm c1 ); }
mkr "$T/ex1"; echo u > "$T/ex1/u"; echo u >> "$T/ex1/.git/info/exclude"
row P20 "info/exclude hides untracked u: status lines (true 1)" "$(untr "$T/ex1")" "per-dir only: see verifier" "YES" "needs the A-vs-D comparison"
mkr "$T/ex2"; echo u > "$T/ex2/u"; echo u > "$T/ex2.excl"; git -C "$T/ex2" config core.excludesFile "$T/ex2.excl"
row P21 "repo core.excludesFile (direct) hides u" "$(untr "$T/ex2")" "scope=$(git -C "$T/ex2" config --show-scope --get core.excludesFile | cut -f1)" YES "scope local"
mkr "$T/ex3"; echo u > "$T/ex3/u"; echo u > "$T/ex3.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/ex3.excl" > "$T/ex3.inc"; git -C "$T/ex3" config include.path "$T/ex3.inc"
row P22 "include.path in .git/config sets core.excludesFile: status lines; --local --get / --show-scope" "$(untr "$T/ex3")" "local-get='$(git -C "$T/ex3" config --local --get core.excludesFile)' scope=$(git -C "$T/ex3" config --show-scope --get core.excludesFile | cut -f1)" YES "--local does NOT follow include (empty); --show-scope reports local"
mkr "$T/ex4"; echo u > "$T/ex4/u"; echo u > "$T/ex4.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/ex4.excl" > "$T/ex4.inc"; printf '[includeIf "gitdir:%s/"]\n\tpath = %s\n' "$T/ex4" "$T/ex4.inc" >> "$T/ex4/.git/config"
row P23 "includeIf gitdir: sets core.excludesFile" "$(untr "$T/ex4")" "local-get='$(git -C "$T/ex4" config --local --get core.excludesFile)' scope=$(git -C "$T/ex4" config --show-scope --get core.excludesFile | cut -f1)" YES "same"
mkr "$T/ex5"; echo u > "$T/ex5/u"; echo u > "$T/ex5.excl"; git -C "$T/ex5" config extensions.worktreeConfig true; git -C "$T/ex5" config --worktree core.excludesFile "$T/ex5.excl"
row P24 "config.worktree sets core.excludesFile" "$(untr "$T/ex5")" "local-get='$(git -C "$T/ex5" config --local --get core.excludesFile)' scope=$(git -C "$T/ex5" config --show-scope --get core.excludesFile | cut -f1)" YES "scope worktree"
mkr "$T/ex6"; echo u > "$T/ex6/u"; echo u > "$T/ex6.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/ex6.excl" > "$T/ex6.glob"
row P25 "GLOBAL core.excludesFile hides u (GIT_CONFIG_GLOBAL)" "$(GIT_CONFIG_GLOBAL="$T/ex6.glob" untr "$T/ex6")" "scope=$(GIT_CONFIG_GLOBAL="$T/ex6.glob" git -C "$T/ex6" config --show-scope --get core.excludesFile | cut -f1)" "no (git calls it clean)" "scope global: decision N1 = clean"
mkr "$T/ex7"; echo u > "$T/ex7/u"; mkdir -p "$T/xdg/git"; echo u > "$T/xdg/git/ignore"
row P26 "XDG default ignore file (core.excludesFile unset)" "$(XDG_CONFIG_HOME="$T/xdg" untr "$T/ex7")" "excludesFile='$(XDG_CONFIG_HOME="$T/xdg" git -C "$T/ex7" config --get core.excludesFile)'" "no" "not a config value: no scope; clean per N1"
mkr "$T/ex8"; echo u > "$T/ex8/u"; echo u > "$T/ex8.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/ex8.excl" > "$T/ex8.glob"; printf '[core]\n\texcludesFile = /nonexistent/none\n' > "$T/ex8.loc"
git -C "$T/ex8" config core.excludesFile /nonexistent/none
row P27 "global file hides u, repo core.excludesFile overrides with another path (precedence)" "$(GIT_CONFIG_GLOBAL="$T/ex8.glob" untr "$T/ex8")" "scope=$(GIT_CONFIG_GLOBAL="$T/ex8.glob" git -C "$T/ex8" config --show-scope --get core.excludesFile | cut -f1)" "no" "last value wins: local replaces global entirely"
mkr "$T/ex9"; echo u > "$T/ex9/u"; echo u > "$T/ex9.excl"
row P28 "GIT_CONFIG_COUNT/KEY/VALUE sets core.excludesFile (caller env, scope command)" "$(GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.excludesFile GIT_CONFIG_VALUE_0="$T/ex9.excl" untr "$T/ex9")" "scope=$(GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.excludesFile GIT_CONFIG_VALUE_0="$T/ex9.excl" git -C "$T/ex9" config --show-scope --get core.excludesFile | cut -f1)" YES "dropped by the env isolation"
# ---- other repo config that hides dirt
mkr "$T/fm"; chmod +x "$T/fm/f"; git -C "$T/fm" config core.fileMode false
row P30 "core.fileMode=false hides a chmod +x (status lines; true 1)" "$(untr "$T/fm")" "-c core.fileMode=true: $(git -C "$T/fm" -c core.fileMode=true status --porcelain | wc -l)" YES "not switched off by the verifier (documented boundary)"
mkr "$T/ac"; git -C "$T/ac" config core.autocrlf true; printf 'a
' > "$T/ac/f"
row P31 "core.autocrlf=true, LF blob, CRLF worktree file (status lines)" "$(untr "$T/ac")" "-c core.autocrlf=false: $(git -C "$T/ac" -c core.autocrlf=false status --porcelain | wc -l)" "no (measured)" "NOT reproduced: git 2.53 reports M in both modes for a LF blob and a CRLF worktree file; no hiding observed"
# ---- BASH_ENV / ENV and exported functions into children
printf 'echo ran >> "%s/canary"\n' "$T" > "$T/benv.sh"
mkdir -p "$T/shims"; printf '#!/bin/bash\nexit 0\n' > "$T/shims/bash_np"; printf '#!/bin/bash -p\nexit 0\n' > "$T/shims/bash_p"; printf '#!/bin/sh\nexit 0\n' > "$T/shims/sh_s"; chmod +x "$T/shims"/*
cnt() { : > "$T/canary"; "$@" >/dev/null 2>&1; wc -l < "$T/canary"; }
cnti() { : > "$T/canary"; env -i PATH="$PATH" "$@" >/dev/null 2>&1; wc -l < "$T/canary"; }
row P40 "BASH_ENV set; child = '#!/bin/bash' script (a VERIFY_GIT shim): canary lines" "$(BASH_ENV="$T/benv.sh" cnt "$T/shims/bash_np")" "$(BASH_ENV="$T/benv.sh" cnti "$T/shims/bash_np")" YES "isolated = env -i / allow-list"
row P41 "BASH_ENV; child = '#!/bin/bash -p' script" "$(BASH_ENV="$T/benv.sh" cnt "$T/shims/bash_p")" "n/a" no "-p ignores BASH_ENV"
row P42 "BASH_ENV; child = '#!/bin/sh' script (sh is $(readlink -f /bin/sh))" "$(BASH_ENV="$T/benv.sh" cnt "$T/shims/sh_s")" "n/a" no "sh (dash/bash-as-sh) does not read BASH_ENV"
row P43 "BASH_ENV; child = bash -c" "$(BASH_ENV="$T/benv.sh" cnt bash -c :)" "$(BASH_ENV="$T/benv.sh" cnt bash -p -c :)" YES "bash -p -c is safe"
row P44 "ENV (not BASH_ENV) set, child = non-interactive bash script" "$(ENV="$T/benv.sh" cnt "$T/shims/bash_np")" "n/a" no "ENV is for interactive/posix-mode shells"
row P45 "BASH_ENV; the script's OWN startup when run as 'bash script' (interpreter path)" "$(printf 'true\n' > "$T/t.sh"; BASH_ENV="$T/benv.sh" cnt bash "$T/t.sh")" "$(BASH_ENV="$T/benv.sh" cnt bash -p "$T/t.sh")" YES "unavoidable for plain bash; -p / direct exec of a '#!/bin/bash -p' file does not source"
row P46 "BASH_ENV; real git running a shell alias (!sh) and a core.fsmonitor-less status" "$(git init -q "$T/ga"; git -C "$T/ga" config alias.x '!true'; BASH_ENV="$T/benv.sh" cnt git -C "$T/ga" x)" "n/a" no "git runs /bin/sh"
row P47 "exported function 'git' (BASH_FUNC_git%%) reaches a non-p bash child" "$(printf '#!/bin/bash\ntype -t git\n' > "$T/shims/ty"; chmod +x "$T/shims/ty"; git() { echo fn; }; export -f git; "$T/shims/ty" 2>&1; unset -f git)" "$(env -i PATH="$PATH" "$T/shims/ty")" YES "env -i drops BASH_FUNC_*"
printf '#!/bin/bash -p\nenv | grep -c "^BASH_FUNC_"\n' > "$T/shims/pfn"; chmod +x "$T/shims/pfn"
row P47b "exported function reaches the children of a '#!/bin/bash -p' script (count of BASH_FUNC_ in its child env)" "$(git() { :; }; export -f git; "$T/shims/pfn")" "n/a" "YES" "bash -p does NOT stop an exported function reaching its children (it only declines to import it): the detector must read the real child environment (env -0), compgen -e would miss it"
row P48 "SHELLOPTS=xtrace exported to a '#!/bin/bash' child: stderr lines" "$(printf '#!/bin/bash\ntrue\n' > "$T/shims/so"; chmod +x "$T/shims/so"; env SHELLOPTS=xtrace "$T/shims/so" 2>&1 | wc -l)" "$(env -i PATH="$PATH" "$T/shims/so" 2>&1 | wc -l)" YES "dropped by env -i"
row P49 "dirty-environment detector: 'env -0' lists what a child sees, exported function included (count of BASH_FUNC_ names)" "$(git() { :; }; export -f git; env -0 | tr '\0' '\n' | grep -c '^BASH_FUNC_')" "$(env -i A=1 env -0 | tr '\0' '\n' | grep -c '^BASH_FUNC_')" "detector" "env -0 (GNU coreutils) is the instrument; compgen -e would miss BASH_FUNC_x%%"
# ---- GIT_* variables a caller (or a git hook) exports
mkr "$T/e1"; mkr "$T/e2"; echo z > "$T/e2/z"
row P50 "GIT_DIR=<other repo>/.git : git -C e1 status lines (true 0)" "$(GIT_DIR="$T/e2/.git" git -C "$T/e1" status --porcelain 2>/dev/null | wc -l)" "env -i: $(env -i PATH="$PATH" HOME="$HOME" git -C "$T/e1" status --porcelain | wc -l)" YES "repo selected by env"
row P51 "GIT_INDEX_FILE=/nonexistent" "$(GIT_INDEX_FILE=/nonexistent/idx git -C "$T/e1" status --porcelain 2>&1 | wc -l)" "env -i: 0" YES "all repository-local env dropped by env -i"
row P52 "GIT_CONFIG_PARAMETERS='core.excludesfile=x'" "$(GIT_CONFIG_PARAMETERS="'core.fileMode'='false'" git -C "$T/e1" config --show-scope --get core.fileMode | cut -f1)" "env -i: $(env -i PATH="$PATH" HOME="$HOME" git -C "$T/e1" config --show-scope --get core.fileMode | cut -f1)" YES "scope command"
row P53 "GIT_EXTERNAL_DIFF / GIT_PAGER / GIT_TRACE do not change status/ancestry answers" "n/a" "n/a" no "not state-changing; dropped by isolation anyway"
