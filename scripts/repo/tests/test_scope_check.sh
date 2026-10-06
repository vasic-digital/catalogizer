#!/usr/bin/env bash
# T040 test (TDD): scripts/repo/scope_check.sh - CPA stage S2 scope check (build output, databases, .env files, dirty submodules,
# append-only evidence stores, content-addressed blobs). Throwaway repositories only.
# Usage: bash scripts/repo/tests/test_scope_check.sh   Env: H=<helper>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/scope_check.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/sc_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
R="$T/r"; mkrepo "$R"; EV=ev
cat > "$R/.gitignore" <<'G'
/build/output/
/build/containers/go/*
!/build/containers/go/Containerfile
catalog-web/build/
G
mkdir -p "$R/src" "$R/docs" "$R/$EV/blobs" "$R/scripts/build" "$R/build/containers/go"
echo a > "$R/src/a.go"; echo db > "$R/docs/workable_items.db"; printf 'l1\nl2\n' > "$R/$EV/ledger.jsonl"; printf 'a1\n' > "$R/$EV/anchors.jsonl"
commit_all "$R" init
w() { mkdir -p "$(dirname "$R/$1")"; printf '%b' "$2" > "$R/$1"; }
run() { ( cd "$R" && "$H" --root "$R" --ev "$EV" --exceptions "${EXC:-/dev/null}" "$@" ) >"$T/out" 2>"$T/err"; RC=$?; }
lst() { printf '%s\n' "$@" > "$T/cs.lst"; }
# 1 golden-true: ordinary source, the tracked register database, planned tracked build paths
w src/b.go 'b\n'; w build/containers/go/Containerfile 'FROM x\n'; w build/components.json '{}\n'; w scripts/build/verify_artifact.sh '#!/bin/sh\n'; echo db2 > "$R/docs/workable_items.db"
lst src/b.go docs/workable_items.db build/containers/go/Containerfile build/components.json scripts/build/verify_artifact.sh
run --paths-from "$T/cs.lst"; eq "golden-true change set: 0" "$RC" 0
# 2 refused paths (13), each with its reason named
refused() { # refused <label> <path> <content>
  w "$2" "$3"; lst "$2"; run --paths-from "$T/cs.lst"; eq "$1: 13" "$RC" 13; has "$1 names the path" "$(cat "$T/out")" "$2"; rm -f "$R/$2"
}
refused "build output build/output/x.bin" build/output/x.bin 'x'
refused "build output build/containers/go/x.bin" build/containers/go/x.bin 'x'
refused "build output catalog-web/build/x.js" catalog-web/build/x.js 'x'
refused "other database other/x.db" other/x.db 'x'
refused "an .env file" .env 'K=V\n'
refused "a nested .env.local" app/.env.local 'K=V\n'
w .env.example 'K=\n'; lst .env.example; run --paths-from "$T/cs.lst"; eq ".env.example: 0" "$RC" 0
w build_notes/build.md 'name match only\n'; lst build_notes/build.md; run --paths-from "$T/cs.lst"; eq "a path merely named build is not build output: 0" "$RC" 0
w docs/db_notes.md 'x\n'; lst docs/db_notes.md; run --paths-from "$T/cs.lst"; eq "a name merely containing db is not a database: 0" "$RC" 0
# 3 append-only evidence stores judged against HEAD
printf 'l1\nl2\nl3\n' > "$R/$EV/ledger.jsonl"; lst "$EV/ledger.jsonl"; run --paths-from "$T/cs.lst"; eq "pure append to the ledger: 0" "$RC" 0
printf 'l1\nL2\n' > "$R/$EV/ledger.jsonl"; run --paths-from "$T/cs.lst"; eq "an earlier line changed: 13" "$RC" 13; has "reason append_only" "$(cat "$T/out")" "append_only"
printf 'l1\n' > "$R/$EV/ledger.jsonl"; run --paths-from "$T/cs.lst"; eq "a line removed: 13" "$RC" 13
rm -f "$R/$EV/ledger.jsonl"; run --paths-from "$T/cs.lst"; eq "the store deleted: 13" "$RC" 13
git -C "$R" checkout -q -- "$EV/ledger.jsonl"
for s in anchors.jsonl deferrals.jsonl flake_ledger.jsonl; do
  [ -f "$R/$EV/$s" ] || { printf 'x1\n' > "$R/$EV/$s"; commit_all "$R" "add $s"; }
  printf 'zz\n' > "$R/$EV/$s"; lst "$EV/$s"; run --paths-from "$T/cs.lst"; eq "rewritten store $s: 13" "$RC" 13; git -C "$R" checkout -q -- "$EV/$s"
done
printf 'new\n' > "$R/$EV/brand_new.jsonl"; lst "$EV/brand_new.jsonl"; run --paths-from "$T/cs.lst"; eq "a jsonl that is not a named store: 0" "$RC" 0
# 4 content-addressed blobs
c='content-of-the-blob\n'; h="$(printf "$c" | sha256sum | cut -d' ' -f1)"
w "$EV/blobs/$h" "$c"; lst "$EV/blobs/$h"; run --paths-from "$T/cs.lst"; eq "blob whose sha256 is its name: 0" "$RC" 0
w "$EV/blobs/$(printf 'f%.0s' $(seq 64))" "$c"; lst "$EV/blobs/$(printf 'f%.0s' $(seq 64))"; run --paths-from "$T/cs.lst"; eq "blob named by another hash: 13" "$RC" 13; has "reason blob_name" "$(cat "$T/out")" "blob_name"
w "$EV/blobs/notahash" "$c"; lst "$EV/blobs/notahash"; run --paths-from "$T/cs.lst"; eq "blob with a non-hash name: 13" "$RC" 13
# 5 dirty submodules: refused unless every dirty file has an exceptions row with the file's current sha256
mkrepo "$T/sub"; echo s > "$T/sub/f.txt"; commit_all "$T/sub" s
( cd "$R" && git submodule -q add "$T/sub" mods/sub && git commit -qm sub )
lst src/a.go; run --paths-from "$T/cs.lst"; eq "clean submodule: 0" "$RC" 0
echo dirty > "$R/mods/sub/f.txt"
run --paths-from "$T/cs.lst"; eq "dirty submodule: 13" "$RC" 13; has "reason dirty_submodule" "$(cat "$T/out")" "dirty_submodule"
wh="$(sha256sum "$R/mods/sub/f.txt" | cut -d' ' -f1)"
printf 'mods/sub\tdirty\tf.txt\t%s\t%s\ttest\n' "$wh" "$wh" > "$T/exc.tsv"; EXC="$T/exc.tsv" run --paths-from "$T/cs.lst"; eq "dirty submodule with its exceptions row: 0" "$RC" 0
printf 'mods/sub\tdirty\tf.txt\t%s\t%s\ttest\n' "$(printf 0%.0s $(seq 64))" "$wh" > "$T/exc.tsv"; EXC="$T/exc.tsv" run --paths-from "$T/cs.lst"; eq "row with a stale work-tree hash: 13" "$RC" 13
printf 'mods/sub\tdirty\tother.txt\t%s\t%s\ttest\n' "$wh" "$wh" > "$T/exc.tsv"; EXC="$T/exc.tsv" run --paths-from "$T/cs.lst"; eq "row for another file: 13" "$RC" 13
printf 'mods/sub\tmissing\tf.txt\t%s\t%s\ttest\n' "$wh" "$wh" > "$T/exc.tsv"; EXC="$T/exc.tsv" run --paths-from "$T/cs.lst"; eq "row of another kind: 13" "$RC" 13
echo more > "$R/mods/sub/untracked.txt"
printf 'mods/sub\tdirty\tf.txt\t%s\t%s\ttest\n' "$wh" "$wh" > "$T/exc.tsv"; EXC="$T/exc.tsv" run --paths-from "$T/cs.lst"; eq "an untracked file in the submodule is also dirty: 13" "$RC" 13
git -C "$R/mods/sub" checkout -q -- f.txt; rm -f "$R/mods/sub/untracked.txt"
run --paths-from "$T/cs.lst"; eq "submodule clean again: 0" "$RC" 0
# 6 refusals of the call itself (20): argument injection and misuse
for bad in '../x' '/abs' '-n' 'a/../b'; do lst "$bad"; run --paths-from "$T/cs.lst"; eq "unsafe path '$bad': 20" "$RC" 20; done
printf 'a\tb\n' > "$T/cs.lst"; run --paths-from "$T/cs.lst"; eq "tab in a path: 20" "$RC" 20
run --paths-from "$T/none.lst"; eq "unreadable list: 20" "$RC" 20
run; eq "no --paths-from: 20" "$RC" 20
( cd "$R" && "$H" --root "-x" --paths-from "$T/cs.lst" ) >/dev/null 2>&1; eq "root starting with a dash: 20" "$?" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; ( cd "$T/cwd" && "$H" --root -x --paths-from "$T/cs.lst" ) >/dev/null 2>&1; eq "an existing repository named -x is refused as an option-like value: 20" "$?" 20
( cd "$R" && "$H" --root "$T/plain_nonrepo" --paths-from "$T/cs.lst" ) >/dev/null 2>&1; eq "root that is no repository: 20" "$?" 20
run --bogus; eq "unknown option: 20" "$RC" 20
# 7 nothing written to the tree
eq "no tracked file changed by the check" "$(git -C "$R" diff --stat -- src | wc -l | tr -d ' ')" 0
# a git shim that FAILS (rc 128) when the argument list matches a glob, else runs the real git: a failing listing must never read as clean
mkshim() { mkdir -p "$T/fshim"; local rg; rg="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in $FAILPAT) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$rg" > "$T/fshim/git"; chmod +x "$T/fshim/git"; }
# 7 (WF5 F3) a failing git status / gitlink listing must not read as a clean submodule
mkshim; echo dirty > "$R/mods/sub/f.txt"; lst src/a.go; EXC=/dev/null run --paths-from "$T/cs.lst"; eq "control, real git: dirty submodule: 13" "$RC" 13
PATH="$T/fshim:$PATH" FAILPAT='*status --porcelain=v1*' EXC=/dev/null run --paths-from "$T/cs.lst"; eq "git status failing inside a submodule: 20, not 0 (F3)" "$RC" 20; has "named git_status_failed" "$(cat "$T/err")" git_status_failed
PATH="$T/fshim:$PATH" FAILPAT='*ls-files -s -z*' EXC=/dev/null run --paths-from "$T/cs.lst"; eq "gitlink listing failing: 20, not 0 (F3)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/err")" git_listing_failed
rm -f "$R/mods/sub/f.txt"

# 8 (WF6 W6-6) a submodule whose .git points at a MISSING git directory is broken, not "uninitialised": 20, never a silent skip
echo dirty > "$R/mods/sub/f.txt"; lst src/a.go; EXC=/dev/null run --paths-from "$T/cs.lst"; eq "control: intact submodule, dirty file: 13" "$RC" 13
mv "$R/.git/modules/mods/sub" "$T/gd.bak"; EXC=/dev/null run --paths-from "$T/cs.lst"
eq "submodule .git pointing at a missing git dir: 20 (W6-6)" "$RC" 20; has "named submodule_git_unreadable" "$(cat "$T/err")" submodule_git_unreadable; has "names the submodule" "$(cat "$T/err")" mods/sub
mv "$T/gd.bak" "$R/.git/modules/mods/sub"; EXC=/dev/null run --paths-from "$T/cs.lst"; eq "git dir restored: 13 again" "$RC" 13
mv "$R/mods/sub" "$T/sub.bak"; mkdir "$R/mods/sub"; EXC=/dev/null run --paths-from "$T/cs.lst"; eq "golden-false: an uninitialised submodule (empty directory, no .git) is no failure: 0" "$RC" 0
rmdir "$R/mods/sub"; mv "$T/sub.bak" "$R/mods/sub"; git -C "$R/mods/sub" checkout -q -- f.txt
# 9 (WF6 W6-7) "is the store in HEAD?" is decided only from a successful listing of HEAD's tree: an unreadable HEAD tree is 20, not "a new store"
printf 'l1\nREWRITTEN\n' > "$R/$EV/ledger.jsonl"; lst "$EV/ledger.jsonl"; run --paths-from "$T/cs.lst"; eq "control: rewritten ledger: 13" "$RC" 13
trh="$(git -C "$R" rev-parse "HEAD:$EV")"; mv "$R/.git/objects/${trh:0:2}/${trh:2}" "$T/tree.bak"
run --paths-from "$T/cs.lst"; eq "HEAD tree of the evidence directory unreadable (object missing): 20, not 0 (W6-7)" "$RC" 20; has "named git_tree_unreadable" "$(cat "$T/err")" git_tree_unreadable
mv "$T/tree.bak" "$R/.git/objects/${trh:0:2}/${trh:2}"; run --paths-from "$T/cs.lst"; eq "object restored: 13 again" "$RC" 13
PATH="$T/fshim:$PATH" FAILPAT='*ls-tree*' run --paths-from "$T/cs.lst"; eq "git ls-tree failing (shim): 20 (W6-7)" "$RC" 20
git -C "$R" checkout -q -- "$EV/ledger.jsonl"
mkrepo "$T/unborn"; mkdir -p "$T/unborn/$EV"; printf 'x\n' > "$T/unborn/$EV/ledger.jsonl"; lst "$EV/ledger.jsonl"
( cd "$T/unborn" && "$H" --root "$T/unborn" --ev "$EV" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; eq "golden-false: a repository without any commit (no HEAD) holds no store yet: 0" "$?" 0
# 10 (WF6 W6-8) a gitlink listing that fails only INSIDE a nested submodule refuses 20 (the depth-0 listing succeeds)
( cd "$R/mods/sub" && git init -q -b main "$T/inner" 2>/dev/null ); mkrepo "$T/inner"; echo i > "$T/inner/i"; commit_all "$T/inner" i
( cd "$R/mods/sub" && git config user.email t@t && git config user.name t && git submodule -q add "$T/inner" deep && git commit -qm deep )
lst src/a.go; echo dirty > "$R/mods/sub/deep/z.txt"; EXC=/dev/null run --paths-from "$T/cs.lst"; eq "control: dirty file two levels deep: 13" "$RC" 13
PATH="$T/fshim:$PATH" FAILPAT='*mods/sub ls-files -s -z*' EXC=/dev/null run --paths-from "$T/cs.lst"; eq "listing failing only inside mods/sub: 20 (W6-8)" "$RC" 20; has "named git_listing_failed with the nested path" "$(cat "$T/err")" "git_listing_failed: mods/sub"
rm -f "$R/mods/sub/deep/z.txt"
fin
