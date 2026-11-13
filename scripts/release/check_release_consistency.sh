#!/usr/bin/env bash
# check_release_consistency.sh - release-prep consistency checker (see README.md).
#   (1) version: every product-version field agrees with the expected version.
#   (2) changelog: every backticked repo path in the VERIFIED subsection of a
#       CHANGELOG section exists and is git-tracked; SHA256SUMS manifests verify.
# Exit: 0 all OK, 1 any FAIL, 2 usage/blind (never a silent pass).
# Always prints one machine-readable line:  VERDICT: PASS|PASS-WITH-WAIVERS|FAIL|BLIND ...
# (PASS-WITH-WAIVERS exits 0 like PASS but means declared evidence gaps were relied on.)
set -u

# F6: an inherited GIT_DIR / GIT_INDEX_FILE / GIT_WORK_TREE / GIT_CONFIG_* ... would redirect every git call
# below to another repository or index; the checker reads only the repository named by --repo.
for _gv in $(compgen -e | grep '^GIT_' || true); do unset "$_gv"; done
unset _gv

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SELF="$HERE/$(basename "${BASH_SOURCE[0]}")"

REPO=""
EXPECTED=""
PREV_TAG=""
SECTION=""
CHECK="all"
SELFTEST=0
WAIVERS=""
EV_PREFIXES=()

usage() {
  cat <<'U'
usage: check_release_consistency.sh [--repo DIR] [--version X.Y.Z] [--prev-tag TAG]
                                    [--section X.Y.Z] [--check version|changelog|all]
       check_release_consistency.sh --selftest
  --repo      repository root (default: the repo containing this script)
  --version   expected version (default: versions.json global major.minor.patch)
  --prev-tag  git tag; versionCode of each Android app must be an integer strictly
              greater than its value at that tag
  --section   CHANGELOG section version (default: --version, else versions.json global;
              BLIND when neither is available - the topmost section is never used silently)
  --waivers   file (repo-relative, git-tracked) declaring known evidence gaps; default
              docs/qa/EVIDENCE_WAIVERS.txt when present (see README)
  --evidence-prefix P
              repo-relative prefix whose VERIFIED paths MUST sit under a SHA256SUMS /
              SHA256SUMS.txt manifest (repeatable; default docs/qa/)
exit: 0 OK, 1 FAIL, 2 usage/blind.  Last line printed is "VERDICT: ..." (help: "VERDICT: HELP").
U
}

blind() { echo "BLIND: $*"; echo "VERDICT: BLIND $*"; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --repo) [ $# -ge 2 ] || blind "--repo needs a value"; REPO="$2"; shift 2 ;;
    --version) { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "--version needs a non-empty value"; EXPECTED="$2"; shift 2 ;;
    --prev-tag) [ $# -ge 2 ] || blind "--prev-tag needs a value"; PREV_TAG="$2"; shift 2 ;;
    --section) [ $# -ge 2 ] || blind "--section needs a value"; SECTION="$2"; shift 2 ;;
    --check) [ $# -ge 2 ] || blind "--check needs a value"; CHECK="$2"; shift 2 ;;
    --waivers) { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "--waivers needs a non-empty value"; WAIVERS="$2"; shift 2 ;;
    --evidence-prefix) { [ $# -ge 2 ] && [ -n "$2" ]; } || blind "--evidence-prefix needs a non-empty value"
      p="${2#./}"; p="${p%/}/"; EV_PREFIXES+=("$p"); shift 2 ;;
    --selftest) SELFTEST=1; shift ;;
    -h|--help) usage; echo "VERDICT: HELP"; exit 0 ;;
    *) usage >&2; blind "unknown option $1" ;;
  esac
done

# --version X selects the changelog section [X] (never a silent fall-back to the topmost section).
[ -n "$SECTION" ] || SECTION="$EXPECTED"
[ "${#EV_PREFIXES[@]}" -gt 0 ] || EV_PREFIXES=("docs/qa/")
case "$CHECK" in version|changelog|all) ;; *) blind "bad --check value '$CHECK'" ;; esac

# ---------------------------------------------------------------- selftest
if [ "$SELFTEST" = 1 ]; then
  # shellcheck source=scripts/release/release_fixture.sh
  . "$HERE/release_fixture.sh"
  base="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
  mkdir -p "$base" || blind "cannot create $base"
  st="$(mktemp -d "$base/selftest.XXXXXX")" || blind "mktemp failed"
  case "$st" in "$base"/selftest.*) ;; *) blind "odd selftest dir" ;; esac
  trap 'rm -rf "${st:?}"' EXIT
  make_fixture "$st/good" || blind "fixture build failed"
  mkdir -p "$st/bad_ver" "$st/bad_path"
  if ! { cp -a "$st/good/." "$st/bad_ver/" && cp -a "$st/good/." "$st/bad_path/"; }; then blind "fixture copy failed"; fi
  sed -i 's/^  "version": "2.4.0"/  "version": "2.3.0"/' "$st/bad_ver/catalog-web/package.json"
  # shellcheck disable=SC2016
  sed -i 's#`scripts/tool.sh`#`scripts/planted_fake_path.sh`#' "$st/bad_path/docs/CHANGELOG.md"
  # the planted edits are committed: the checker reads HEAD only, so an uncommitted edit would be BLIND, not FAIL
  for pl in bad_ver bad_path; do
    git -C "$st/$pl" -c user.name=selftest -c user.email=selftest@localhost -c commit.gpgsign=false commit -q -a -m planted >/dev/null 2>&1 \
      || blind "cannot commit the planted $pl edit"
  done
  fail=0
  rc=0; "$SELF" --repo "$st/good" >/dev/null 2>&1 || rc=$?
  [ "$rc" -eq 0 ] || { echo "selftest: known-good fixture did not PASS (rc=$rc)"; fail=1; }
  rc=0; "$SELF" --repo "$st/bad_ver" --check version >/dev/null 2>&1 || rc=$?
  [ "$rc" -eq 1 ] || { echo "selftest: planted wrong version was NOT detected (rc=$rc)"; fail=1; }
  rc=0; "$SELF" --repo "$st/bad_path" --check changelog >/dev/null 2>&1 || rc=$?
  [ "$rc" -eq 1 ] || { echo "selftest: planted fake path was NOT detected (rc=$rc)"; fail=1; }
  if [ "$fail" = 0 ]; then echo "VERDICT: PASS selftest good=PASS bad_version=FAIL bad_path=FAIL"; exit 0; fi
  echo "VERDICT: FAIL selftest"; exit 1
fi

# ---------------------------------------------------------------- setup
if [ -z "$REPO" ]; then
  REPO="$(git -C "$HERE" rev-parse --show-toplevel 2>/dev/null)" || blind "cannot locate repo (use --repo)"
fi
[ -d "$REPO" ] || blind "repo directory '$REPO' not found"
git -C "$REPO" rev-parse --git-dir >/dev/null 2>&1 || blind "'$REPO' is not a git repository"
REPO="$(cd "$REPO" && pwd)"
# CAT-REL-CHECK-2 round 5: HEAD is the single source for EVERY read. replace refs never apply (C8), and the
# contents of HEAD are written once into a throw-away directory (SNAP) from the raw blobs; manifests, version
# files, the changelog, the waiver file, gradle files, .gitignore decisions and the tracked-file sets are all
# read from SNAP or from `git ls-tree HEAD`, never from the working tree, the index, .git/info/* or any
# local/global configuration. The working tree is consulted only to say BLIND when it differs from HEAD.
export GIT_NO_REPLACE_OBJECTS=1
# CAT-REL-CHECK-3 (N2): every git call is bounded; a hung git is an ordinary git failure (BLIND), never a hang
RELCHK_GIT_TIMEOUT="${RELCHK_GIT_TIMEOUT:-120}"; export RELCHK_GIT_TIMEOUT
case "$RELCHK_GIT_TIMEOUT" in ''|*[!0-9]*|0) RELCHK_GIT_TIMEOUT=120 ;; esac
git() { command timeout --kill-after=5 "$RELCHK_GIT_TIMEOUT" git "$@"; }
HEADC="$(git -C "$REPO" rev-parse --verify -q 'HEAD^{commit}' 2>/dev/null)" || blind "'$REPO' has no HEAD commit"
SNAPBASE="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
mkdir -p "$SNAPBASE" || blind "cannot create $SNAPBASE"
if [ -n "${RELCHK_SNAP_DIR:-}" ]; then
  # test hook: a fixed, absolute, not yet existing snapshot path (lets a test place a sibling directory whose name
  # starts with the snapshot's name); never used by a normal run
  case "$RELCHK_SNAP_DIR" in /*) ;; *) blind "RELCHK_SNAP_DIR must be absolute" ;; esac
  [ ! -e "$RELCHK_SNAP_DIR" ] || blind "RELCHK_SNAP_DIR exists already"
  mkdir -p -- "$RELCHK_SNAP_DIR" || blind "cannot create RELCHK_SNAP_DIR"
  SNAP="$RELCHK_SNAP_DIR"
else
  SNAP="$(mktemp -d "$SNAPBASE/snap.XXXXXX")" || blind "cannot create a HEAD snapshot directory"
  case "$SNAP" in "$SNAPBASE"/snap.*) ;; *) blind "odd snapshot directory" ;; esac
fi
trap 'rm -rf -- "${SNAP:?}"' EXIT
# the snapshot writer resolves its interpreter from fixed system locations first, so a python3 shadowed on PATH
# (the helper-output tests do that on purpose) cannot fake the snapshot
SNAP_PY=""; for _p in /usr/bin/python3 /usr/local/bin/python3 /bin/python3; do [ -x "$_p" ] && { SNAP_PY="$_p"; break; }; done
[ -n "$SNAP_PY" ] || SNAP_PY="$(command -v python3 2>/dev/null)" || blind "python3 not found"
SNAP_ERR="$("$SNAP_PY" -I - "$REPO" "$HEADC" "$SNAP" 2>&1 >/dev/null <<'PY'
import os, signal, subprocess, sys
repo, commit, snap = sys.argv[1:4]
CHILD = []
def die(code, msg):
    sys.stderr.write("snapshot refused: %s\n" % msg); sys.stderr.flush()
    for ch in CHILD:   # a hung git must not outlive the writer (it would keep the caller's pipe open); pid > 1 only
        try:
            if ch.pid > 1:
                ch.kill()
        except Exception:
            pass
    os._exit(code)
def envint(name, dflt):
    try:
        v = int(os.environ.get(name, dflt))
        return v if v > 0 else dflt
    except ValueError:
        return dflt
MAXFILES = envint("RELCHK_MAX_FILES", 1000000)
MAXBYTES = envint("RELCHK_MAX_BYTES", 4 << 30)
GIT_TO = envint("RELCHK_GIT_TIMEOUT", 120)
signal.signal(signal.SIGALRM, lambda *_: die(7, "snapshot writer exceeded its time limit"))
signal.alarm(envint("RELCHK_SNAP_TIMEOUT", 900))
try:
    r = subprocess.run(["git", "-C", repo, "ls-tree", "-r", "-z", "--full-tree", commit], capture_output=True, timeout=GIT_TO)
except subprocess.TimeoutExpired:
    die(7, "git ls-tree timed out")
if r.returncode != 0:
    die(3, "git ls-tree failed")
# ---- 1. parse and validate the whole listing BEFORE anything is written (class: no content-derived write may
# resolve outside SNAP; duplicate / conflicting paths, special names and size excesses are refused up front)
ents = []
seen = set(); prefixes = set()
for rec in r.stdout.split(b"\0"):
    if not rec:
        continue
    try:
        meta, path = rec.split(b"\t", 1)
        mode, typ, sha = meta.split(b" ")
    except ValueError:
        die(4, "malformed ls-tree record")
    if (mode, typ) not in ((b"100644", b"blob"), (b"100755", b"blob"), (b"120000", b"blob"), (b"160000", b"commit")):
        die(4, "unsupported tree entry %r" % (mode,))
    parts = path.split(b"/")
    if any(p in (b"", b".", b"..") or p.lower() == b".git" or b"\n" in p for p in parts):
        die(4, "unsafe path component in %r" % (path,))
    if path in seen:
        die(4, "duplicate tree entry %r" % (path,))
    seen.add(path)
    for i in range(1, len(parts)):
        prefixes.add(b"/".join(parts[:i]))
    ents.append((mode, typ, sha, path, parts))
if len(ents) > MAXFILES:
    die(8, "more than %d tree entries" % MAXFILES)
clash = seen & prefixes
if clash:
    die(4, "a path is both an entry and a directory: %r" % (sorted(clash)[0],))
# ---- 2. extraction through directory file descriptors, no symlink is ever followed
base = os.fsencode(snap)
RFD = os.open(base, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
FLAGS = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
_cache = {"key": None, "fd": None}
def dirfd(parts, create=True):
    key = tuple(parts)
    if _cache["key"] == key:
        return _cache["fd"]
    if _cache["fd"] is not None:
        os.close(_cache["fd"]); _cache["key"] = None; _cache["fd"] = None
    fd = os.dup(RFD)
    for p in parts:
        if create:
            try:
                os.mkdir(p, 0o755, dir_fd=fd)
            except FileExistsError:
                pass
        nfd = os.open(p, FLAGS, dir_fd=fd)   # O_NOFOLLOW: a symlink here is an error, never followed
        os.close(fd); fd = nfd
    _cache["key"] = key; _cache["fd"] = fd
    return fd
cat = subprocess.Popen(["git", "-C", repo, "cat-file", "--batch"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
CHILD.append(cat)
total = 0
links = []
expect = {}
for mode, typ, sha, path, parts in ents:
    if typ == b"commit":
        pfd = dirfd(parts[:-1])
        os.mkdir(parts[-1], 0o755, dir_fd=pfd)
        expect[path] = "d"
        continue
    cat.stdin.write(sha + b"\n"); cat.stdin.flush()
    hdr = cat.stdout.readline().split()
    if len(hdr) != 3 or hdr[1] != b"blob":
        die(5, "unreadable blob")
    n = int(hdr[2])
    total += n
    if total > MAXBYTES:
        die(8, "more than %d bytes of content" % MAXBYTES)
    data = cat.stdout.read(n); cat.stdout.read(1)
    if len(data) != n:
        die(6, "short blob")
    if mode == b"120000":
        links.append((path, parts, data)); continue
    pfd = dirfd(parts[:-1])
    fd = os.open(parts[-1], os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o644, dir_fd=pfd)
    with os.fdopen(fd, "wb") as fh:
        fh.write(data)
        if mode == b"100755":
            os.fchmod(fh.fileno(), 0o755)
    expect[path] = "f"
cat.stdin.close(); cat.wait()
# symlinks last: every parent is a real directory made above (a path that is both a link and a directory was refused)
for path, parts, data in links:
    pfd = dirfd(parts[:-1])
    os.symlink(data, parts[-1], dir_fd=pfd)
    expect[path] = "l"
# ---- 3. verify: exactly the expected entries exist, none through a link, every parent resolves inside SNAP
real = os.path.realpath(base)
found = {}
for dp, dns, fns in os.walk(base, followlinks=False):
    rdp = os.path.realpath(dp)
    if rdp != real and not rdp.startswith(real + b"/"):
        die(9, "extracted directory resolves outside the snapshot")
    for nm in dns + fns:
        full = os.path.join(dp, nm)
        rel = full[len(base) + 1:]
        found[rel] = "l" if os.path.islink(full) else ("d" if os.path.isdir(full) else "f")
for rel, k in expect.items():
    if found.get(rel) != k:
        die(9, "extracted entry %r is not what the tree says" % (rel,))
dirs_expected = set(prefixes) | {p for p, k in expect.items() if k == "d"}
for rel, k in found.items():
    if k == "d":
        if rel not in dirs_expected:
            die(9, "unexpected directory %r" % (rel,))
    elif rel not in expect:
        die(9, "unexpected entry %r" % (rel,))
PY
)" || blind "cannot write the HEAD snapshot of $REPO: ${SNAP_ERR:-no detail}"
# the snapshot is a git repository only so that `check-ignore` can apply the COMMITTED .gitignore files;
# no .git/info/exclude, no global/system config, no core.excludesFile
GIT_SNAP=(env GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 timeout --kill-after=5 "$RELCHK_GIT_TIMEOUT" git -c core.excludesFile=/dev/null -C "$SNAP")
"${GIT_SNAP[@]}" init -q --template= . >/dev/null 2>&1 || blind "cannot initialise the HEAD snapshot repository"
V_OK=0; V_FAIL=0; C_PATHS=0; C_MISS=0; C_SKIP=0; S_OK=0; S_FAIL=0; S_UNV=0; BLINDS=""
C_NOMAN=0; U_FILES=0; W_APPLIED=0; W_BAD=0; W_UNUSED=0
declare -A WV=() WVUSED=()
SECTION_USED="-"
note_blind() { BLINDS="$BLINDS ${1}"; echo "BLIND: $1"; }

# ---------------------------------------------------------------- (1) versions
# head_check <label> <repo-relative path>...: a clone sees HEAD only, so every existing path must read the
# same in a clean clone as it does here. One generic mechanism (CAT-REL-CHECK-2, see
# docs/release_checker_path_inventory.md): the path is walked component by component exactly as the kernel
# would walk it, following symlinks hop by hop; EVERY component and EVERY hop must be in HEAD with the same
# type, a symlink hop must carry the same target as HEAD, the final file must hash to its HEAD blob, and the
# final file or directory must show no status difference and no skip-worktree / assume-unchanged flag.
# Anything that differs, or that cannot be proven (absolute hop, hop into a submodule, hop out of the
# repository, more than 40 hops, case-insensitive name collision, git failing), makes the run BLIND.
CHAIN_BAD=""
chain_bad() { CHAIN_BAD="$CHAIN_BAD [$1]"; }

# head_entry <repo-relative path>: sets E_MODE E_SHA from HEAD's tree for exactly that path
# (E_MODE empty when HEAD has no such entry). Returns 1 when git itself fails.
head_entry() {
  local ent="$1" out line meta nm
  E_MODE=""; E_SHA=""
  out="$(git -C "$REPO" -c core.quotepath=off ls-tree HEAD -- "$ent" 2>/dev/null)" || return 1
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    meta="${line%%$'\t'*}"; nm="${line#*$'\t'}"
    [ "$nm" = "$ent" ] || continue
    read -r E_MODE _ E_SHA <<< "$meta"
  done <<< "$out"
  return 0
}

# name_collides <parent> <name>: true when HEAD's tree holds a second entry in <parent> whose name equals
# <name> ignoring case (a clone on a case-insensitive filesystem would merge them).
name_collides() {
  local parent="$1" name="$2" want out e n=0
  want="${parent:+$parent/}$name"; want="${want,,}"
  if [ -n "$parent" ]; then out="$(git -C "$REPO" -c core.quotepath=off ls-tree --name-only HEAD -- "$parent/" 2>/dev/null)" || return 2
  else out="$(git -C "$REPO" -c core.quotepath=off ls-tree --name-only HEAD 2>/dev/null)" || return 2; fi
  while IFS= read -r e; do
    [ -n "$e" ] || continue
    [ "${e,,}" = "$want" ] && n=$((n+1))
  done <<< "$out"
  [ "$n" -gt 1 ]
}

# entry_clean <path>: status and skip-worktree / assume-unchanged check for one tracked entry (a symlink hop,
# the final file or the final directory). Appends to CHAIN_BAD; returns 1 only when git failed (BLIND noted).
entry_clean() {
  local cur="$1" out flagged
  if ! out="$(git -C "$REPO" -c core.quotepath=off status --porcelain=v1 --ignore-submodules=none --untracked-files=no -- ":(literal)$cur" 2>/dev/null)"; then
    note_blind "$(printf '%q' "$cur"): git status failed; cannot prove the files match HEAD"; return 1
  fi
  [ -z "$out" ] || chain_bad "$cur differs from HEAD: $(printf '%s\n' "$out" | head -3 | tr '\n' ';')"
  if ! flagged="$(git -C "$REPO" -c core.quotepath=off ls-files -v -- ":(literal)$cur" 2>/dev/null)"; then
    note_blind "$(printf '%q' "$cur"): git ls-files failed; cannot prove no file is hidden from git status"; return 1
  fi
  flagged="$(printf '%s\n' "$flagged" | grep -m1 '^[a-zS] ' || true)"
  [ -z "$flagged" ] || chain_bad "$cur holds a skip-worktree or assume-unchanged file ($flagged), which hides changes from git status"
  return 0
}

# chain_one <path>: appends to CHAIN_BAD; returns 1 only when git failed (a BLIND was already noted).
chain_one() {
  local spec="$1" cur="" c ent hops=0 ftype="tree" fsha="" tgt want out flagged rest r
  local -a pend=() parts=()
  [ -e "$REPO/$spec" ] || return 0
  case "$spec" in /*) chain_bad "$spec is not a repo-relative path"; return 0 ;; esac
  IFS='/' read -r -a pend <<< "$spec"
  while [ "${#pend[@]}" -gt 0 ]; do
    c="${pend[0]}"; pend=("${pend[@]:1}")
    case "$c" in
      ""|.) continue ;;
      ..) if [ -z "$cur" ]; then chain_bad "$spec leaves the repository"; return 0; fi
          if [[ "$cur" == */* ]]; then cur="${cur%/*}"; else cur=""; fi
          ftype="tree"; fsha=""; continue ;;
    esac
    ent="${cur:+$cur/}$c"
    head_entry "$ent" || { note_blind "git ls-tree failed for $(printf '%q' "$ent"); cannot prove it matches HEAD"; return 1; }
    if [ -z "$E_MODE" ]; then chain_bad "$ent is not in HEAD"; return 0; fi
    name_collides "$cur" "$c"; r=$?
    if [ "$r" -eq 2 ]; then note_blind "git ls-tree failed for $(printf '%q' "${cur:-.}"); cannot prove it matches HEAD"; return 1; fi
    if [ "$r" -eq 0 ]; then chain_bad "$ent collides with another HEAD entry ignoring case"; return 0; fi
    case "$E_MODE" in
      120000)
        if [ ! -L "$REPO/$ent" ]; then chain_bad "$ent is a symlink in HEAD but not in the working tree"; return 0; fi
        # F2: compare the RAW bytes. $(...) strips trailing newlines and `read` keeps one line, so a
        # sentinel byte is appended and removed again (readlink adds exactly one newline of its own).
        tgt="$(readlink -- "$REPO/$ent"; printf .)"; tgt="${tgt%.}"; tgt="${tgt%$'\n'}"
        want="$(git -C "$REPO" cat-file blob "$E_SHA" 2>/dev/null; printf .)" \
          || { note_blind "git cat-file failed for $(printf '%q' "$ent")"; return 1; }
        want="${want%.}"
        # HEAD's own target decides what is supportable, so these checks come before the comparison
        case "$want" in
          "") chain_bad "$ent is an empty symlink in HEAD"; return 0 ;;
          /*) chain_bad "$ent is an absolute symlink ($want); a clone elsewhere cannot prove it"; return 0 ;;
        esac
        if [ "$tgt" != "$want" ]; then chain_bad "$ent link target differs from HEAD (working tree $(printf '%q' "$tgt"), HEAD $(printf '%q' "$want"))"; return 0; fi
        if [[ "$tgt" == *[[:cntrl:]]* ]]; then chain_bad "$ent link target contains a control character or newline ($(printf '%q' "$tgt"))"; return 0; fi
        # F2: a hop is a tracked entry too: its own status and skip-worktree / assume-unchanged flags
        entry_clean "$ent" || return 1
        hops=$((hops+1))
        if [ "$hops" -gt 40 ]; then chain_bad "$spec needs more than 40 symlink hops"; return 0; fi
        IFS='/' read -r -a parts <<< "$tgt"
        pend=("${parts[@]}" "${pend[@]}") ;;
      040000)
        if [ -L "$REPO/$ent" ] || [ ! -d "$REPO/$ent" ]; then chain_bad "$ent is a directory in HEAD but not in the working tree"; return 0; fi
        cur="$ent"; ftype="tree"; fsha="" ;;
      160000)
        if [ -L "$REPO/$ent" ] || [ ! -d "$REPO/$ent" ]; then chain_bad "$ent is a submodule in HEAD but not a directory in the working tree"; return 0; fi
        rest=""; for r in "${pend[@]}"; do case "$r" in ""|.) ;; *) rest=1 ;; esac; done
        if [ -n "$rest" ]; then chain_bad "$spec continues inside the submodule $ent, which a clone does not provide"; return 0; fi
        cur="$ent"; ftype="commit"; fsha="" ;;
      100644|100755)
        rest=""; for r in "${pend[@]}"; do case "$r" in ""|.) ;; *) rest=1 ;; esac; done
        if [ -n "$rest" ]; then chain_bad "$spec continues below the file $ent"; return 0; fi
        if [ -L "$REPO/$ent" ] || [ ! -f "$REPO/$ent" ]; then chain_bad "$ent is a file in HEAD but not in the working tree"; return 0; fi
        cur="$ent"; ftype="blob"; fsha="$E_SHA" ;;
      *) chain_bad "$ent has the unsupported HEAD mode $E_MODE"; return 0 ;;
    esac
  done
  [ -n "$cur" ] || return 0
  if [ "$ftype" = blob ]; then
    out="$(git -C "$REPO" hash-object --no-filters -- "$cur" 2>/dev/null)" || { note_blind "git hash-object failed for $(printf '%q' "$cur")"; return 1; }
    # F3: RAW bytes against the blob. A local clean filter, .git/info/attributes, core.attributesFile,
    # core.autocrlf / core.eol or an untracked .gitattributes can make `git status` clean while the bytes a
    # clone checks out differ; any raw difference is therefore unprovable here (BLIND), never a pass.
    [ "$out" = "$fsha" ] || chain_bad "$cur raw bytes differ from the HEAD blob (content, or a local filter / eol / attribute conversion that a clone would not apply; to resolve, commit the bytes exactly as they are in the working tree or remove the eol/filter attribute for this path)"
  fi
  entry_clean "$cur" || return 1
  return 0
}

head_check() {
  local label="$1" p; shift
  CHAIN_BAD=""
  for p in "$@"; do chain_one "$p" || return 0; done
  [ -z "$CHAIN_BAD" ] || note_blind "$label are not committed as read (a clone would differ):$CHAIN_BAD"
  return 0
}

run_versions() {
  echo "== version consistency"
  local kind label detail vout vrc
  vout="$(python3 -I - "$REPO" "$EXPECTED" "$PREV_TAG" "$SNAP" <<'PY'
import json, os, re, subprocess, sys, tomllib

repo, expected, prevtag, snap = sys.argv[1:5]
GIT_TO = int(os.environ.get("RELCHK_GIT_TIMEOUT", "120") or "120")   # repo: git queries only; snap: the HEAD snapshot every file read uses

def emit(kind, label, detail=""):
    print(f"{kind}\t{label}\t{detail}")

READ = []
def read(path):
    READ.append(path)
    with open(os.path.join(snap, path), encoding="utf-8") as fh:
        return fh.read()

# expected version
vj_ok = True
derived = None
try:
    g = json.loads(read("versions.json"))["global"]
    derived = f"{g['major']}.{g['minor']}.{g['patch']}"
except Exception as exc:  # noqa: BLE001
    vj_ok = False
    vj_err = f"{type(exc).__name__}: {exc}"
if not expected:
    if not vj_ok:
        emit("BLIND", "versions.json", f"unreadable and no --version given ({vj_err})")
        sys.exit(0)
    expected = derived

def check(label, getter):
    try:
        actual = getter()
    except Exception as exc:  # noqa: BLE001
        emit("FAIL", label, f"cannot read: {type(exc).__name__}: {exc}")
        return
    if actual == expected:
        emit("OK", label, actual)
    else:
        emit("FAIL", label, f"found {actual!r} expected {expected!r}")

NPM = ("catalog-web", "catalogizer-desktop", "installer-wizard")
TAURI = ("catalogizer-desktop", "installer-wizard")
CARGO = ("catalogizer-desktop", "installer-wizard")
APPS = ("catalogizer-android", "catalogizer-androidtv")
# versions.json + openapi + 3 rows per npm component + tauri + cargo + (name, code) per app
emit("PLAN", str(2 + 3 * len(NPM) + len(TAURI) + len(CARGO) + 2 * len(APPS)))

# versions.json global
check("versions.json global", lambda: (_ for _ in ()).throw(ValueError(vj_err)) if not vj_ok else derived)

# openapi info.version (indent-2 key directly inside top-level info:)
def openapi():
    lines = read("docs/api/openapi.yaml").splitlines()
    inside = False
    found = []
    for ln in lines:
        if re.match(r"^info:\s*(#.*)?$", ln):
            inside = True
            continue
        if inside:
            if ln and not ln.startswith((" ", "\t", "#")):
                break
            m = re.match(r"^  version:\s*[\"']?([^\"'#\s]+)[\"']?\s*(#.*)?$", ln)
            if m:
                found.append(m.group(1))
    if len(found) != 1:
        raise ValueError(f"expected exactly one info.version, found {len(found)}")
    return found[0]
check("docs/api/openapi.yaml info.version", openapi)

# npm packages and lockfiles
for comp in NPM:
    check(f"{comp}/package.json version", lambda c=comp: json.loads(read(f"{c}/package.json"))["version"])
    check(f"{comp}/package-lock.json version", lambda c=comp: json.loads(read(f"{c}/package-lock.json"))["version"])
    check(f"{comp}/package-lock.json packages[\"\"].version",
          lambda c=comp: json.loads(read(f"{c}/package-lock.json"))["packages"][""]["version"])

# tauri.conf.json (v2 top-level version; v1 package.version)
def tauri(path):
    d = json.loads(read(path))
    if "version" in d:
        return d["version"]
    return d["package"]["version"]
for comp in TAURI:
    check(f"{comp}/src-tauri/tauri.conf.json version", lambda c=comp: tauri(f"{c}/src-tauri/tauri.conf.json"))

# Cargo.toml [package] version
def cargo(path):
    v = tomllib.loads(read(path))["package"]["version"]
    if not isinstance(v, str):
        raise ValueError(f"non-literal package.version {v!r}")
    return v
for comp in CARGO:
    check(f"{comp}/src-tauri/Cargo.toml [package] version", lambda c=comp: cargo(f"{c}/src-tauri/Cargo.toml"))

# Gradle versionName / versionCode
def gradle_field(text, name):
    pat = re.compile(r"^\s*" + name + r"(?![A-Za-z0-9_])\s*(?:=\s*)?(\"[^\"]*\"|[^\s/;]+)\s*;?\s*(?:/\*(?:[^*]|\*(?!/))*\*/\s*)*(?://.*)?$")
    hits = []
    for ln in text.splitlines():
        if ln.lstrip().startswith("//"):
            continue
        m = pat.match(ln)
        if m:
            hits.append(m.group(1).strip('"'))
    if len(hits) != 1:
        raise ValueError(f"expected exactly one {name}, found {len(hits)}")
    return hits[0]

tag_ok = True
if prevtag:
    r = subprocess.run(["git", "-C", repo, "rev-parse", "-q", "--verify", prevtag + "^{commit}"],
                       capture_output=True, text=True, timeout=GIT_TO)
    if r.returncode != 0:
        tag_ok = False
        emit("FAIL", "--prev-tag", f"tag {prevtag!r} not found")

for app in APPS:
    path = f"{app}/app/build.gradle.kts"
    both = (os.path.exists(os.path.join(snap, path))
            and os.path.exists(os.path.join(snap, f"{app}/app/build.gradle")))
    if both:
        emit("FAIL", f"{app}/app", "both build.gradle.kts and build.gradle exist; ambiguous")
    if not os.path.exists(os.path.join(snap, path)):
        path = f"{app}/app/build.gradle"
    check(f"{path} versionName", lambda p=path: gradle_field(read(p), "versionName"))
    label = f"{path} versionCode"
    try:
        code = gradle_field(read(path), "versionCode")
    except Exception as exc:  # noqa: BLE001
        emit("FAIL", label, f"cannot read: {exc}")
        continue
    if not re.fullmatch(r"[0-9]+", code):
        emit("FAIL", label, f"not an integer: {code!r}")
        continue
    cur = int(code)
    if not (prevtag and tag_ok):
        emit("INFO", label, f"{cur} (no --prev-tag, monotonicity not checked)" if not prevtag else str(cur))
        continue
    r = subprocess.run(["git", "-C", repo, "show", f"{prevtag}:{path}"], capture_output=True, text=True, timeout=GIT_TO)
    if r.returncode != 0:
        emit("OK", label, f"{cur} (file absent at {prevtag}: new)")
        continue
    try:
        prev_s = gradle_field(r.stdout, "versionCode")
    except Exception as exc:  # noqa: BLE001
        emit("FAIL", label, f"cannot read versionCode at {prevtag}: {exc}")
        continue
    if not re.fullmatch(r"[0-9]+", prev_s):
        emit("FAIL", label, f"previous value not an integer: {prev_s!r}")
    elif cur > int(prev_s):
        emit("OK", label, f"{cur} > {prev_s} at {prevtag}")
    else:
        emit("FAIL", label, f"{cur} is not > {prev_s} at {prevtag}")
for _p in sorted(set(READ)):
    print(f"READ\t{_p}")
PY
)"
  vrc=$?
  local plan="" rows=0 nplan=0 key base dup="" miss="" foreign="" expected_n=19
  local -A seen=() kinds=()
  local -a vread=()
  # N1: the expected field rows are fixed here, independent of the helper's own PLAN.
  local -a want=("versions.json global" "docs/api/openapi.yaml info.version")
  local c a
  for c in catalog-web catalogizer-desktop installer-wizard; do
    want+=("$c/package.json version" "$c/package-lock.json version" "$c/package-lock.json packages[\"\"].version")
  done
  for c in catalogizer-desktop installer-wizard; do
    want+=("$c/src-tauri/tauri.conf.json version" "$c/src-tauri/Cargo.toml [package] version")
  done
  for a in catalogizer-android catalogizer-androidtv; do
    want+=("$a/app/build.gradle versionName" "$a/app/build.gradle versionCode")
  done
  while IFS=$'\t' read -r kind label detail; do
    case "$kind" in
      PLAN) nplan=$((nplan+1)); plan="$label" ;;
      OK|FAIL|INFO)
        rows=$((rows+1))
        key="${label/build.gradle.kts/build.gradle}"
        seen["$key"]=$(( ${seen["$key"]:-0} + 1 ))
        kinds["$key"]="$kind"
        if [ "$kind" = OK ]; then V_OK=$((V_OK+1)); echo "OK    $label: $detail"
        elif [ "$kind" = FAIL ]; then V_FAIL=$((V_FAIL+1)); echo "FAIL  $label: $detail"
        else echo "INFO  $label: $detail"; fi ;;
      BLIND) note_blind "$label $detail" ;;
      READ) vread+=("$label") ;;
    esac
  done <<< "$vout"
  head_check "version files" "${vread[@]+"${vread[@]}"}"
  # B1: a crashed or silent helper must never read as PASS.
  if [ "$vrc" -ne 0 ]; then note_blind "version helper exited $vrc (rows seen: $rows)"; return; fi
  if [ "$nplan" -gt 1 ]; then note_blind "version helper emitted more than one PLAN line"; return; fi
  if ! [[ "$plan" =~ ^[0-9]+$ ]]; then
    case "$BLINDS" in *versions.json*) ;; *) note_blind "version helper emitted no PLAN line" ;; esac
    return
  fi
  if [ "$plan" -ne "$expected_n" ]; then note_blind "version helper plan $plan, expected $expected_n"; return; fi
  if [ "$rows" -lt "$plan" ]; then note_blind "version helper produced $rows of $plan planned field rows"; return; fi
  for key in "${want[@]}"; do
    base="${seen["$key"]:-0}"
    if [ "$base" -eq 0 ]; then miss="$miss [$key]"
    elif [ "$base" -gt 1 ]; then dup="$dup [$key]"; fi
    case "$key" in
      *" versionCode") ;;
      *) [ "${kinds["$key"]:-}" = INFO ] && foreign="$foreign [$key INFO]" ;;
    esac
  done
  for key in "${!seen[@]}"; do
    case "$key" in
      "--prev-tag"|*"/app") [ "${kinds["$key"]}" = FAIL ] || foreign="$foreign [$key]" ;;
      *) c=0; for a in "${want[@]}"; do [ "$a" = "$key" ] && c=1; done
         [ "$c" -eq 1 ] || foreign="$foreign [$key]" ;;
    esac
  done
  # F4: INFO on a versionCode row is only legitimate without --prev-tag, or when the
  # --prev-tag row itself FAILed (unknown tag); otherwise it is a forged downgrade.
  if [ -n "$PREV_TAG" ] && [ "${kinds["--prev-tag"]:-}" != FAIL ]; then
    for key in "${want[@]}"; do
      [ "${kinds["$key"]:-}" = INFO ] && foreign="$foreign [$key INFO with --prev-tag]"
    done
  fi
  if [ -n "$miss" ]; then note_blind "version helper rows missing:$miss"; fi
  if [ -n "$dup" ]; then note_blind "version helper duplicate rows:$dup"; fi
  if [ -n "$foreign" ]; then note_blind "version helper unexpected rows:$foreign"; fi
}

# ---------------------------------------------------------------- (2) changelog
tracked() { # tracked <path>: true when HEAD holds the path (file, symlink, gitlink or directory); never the index
  head_entry "$1" && [ -n "$E_MODE" ]
}

run_changelog() {
  echo "== changelog evidence paths"
  local cl="$SNAP/docs/CHANGELOG.md"
  [ -f "$cl" ] || { note_blind "docs/CHANGELOG.md not found"; return; }

  # 11.4.273 control needles: the existence instrument must see a known-present tracked
  # path and must not see a fabricated one.
  local pos neg
  # tracked-instrument needle: first tracked path with a plain name (no quoting issues), judged
  # by git only; existence-instrument needle: the changelog itself, which was just found.
  pos="$(git -C "$REPO" ls-tree -r --name-only HEAD | grep -m1 -E '^[A-Za-z0-9._/-]+$')"
  neg="__no_such_path_$$__/none"
  if [ -z "$pos" ] || ! tracked "$pos" || [ ! -e "$cl" ] || [ -e "$SNAP/$neg" ] || tracked "$neg"; then
    note_blind "control needle failed: existence/tracked instrument cannot be trusted"; return
  fi

  local out kind a b
  out="$(python3 -I - "$REPO" "$SECTION" "$SNAP" <<'PY'
import json, os, re, subprocess, sys
repo, want, snap = sys.argv[1:4]
GIT_TO = int(os.environ.get("RELCHK_GIT_TIMEOUT", "120") or "120")
if not want:
    # F6: no --version/--section: the section is derived from versions.json (global), never the topmost heading
    try:
        with open(os.path.join(snap, "versions.json"), encoding="utf-8") as vf:
            g = json.load(vf)["global"]
        want = f"{g['major']}.{g['minor']}.{g['patch']}"
    except Exception as exc:  # noqa: BLE001
        print(f"ERR\tno --version/--section given and versions.json global is unreadable ({type(exc).__name__}); refusing to use the topmost changelog section")
        sys.exit(0)
with open(os.path.join(snap, "docs/CHANGELOG.md"), encoding="utf-8") as fh:
    lines = fh.read().splitlines()
head = re.compile(r"^## \[(\d[^\]]*)\]")
# lines inside fenced code blocks are never headings (fence lines included)
fenced = []
infence = False
for ln in lines:
    if re.match(r"^\s{0,3}(```|~~~)", ln):
        infence = not infence
        fenced.append(True)
    else:
        fenced.append(infence)
start = None
for i, ln in enumerate(lines):
    m = None if fenced[i] else head.match(ln)
    if m and (not want or m.group(1) == want):
        start, name = i, m.group(1)
        break
if start is None:
    print("ERR\tsection not found" + (f": {want}" if want else " (no numbered section)"))
    sys.exit(0)
if sum(1 for k, ln in enumerate(lines) if not fenced[k] and (hm := head.match(ln)) and hm.group(1) == name) > 1:
    print(f"ERR\tduplicate section heading [{name}] in CHANGELOG")
    sys.exit(0)
end = len(lines)
for j in range(start + 1, len(lines)):
    if not fenced[j] and lines[j].startswith("## "):
        end = j
        break
body = lines[start + 1:end]
vlines = []
nsub = 0
inv = False
for k, ln in enumerate(body):
    isf = fenced[start + 1 + k]
    if not isf and re.match(r"^###\s+VERIFIED\b", ln):
        inv = True
        nsub += 1
        continue
    if not isf and re.match(r"^#{1,3} ", ln):
        inv = False
        continue
    if inv:
        vlines.append(ln)
if nsub == 0:
    print(f"ERR\tno VERIFIED subsection in section {name}")
    sys.exit(0)
print(f"SECTION\t{name}")
tops = set()
tfiles = set()
tdirs = set()
r = subprocess.run(["git", "-C", repo, "ls-tree", "-r", "-z", "--name-only", "--full-tree", "HEAD"], capture_output=True, text=True, timeout=GIT_TO)
for p in r.stdout.split("\0"):
    if p:
        tfiles.add(p)
    if "/" in p:
        tops.add(p.split("/", 1)[0])
        parts = p.split("/")
        for i in range(1, len(parts)):
            tdirs.add("/".join(parts[:i]))
known = re.compile(r"^(docs|scripts|challenges|tests?|tools|specs|catalog[a-z0-9_-]*|installer-wizard|submodules|constitution|Website|config|deployment|\.github)$")
seen = set()
nskip = 0
for ln in vlines:
    for tok in re.findall(r"`([^`\n]+)`", ln):
        t = tok.strip()
        if "/" not in t:
            continue
        if re.search(r"[\x00-\x1f\x7f]", tok):
            print(f"CTRL\t{tok}")
            continue
        if re.search(r"[*?<>${}|\\=(),;\"']", t) or re.match(r"^[a-z][a-z0-9+.-]*://", t) \
                or t.startswith(("/", "~", "-")):
            print(f"SKIP\t{tok.strip()}")
            nskip += 1
            continue
        t = re.sub(r":\d+(-\d+)?$", "", t.rstrip(".,;:"))
        if t.startswith("./"):
            t = t[2:]
        if re.search(r"\s", t) and t not in tfiles and t not in tdirs:
            # m-2: a token with whitespace is a path only when it is exactly a tracked file or directory;
            # otherwise it is indistinguishable from a command line and stays a skipped token.
            print(f"SKIP\t{t}")
            nskip += 1
            continue
        top = t.split("/", 1)[0]
        if top in tops or known.match(top):
            if t not in seen:
                seen.add(t)
                print(f"PATH\t{t}")
        else:
            print(f"SKIP\t{t}")
            nskip += 1
print(f"END\t{len(seen)}\t{nskip}")
PY
)" || { note_blind "changelog extraction crashed"; return; }

  local paths=() skips=() nsec=0 nend=0 endp=-1 ends=-1 lastkind=""
  while IFS=$'\t' read -r kind a b; do
    case "$kind" in
      ERR) note_blind "$a"; return ;;
      SECTION) nsec=$((nsec+1)); SECTION_USED="$a" ;;
      PATH) paths+=("$a") ;;
      SKIP) skips+=("$a") ;;
      CTRL) note_blind "VERIFIED token contains a control character: $(printf '%q' "$a")" ;;
      END) nend=$((nend+1)); endp="$a"; ends="$b" ;;
    esac
    [ -z "$kind" ] || lastkind="$kind"
  done <<< "$out"
  # N1: the helper must prove it ran to completion (one SECTION, one trailing END with matching counts).
  if [ "$nsec" -ne 1 ] || [ "$nend" -ne 1 ] || [ "$lastkind" != END ] \
     || [ "$endp" != "${#paths[@]}" ] || [ "$ends" != "${#skips[@]}" ]; then
    SECTION_USED="-"; note_blind "changelog helper output incomplete (sections=$nsec ends=$nend paths=${#paths[@]} skips=${#skips[@]})"; return
  fi
  C_SKIP=${#skips[@]}
  if [ "${#paths[@]}" -eq 0 ]; then
    note_blind "zero repo paths extracted from VERIFIED subsection of section $SECTION_USED"; return
  fi
  echo "section $SECTION_USED: ${#paths[@]} path(s) in VERIFIED; skipped non-repo-looking: ${skips[*]:-none}"

  head_check "docs/CHANGELOG.md" "docs/CHANGELOG.md"
  # C7: the derived section comes from versions.json, so the working-tree copy must read like HEAD's as well
  [ -n "$SECTION" ] || head_check "versions.json" "versions.json"
  local p q d rp mf ev evrel cited_ok
  local REPO_REAL; REPO_REAL="$(cd "$SNAP" && pwd -P)"   # the HEAD snapshot: every symlink below resolves inside HEAD
  declare -A seen_dir=()
  local -a queue=()
  load_waivers || return
  for p in "${paths[@]}"; do
    C_PATHS=$((C_PATHS+1))
    if [[ "$p" == *[[:cntrl:]]* ]]; then
      note_blind "VERIFIED path contains a control character: $(printf '%q' "$p")"; continue
    fi
    q="${p%/}"
    # N3: a tracked symlink must not lead outside the repository.
    rp="$(readlink -f -- "$SNAP/$q" 2>/dev/null || true)"
    if [ -e "$SNAP/$q" ] && { [ -z "$rp" ] || { [ "$rp" != "$REPO_REAL" ] && [[ "$rp" != "$REPO_REAL"/* ]]; }; }; then
      C_MISS=$((C_MISS+1)); echo "MISS  $p (resolves outside the repository)"; continue
    fi
    # RC-3: decide on the REAL in-repo location (// and /./ collapsed, symlinks resolved), so a
    # spelling trick or an alias cannot dodge the evidence prefix or the manifest search.
    ev="$(readlink -m -- "$SNAP/$q" 2>/dev/null || true)"
    if [ -n "$ev" ] && { [ "$ev" = "$REPO_REAL" ] || [[ "$ev" == "$REPO_REAL"/* ]]; }; then
      evrel="${ev#"$REPO_REAL"}"; evrel="${evrel#/}"; [ -n "$evrel" ] || evrel="."
    else
      evrel="$q"
    fi
    cited_ok=0
    if [ ! -e "$SNAP/$q" ]; then
      # CAT-REL-CHECK-3 (M1): existence is decided by HEAD's snapshot, as in a clean clone (a dangling HEAD symlink
      # is missing there too); the working tree is consulted only afterwards, by head_check, to say BLIND
      C_MISS=$((C_MISS+1)); echo "MISS  $p (not in HEAD)"
    elif ! tracked "$q" && ! tracked "$evrel"; then
      C_MISS=$((C_MISS+1)); echo "MISS  $p (exists but not git-tracked)"
    else
      echo "OK    $p"; cited_ok=1
    fi
    # CAT-REL-CHECK-2: the cited spelling is walked component by component and hop by hop (head_check).
    # F1: no exemption for a path that resolves to the repository root; the cited spelling is always walked.
    [ "$cited_ok" = 0 ] || head_check "cited path $p" "$q"
    # nearest tracked manifest (SHA256SUMS, SHA256SUMS.txt, MANIFEST*.sha256) at or above the path
    # (never the repo root itself)
    if [ -d "$SNAP/$q" ]; then d="$evrel"; else d="$(dirname "$evrel")"; fi
    local found=0 guard=0
    while [ -n "$d" ] && [ "$d" != "." ] && [ "$d" != "/" ] && [ "$guard" -lt 500 ]; do
      guard=$((guard+1))
      if [ -n "$(manifests_in "$d")" ]; then seen_dir["$d"]=1; found=1; break; fi
      d="$(dirname "$d")"
    done
    # RC-3: evidence without any tracked integrity manifest is an unsupported VERIFIED claim, never a silent pass.
    if [ "$found" = 0 ] && is_evidence "$evrel"; then
      if ! waived "no-manifest|$evrel" "$p (no manifest)"; then
        C_NOMAN=$((C_NOMAN+1)); echo "NOMANIFEST  $p (evidence path with no tracked SHA256SUMS, SHA256SUMS.txt or MANIFEST*.sha256 at or above it)"
      fi
    fi
  done

  local -A done_dir=()
  queue=("${!seen_dir[@]}")
  while [ "${#queue[@]}" -gt 0 ]; do
    d="${queue[0]}"; queue=("${queue[@]:1}")
    [ -z "${done_dir[$d]:-}" ] || continue
    done_dir["$d"]=1
    check_manifest_dir "$d"
  done
  report_unused_waivers
}

# manifests_in <dir>: names of the git-tracked manifests directly in <dir>, one per line.
# A manifest that is untracked or gitignored does not exist for a clone, so it does not count.
manifests_in() {
  local d="$1" f n
  for f in "$SNAP/$d"/SHA256SUMS "$SNAP/$d"/SHA256SUMS.txt "$SNAP/$d"/MANIFEST*.sha256; do
    [ -f "$f" ] || continue
    n="${f##*/}"
    tracked "$d/$n" && printf '%s\n' "$n"
  done
  return 0
}

# is_evidence <repo-relative path>: true when the path lies under one of the evidence prefixes.
is_evidence() {
  local pre
  for pre in "${EV_PREFIXES[@]}"; do
    [[ "$1/" == "$pre"* ]] && return 0
  done
  return 1
}

trim() { local v="$1"; v="${v#"${v%%[![:space:]]*}"}"; v="${v%"${v##*[![:space:]]}"}"; printf '%s' "$v"; }

# load_waivers: parse the tracked waiver file (RC-3 known-gap declarations). Format, one per line:
#   no-manifest|<evidence path>|<item>|<expires>|<reason>
#   unlisted|<manifest dir>|<file relative to it>|<item>|<expires>|<reason>
#   bad-entry|<manifest dir>|<manifest entry>|<item>|<expires>|<reason>   (missing entry only; a mismatch is never waivable)
# '#' lines and blank lines are ignored. Every applied waiver is printed (WAIVED) and counted.
load_waivers() {
  local f="$WAIVERS" rel explicit=0 n=0 line k reason key item exp today nf i
  local -a F
  if [ -n "$f" ]; then explicit=1; else f="docs/qa/EVIDENCE_WAIVERS.txt"; fi
  rel="$f"
  if [ ! -f "$SNAP/$rel" ]; then
    # not in HEAD: a clone has no such waiver file
    if [ -e "$REPO/$rel" ] || [ -L "$REPO/$rel" ]; then note_blind "waiver file $rel exists but is not git-tracked (not in HEAD)"; return 1; fi
    [ "$explicit" = 0 ] || { note_blind "waiver file $rel not found"; return 1; }
    return 0
  fi
  # the waivers are read from HEAD; a working-tree copy that reads differently (edit, deletion, assume-unchanged /
  # skip-worktree flag, local clean filter, stat-cache trick) makes the run BLIND instead of passing on either copy
  head_check "waiver file $rel" "$rel"
  today="${RELCHK_TODAY:-$(date +%F)}"
  while IFS= read -r line || [ -n "$line" ]; do
    n=$((n+1))
    line="${line%$'\r'}"
    [ -n "$(trim "$line")" ] || continue
    [[ "$(trim "$line")" != \#* ]] || continue
    IFS='|' read -r -a F <<< "$line"
    for i in "${!F[@]}"; do F[i]="$(trim "${F[i]}")"; done
    k="${F[0]}"
    case "$k" in
      no-manifest) nf=2 ;;   # kind|path|item|expires|reason
      unlisted|bad-entry) nf=3 ;;   # kind|dir|file|item|expires|reason
      *) W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n unknown kind '$k'"; continue ;;
    esac
    item="${F[nf]:-}"; exp="${F[nf+1]:-}"
    reason=""
    for ((i=nf+2; i<${#F[@]}; i++)); do reason="${reason:+$reason|}${F[i]}"; done
    reason="$(trim "$reason")"
    if [ "$k" = no-manifest ]; then key="no-manifest|${F[1]:-}"; else key="$k|${F[1]:-}|${F[2]:-}"; fi
    if [ -z "${F[1]:-}" ] || { [ "$k" != no-manifest ] && [ -z "${F[2]:-}" ]; }; then
      W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n empty path field"; continue
    fi
    if [ -z "$reason" ]; then W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing reason"; continue; fi
    if ! [[ "$item" =~ ^[A-Za-z][A-Za-z0-9]*(-[A-Za-z0-9]+)+$ ]]; then
      W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing or malformed tracked item id '$item'"; continue
    fi
    if ! [[ "$exp" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || [ "$(date -d "$exp" +%F 2>/dev/null)" != "$exp" ]; then
      W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing or invalid expires date '$exp' (want YYYY-MM-DD)"; continue
    fi
    if [[ "$exp" < "$today" ]]; then
      W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n expired on $exp (today $today)"; continue
    fi
    if [ -n "${WV[$key]+x}" ]; then W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n duplicate waiver"; continue; fi
    WV["$key"]="$reason [item $item, expires $exp]"
  done < "$SNAP/$rel"
  return 0
}

# waived <key> <label>: true (and prints WAIVED with the reason) when a declared waiver matches.
waived() {
  [ -n "${WV[$1]+x}" ] || return 1
  WVUSED["$1"]=1; W_APPLIED=$((W_APPLIED+1))
  echo "WAIVED  $2: ${WV[$1]}"
  return 0
}

report_unused_waivers() {
  local key
  for key in "${!WV[@]}"; do
    if [ -z "${WVUSED[$key]:-}" ]; then W_UNUSED=$((W_UNUSED+1)); echo "UNUSED-WAIVER  $key: ${WV[$key]}"; fi
  done
}

# raw_tree_check <dir>: N1. Every tracked file under <dir> (the manifest tree) is compared as RAW bytes with its
# HEAD blob (hash-object --no-filters, which never consults the stat cache or any local filter), and every
# tracked symlink by its target bytes. `git status` alone cannot prove this: its stat cache can be fooled
# (core.checkStat / core.trustctime plus a restored mtime) and local clean filters make a tampered HEAD blob
# look like the restored working-tree bytes. Returns 1 after noting a BLIND.
raw_tree_check() {
  local d="$1" rec meta path mode sha tgt want n i got
  local -a paths=() shas=() gots=()
  # a failing ls-tree inside the process substitution below would look like an empty tree; probe it first
  if ! git -C "$REPO" ls-tree HEAD -- "$d/" >/dev/null 2>&1; then
    note_blind "git ls-tree failed for $(printf '%q' "$d"); cannot prove the manifest tree matches HEAD"; return 1
  fi
  # -z: names are NUL-delimited and never quoted, so tabs, quotes and newlines in names are handled
  while IFS= read -r -d '' rec; do
    meta="${rec%%$'\t'*}"; path="${rec#*$'\t'}"
    read -r mode _ sha <<< "$meta"
    case "$mode" in
      100644|100755)
        if [[ "$path" == *$'\n'* ]]; then
          # --stdin-paths is newline-delimited: hash such a file on its own
          got="$(git -C "$REPO" hash-object --no-filters -- "$path" 2>/dev/null)" || { note_blind "git hash-object failed under manifest tree $(printf '%q' "$d")"; return 1; }
          if [ "$got" != "$sha" ]; then note_blind "manifest tree $(printf '%q' "$d"): $(printf '%q' "$path") raw bytes differ from the HEAD blob"; return 1; fi
        else
          paths+=("$path"); shas+=("$sha")
        fi ;;
      120000)
        tgt="$(readlink -- "$REPO/$path"; printf .)"; tgt="${tgt%.}"; tgt="${tgt%$'\n'}"
        want="$(git -C "$REPO" cat-file blob "$sha" 2>/dev/null; printf .)" || { note_blind "git cat-file failed for $(printf '%q' "$path")"; return 1; }
        want="${want%.}"
        if [ ! -L "$REPO/$path" ] || [ "$tgt" != "$want" ]; then
          note_blind "manifest tree $(printf '%q' "$d"): symlink $(printf '%q' "$path") differs from HEAD"; return 1
        fi;;
      160000)
        # R3-1: a clone (non-recursive) has nothing below a gitlink and git status may be told to ignore it
        note_blind "manifest tree $(printf '%q' "$d") contains the submodule $(printf '%q' "$path"); a clone does not provide its files, so the manifest cannot be proven"; return 1 ;;
      *)
        note_blind "manifest tree $(printf '%q' "$d"): $(printf '%q' "$path") has the unsupported HEAD mode $mode"; return 1 ;;
    esac
  done < <(git -C "$REPO" ls-tree -r -z HEAD -- "$d/" 2>/dev/null)
  n="${#paths[@]}"
  [ "$n" -gt 0 ] || return 0
  if ! got="$(printf '%s\n' "${paths[@]}" | git -C "$REPO" hash-object --no-filters --stdin-paths 2>/dev/null)"; then
    note_blind "git hash-object failed under manifest tree $(printf '%q' "$d") (a tracked file is missing or unreadable)"; return 1
  fi
  mapfile -t gots <<< "$got"
  if [ "${#gots[@]}" -ne "$n" ]; then
    note_blind "git hash-object returned ${#gots[@]} hashes for $n files under $(printf '%q' "$d")"; return 1
  fi
  for ((i=0; i<n; i++)); do
    if [ "${gots[$i]}" != "${shas[$i]}" ]; then
      note_blind "manifest tree $(printf '%q' "$d"): $(printf '%q' "${paths[$i]}") raw bytes differ from the HEAD blob (edit hidden from git status, or a local filter / eol conversion)"; return 1
    fi
  done
  return 0
}

# check_manifest_dir <dir>: verify every manifest in the directory (syntax, escapes, checksums) and
# that the manifests list every regular file of the tree they cover (RC-3b). Nested manifests found
# below are queued for the same checks.
check_manifest_dir() {
  local d="$1" mf line name status rc o esc ent rp sawfail sawunv sawwaived mfs
  mfs="$(manifests_in "$d")"
  if [ ! -d "$SNAP/$d" ] || [ -z "$mfs" ]; then
    note_blind "manifest directory $(printf '%q' "$d") is missing or has no tracked manifest"; return
  fi
  # I-1: a clone sees HEAD, not the index or the working tree. Any tracked file under the covered tree
  # (manifests included) that is staged-but-uncommitted, modified or deleted makes this verdict unprovable.
  local dirty
  if ! dirty="$(git -C "$REPO" -c core.quotepath=off status --porcelain=v1 --ignore-submodules=none --untracked-files=no -- ":(literal)$d" 2>/dev/null)"; then
    note_blind "git status failed for $(printf '%q' "$d"); cannot prove it matches HEAD"; return
  fi
  local flagged
  if ! flagged="$(git -C "$REPO" -c core.quotepath=off ls-files -v -- ":(literal)$d" 2>/dev/null)"; then
    note_blind "git ls-files failed for $(printf '%q' "$d"); cannot prove no file is hidden from git status"; return
  fi
  flagged="$(printf '%s\n' "$flagged" | grep -m1 '^[a-zS] ' || true)"
  if [ -n "$flagged" ]; then
    note_blind "manifest tree $(printf '%q' "$d") holds a skip-worktree or assume-unchanged file ($flagged), which hides changes from git status"; return
  fi
  if [ -n "$dirty" ]; then
    note_blind "manifest tree $(printf '%q' "$d") has changes that are not committed (index or working tree differ from HEAD): $(printf '%s\n' "$dirty" | head -3 | tr '\n' ';')"; return
  fi
  raw_tree_check "$d" || return
  while IFS= read -r mf; do
    [ -n "$mf" ] || continue
    # F4: manifest entries must stay inside the manifest's directory.
    esc=0
    while IFS= read -r line || [ -n "$line" ]; do
      ent=""
      line="${line%$'\r'}"   # CRLF manifests: sha256sum implementations strip one trailing CR
      if [[ "$line" == *$'\r'* ]]; then
        S_FAIL=$((S_FAIL+1)); esc=1; echo "SHAFAIL  $d/$mf unsupported manifest line: CR inside a line"; continue
      fi
      if [[ "$line" =~ ^[0-9a-fA-F]{64}\ [\ \*](.*)$ ]]; then
        ent="${BASH_REMATCH[1]}"
      elif [[ "$line" =~ ^[A-Za-z0-9-]+\ \((.*)\)\ =\ [0-9a-fA-F]+$ ]]; then
        ent="${BASH_REMATCH[1]}"   # N2: BSD-tag form is parsed too
      fi
      if [ -n "$ent" ]; then
        if [[ "$ent" == /* || "$ent" == ../* || "$ent" == */../* || "$ent" == */.. || "$ent" == .. ]]; then
          S_FAIL=$((S_FAIL+1)); esc=1; echo "SHAFAIL  $d/$mf entry escapes its directory: $ent"
        else
          # F2: an in-directory symlink must not lead outside the repository.
          rp="$(readlink -m -- "$SNAP/$d/$ent" 2>/dev/null || true)"   # -m: missing components are fine
          # N1: an entry reached through a symlink reads a file the tree walk above did not compare; walk it
          if [ -n "$rp" ] && [ "$rp" != "$(realpath -m -s -- "$REPO_REAL/$d/$ent" 2>/dev/null)" ]; then
            head_check "manifest entry $d/$ent" "$d/$ent"
          fi
          if [ -z "$rp" ] || { [ "$rp" != "$REPO_REAL" ] && [[ "$rp" != "$REPO_REAL"/* ]]; }; then
            S_FAIL=$((S_FAIL+1)); esc=1; echo "SHAFAIL  $d/$mf entry resolves outside the repository: $ent"
          fi
        fi
      elif [ -n "${line//[[:space:]]/}" ]; then
        # F1: fail closed on any non-blank line that is not one of the two unescaped forms
        # (backslash-escaped GNU/BSD lines, comments, anything else the parser cannot judge).
        S_FAIL=$((S_FAIL+1)); esc=1; echo "SHAFAIL  $d/$mf unsupported manifest line: $line"
      fi
    done < "$SNAP/$d/$mf"
    [ "$esc" = 0 ] || continue
    o="$(cd "$SNAP/$d" && sha256sum --strict -c "$mf" 2>&1)"; rc=$?
    sawfail=0; sawunv=0; sawwaived=0
    while IFS= read -r line; do
      if [[ "$line" =~ ^(.*):\ (OK|FAILED\ open\ or\ read|FAILED)$ ]]; then
        name="${BASH_REMATCH[1]}"; status="${BASH_REMATCH[2]}"
        name="${name#./}"   # sha256sum echoes entries as written; waivers use the bare name
        case "$status" in
          OK) S_OK=$((S_OK+1)) ;;
          FAILED)
            # I-2: a checksum MISMATCH is tampering or corruption and is never waivable; bad-entry
            # waivers cover only a manifest entry whose file is missing.
            S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"
            if [ -n "${WV["bad-entry|$d|$name"]+x}" ]; then
              WVUSED["bad-entry|$d|$name"]=1; W_BAD=$((W_BAD+1))
              echo "BADWAIVER  bad-entry waiver for $d/$name refused: a checksum mismatch is tampering or corruption and is never waivable"
            fi ;;
          *)
            if "${GIT_SNAP[@]}" check-ignore -q -- "$d/$name"; then
              S_UNV=$((S_UNV+1)); sawunv=1; echo "UNVERIFIABLE-FROM-CLONE  $d/$name (gitignored, absent/unreadable)"
            elif waived "bad-entry|$d|$name" "$d/$name (missing/unreadable)"; then sawwaived=1
            else
              S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (missing/unreadable, not gitignored)"
            fi ;;
        esac
      fi
    done <<< "$o"
    if grep -q 'improperly formatted' <<< "$o" || { [ "$rc" -ne 0 ] && [ "$sawfail" = 0 ] && [ "$sawunv" = 0 ] && [ "$sawwaived" = 0 ]; }; then
      S_FAIL=$((S_FAIL+1)); echo "SHAFAIL  $d/$mf (malformed or sha256sum error: $(grep -m1 -E 'improperly|sha256sum:' <<< "$o" || head -1 <<< "$o"))"
    fi
  done <<< "$mfs"

  # RC-3b: every regular file of the covered tree must be listed by the nearest manifest above it.
  local cout kind a b nend=0 lastkind="" ne=-1 nu=-1 nn=-1
  cout="$(python3 -I - "$REPO" "$d" "$SNAP" <<'PY'
import fnmatch, os, re, subprocess, sys
repo, d, snap = sys.argv[1:4]
GIT_TO = int(os.environ.get("RELCHK_GIT_TIMEOUT", "120") or "120")
def is_mf(n):
    return n in ("SHA256SUMS", "SHA256SUMS.txt") or fnmatch.fnmatchcase(n, "MANIFEST*.sha256")
def esc(t):
    return re.sub(r"[\x00-\x1f\x7f]", lambda m: "\\x%02x" % ord(m.group()), t)
def entries(path):
    out = set()
    with open(path, encoding="utf-8", errors="surrogateescape") as fh:
        for ln in fh.read().split("\n"):
            # (no CR strip: open() in text mode already folds \r\n and lone \r into \n)
            m = re.match(r"^[0-9a-fA-F]{64} [ *](.*)$", ln) or re.match(r"^[A-Za-z0-9-]+ \((.*)\) = [0-9a-fA-F]+$", ln)
            if m:
                out.add(os.path.normpath(m.group(1)))
    return out
def ls():
    # HEAD's tree only: never the index, never untracked or ignored working-tree files
    r = subprocess.run(["git", "-C", repo, "ls-tree", "-r", "-z", "--name-only", "--full-tree", "HEAD", "--", d + "/"], capture_output=True, timeout=GIT_TO)
    if r.returncode != 0:
        print("ERR\tgit ls-tree failed"); sys.exit(0)
    return {p.decode("utf-8", "surrogateescape") for p in r.stdout.split(b"\0") if p}
tracked = ls()
files = sorted(tracked)
tmf = {f for f in tracked if is_mf(os.path.basename(f))}
mdirs = {os.path.dirname(f) for f in tmf}
listed = {}
def listed_for(md):
    if md not in listed:
        s = set()
        for f in sorted(tmf):
            if os.path.dirname(f) == md and os.path.isfile(os.path.join(snap, f)):
                s |= entries(os.path.join(snap, f))
        listed[md] = s
    return listed[md]
nested = set()
nfiles = 0
nunl = 0
for f in files:
    full = os.path.join(snap, f)
    if f in tmf or os.path.islink(full) or not os.path.isfile(full):
        continue
    nfiles += 1
    md = os.path.dirname(f)
    while md and md != d and md not in mdirs:
        md = os.path.dirname(md)
    if md != d:
        nested.add(md)
        continue
    rel = os.path.relpath(f, d)
    if os.path.normpath(rel) not in listed_for(d):
        nunl += 1
        print("UNLISTED\t" + esc(rel))
for n in sorted(nested):
    if re.search(r"[\x00-\x1f\x7f]", n):
        print("BADNAME\t" + esc(n))
    else:
        print("NESTED\t" + n)
print(f"END\t{nfiles}\t{nunl}\t{len(nested)}")
PY
)" || { note_blind "completeness helper crashed for $d"; return; }
  local -a unl=() nst=()
  while IFS=$'\t' read -r kind a b; do
    case "$kind" in
      ERR) note_blind "completeness helper: $a"; return ;;
      UNLISTED) unl+=("$a") ;;
      NESTED) nst+=("$a") ;;
      BADNAME) note_blind "nested manifest directory name contains a control character: $a"; return ;;
      END) nend=$((nend+1)); ne="$a"; nu="$b"; nn="${b#*$'\t'}"; nu="${nu%%$'\t'*}" ;;
    esac
    [ -z "$kind" ] || lastkind="$kind"
  done <<< "$cout"
  if [ "$nend" -ne 1 ] || [ "$lastkind" != END ] || ! [[ "$ne" =~ ^[0-9]+$ ]] \
     || [ "$nu" != "${#unl[@]}" ] || [ "$nn" != "${#nst[@]}" ]; then
    note_blind "completeness helper output incomplete for $d"; return
  fi
  local f
  for f in "${unl[@]+"${unl[@]}"}"; do
    if ! waived "unlisted|$d|$f" "$d/$f (not listed in manifest)"; then
      U_FILES=$((U_FILES+1)); echo "UNLISTED  $d/$f (regular file not listed in $d's manifest)"
    fi
  done
  for f in "${nst[@]+"${nst[@]}"}"; do queue+=("$f"); done
}

# ---------------------------------------------------------------- run
case "$CHECK" in
  version) run_versions ;;
  changelog) run_changelog ;;
  all) run_versions; run_changelog ;;
esac

SUMMARY="section=$SECTION_USED version_ok=$V_OK version_fail=$V_FAIL changelog_paths=$C_PATHS miss=$C_MISS skipped=$C_SKIP sha_ok=$S_OK sha_fail=$S_FAIL unverifiable=$S_UNV no_manifest=$C_NOMAN unlisted=$U_FILES waived=$W_APPLIED bad_waivers=$W_BAD unused_waivers=$W_UNUSED"
if [ $((V_FAIL + C_MISS + S_FAIL + C_NOMAN + U_FILES + W_BAD)) -gt 0 ]; then
  echo "VERDICT: FAIL $SUMMARY"; exit 1
fi
if [ -n "$BLINDS" ]; then
  echo "VERDICT: BLIND $SUMMARY reason:$BLINDS"; exit 2
fi
if [ "$W_APPLIED" -gt 0 ]; then
  # exit 0 like PASS, but distinct on the VERDICT line: the result relies on declared evidence gaps
  echo "VERDICT: PASS-WITH-WAIVERS $SUMMARY"; exit 0
fi
echo "VERDICT: PASS $SUMMARY"
exit 0
