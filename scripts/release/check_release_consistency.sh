#!/usr/bin/env bash
# check_release_consistency.sh - release-prep consistency checker (see README.md).
#   (1) version: every product-version field agrees with the expected version.
#   (2) changelog: every backticked repo path in the VERIFIED subsection of a
#       CHANGELOG section exists and is git-tracked; SHA256SUMS manifests verify.
# Exit: 0 all OK, 1 any FAIL, 2 usage/blind (never a silent pass).
# Always prints one machine-readable line:  VERDICT: PASS|FAIL|BLIND ...
set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SELF="$HERE/$(basename "${BASH_SOURCE[0]}")"

REPO=""
EXPECTED=""
PREV_TAG=""
SECTION=""
CHECK="all"
SELFTEST=0

usage() {
  cat <<'U'
usage: check_release_consistency.sh [--repo DIR] [--version X.Y.Z] [--prev-tag TAG]
                                    [--section X.Y.Z] [--check version|changelog|all]
       check_release_consistency.sh --selftest
  --repo      repository root (default: the repo containing this script)
  --version   expected version (default: versions.json global major.minor.patch)
  --prev-tag  git tag; versionCode of each Android app must be an integer strictly
              greater than its value at that tag
  --section   CHANGELOG section version (default: topmost numbered section)
exit: 0 OK, 1 FAIL, 2 usage/blind.  Last line printed is "VERDICT: ...".
U
}

blind() { echo "BLIND: $*"; echo "VERDICT: BLIND $*"; exit 2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --repo) [ $# -ge 2 ] || blind "--repo needs a value"; REPO="$2"; shift 2 ;;
    --version) [ $# -ge 2 ] || blind "--version needs a value"; EXPECTED="$2"; shift 2 ;;
    --prev-tag) [ $# -ge 2 ] || blind "--prev-tag needs a value"; PREV_TAG="$2"; shift 2 ;;
    --section) [ $# -ge 2 ] || blind "--section needs a value"; SECTION="$2"; shift 2 ;;
    --check) [ $# -ge 2 ] || blind "--check needs a value"; CHECK="$2"; shift 2 ;;
    --selftest) SELFTEST=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; blind "unknown option $1" ;;
  esac
done

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

V_OK=0; V_FAIL=0; C_PATHS=0; C_MISS=0; C_SKIP=0; S_OK=0; S_FAIL=0; S_UNV=0; BLINDS=""
SECTION_USED="-"
note_blind() { BLINDS="$BLINDS ${1}"; echo "BLIND: $1"; }

# ---------------------------------------------------------------- (1) versions
run_versions() {
  echo "== version consistency"
  local kind label detail
  while IFS=$'\t' read -r kind label detail; do
    case "$kind" in
      OK)   V_OK=$((V_OK+1));   echo "OK    $label: $detail" ;;
      FAIL) V_FAIL=$((V_FAIL+1)); echo "FAIL  $label: $detail" ;;
      INFO) echo "INFO  $label: $detail" ;;
      BLIND) note_blind "$label $detail" ;;
    esac
  done < <(python3 -I - "$REPO" "$EXPECTED" "$PREV_TAG" <<'PY'
import json, os, re, subprocess, sys, tomllib

repo, expected, prevtag = sys.argv[1:4]

def emit(kind, label, detail=""):
    print(f"{kind}\t{label}\t{detail}")

def read(path):
    with open(os.path.join(repo, path), encoding="utf-8") as fh:
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

# versions.json global
check("versions.json global", lambda: (_ for _ in ()).throw(ValueError(vj_err)) if not vj_ok else derived)

# openapi info.version (indent-2 key directly inside top-level info:)
def openapi():
    lines = read("docs/api/openapi.yaml").splitlines()
    inside = False
    for ln in lines:
        if re.match(r"^info:\s*(#.*)?$", ln):
            inside = True
            continue
        if inside:
            if ln and not ln.startswith((" ", "\t", "#")):
                break
            m = re.match(r"^  version:\s*[\"']?([^\"'#\s]+)[\"']?\s*(#.*)?$", ln)
            if m:
                return m.group(1)
    raise ValueError("info.version not found")
check("docs/api/openapi.yaml info.version", openapi)

# npm packages and lockfiles
for comp in ("catalog-web", "catalogizer-desktop", "installer-wizard"):
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
for comp in ("catalogizer-desktop", "installer-wizard"):
    check(f"{comp}/src-tauri/tauri.conf.json version", lambda c=comp: tauri(f"{c}/src-tauri/tauri.conf.json"))

# Cargo.toml [package] version
def cargo(path):
    v = tomllib.loads(read(path))["package"]["version"]
    if not isinstance(v, str):
        raise ValueError(f"non-literal package.version {v!r}")
    return v
for comp in ("catalogizer-desktop", "installer-wizard"):
    check(f"{comp}/src-tauri/Cargo.toml [package] version", lambda c=comp: cargo(f"{c}/src-tauri/Cargo.toml"))

# Gradle versionName / versionCode
def gradle_field(text, name):
    pat = re.compile(r"^\s*" + name + r"\s*(?:=\s*)?(\"[^\"]*\"|[^\s/]+)")
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
                       capture_output=True, text=True)
    if r.returncode != 0:
        tag_ok = False
        emit("FAIL", "--prev-tag", f"tag {prevtag!r} not found")

for app in ("catalogizer-android", "catalogizer-androidtv"):
    path = f"{app}/app/build.gradle.kts"
    if not os.path.exists(os.path.join(repo, path)):
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
    r = subprocess.run(["git", "-C", repo, "show", f"{prevtag}:{path}"], capture_output=True, text=True)
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
PY
)
}

# ---------------------------------------------------------------- (2) changelog
tracked() { # tracked <path>: true when git tracks the path (file or directory)
  git -C "$REPO" ls-files -- ":(literal)$1" 2>/dev/null | grep -q .
}

run_changelog() {
  echo "== changelog evidence paths"
  local cl="$REPO/docs/CHANGELOG.md"
  [ -f "$cl" ] || { note_blind "docs/CHANGELOG.md not found"; return; }

  # 11.4.273 control needles: the existence instrument must see a known-present tracked
  # path and must not see a fabricated one.
  local pos neg
  pos="$(git -C "$REPO" ls-files | head -1)"
  neg="__no_such_path_$$__/none"
  if [ -z "$pos" ] || ! { [ -e "$REPO/$pos" ] && tracked "$pos"; } || { [ -e "$REPO/$neg" ] || tracked "$neg"; }; then
    note_blind "control needle failed: existence/tracked instrument cannot be trusted"; return
  fi

  local out kind a b
  out="$(python3 -I - "$REPO" "$SECTION" <<'PY'
import os, re, subprocess, sys
repo, want = sys.argv[1:3]
with open(os.path.join(repo, "docs/CHANGELOG.md"), encoding="utf-8") as fh:
    lines = fh.read().splitlines()
head = re.compile(r"^## \[(\d[^\]]*)\]")
start = None
for i, ln in enumerate(lines):
    m = head.match(ln)
    if m and (not want or m.group(1) == want):
        start, name = i, m.group(1)
        break
if start is None:
    print("ERR\tsection not found" + (f": {want}" if want else " (no numbered section)"))
    sys.exit(0)
end = len(lines)
for j in range(start + 1, len(lines)):
    if lines[j].startswith("## "):
        end = j
        break
body = lines[start + 1:end]
sub = None
for k, ln in enumerate(body):
    if re.match(r"^###\s+VERIFIED\b", ln):
        sub = k
        break
if sub is None:
    print(f"ERR\tno VERIFIED subsection in section {name}")
    sys.exit(0)
vlines = []
for ln in body[sub + 1:]:
    if re.match(r"^#{1,3} ", ln):
        break
    vlines.append(ln)
print(f"SECTION\t{name}")
tops = set()
r = subprocess.run(["git", "-C", repo, "ls-files"], capture_output=True, text=True)
for p in r.stdout.splitlines():
    if "/" in p:
        tops.add(p.split("/", 1)[0])
known = re.compile(r"^(docs|scripts|challenges|tests?|tools|specs|catalog[a-z0-9_-]*|installer-wizard|submodules|constitution|Website|config|deployment|\.github)$")
seen = set()
for ln in vlines:
    for tok in re.findall(r"`([^`\n]+)`", ln):
        t = tok.strip()
        if "/" not in t or re.search(r"[\s*?<>${}|\\=(),;\"']", t):
            continue
        if re.match(r"^[a-z][a-z0-9+.-]*://", t) or t.startswith(("/", "~", "-")):
            continue
        t = re.sub(r":\d+(-\d+)?$", "", t.rstrip(".,;:)"))
        if t.startswith("./"):
            t = t[2:]
        top = t.split("/", 1)[0]
        if top in tops or known.match(top):
            if t not in seen:
                seen.add(t)
                print(f"PATH\t{t}")
        else:
            print(f"SKIP\t{t}")
PY
)" || { note_blind "changelog extraction crashed"; return; }

  local paths=() skips=()
  while IFS=$'\t' read -r kind a b; do
    case "$kind" in
      ERR) note_blind "$a"; return ;;
      SECTION) SECTION_USED="$a" ;;
      PATH) paths+=("$a") ;;
      SKIP) skips+=("$a") ;;
    esac
  done <<< "$out"
  : "${b:-}"
  C_SKIP=${#skips[@]}
  if [ "${#paths[@]}" -eq 0 ]; then
    note_blind "zero repo paths extracted from VERIFIED subsection of section $SECTION_USED"; return
  fi
  echo "section $SECTION_USED: ${#paths[@]} path(s) in VERIFIED; skipped non-repo-looking: ${skips[*]:-none}"

  local p q d
  declare -A seen_dir=()
  for p in "${paths[@]}"; do
    C_PATHS=$((C_PATHS+1))
    q="${p%/}"
    if [ ! -e "$REPO/$q" ]; then
      C_MISS=$((C_MISS+1)); echo "MISS  $p (not in working tree)"
    elif ! tracked "$q"; then
      C_MISS=$((C_MISS+1)); echo "MISS  $p (exists but not git-tracked)"
    else
      echo "OK    $p"
    fi
    # nearest SHA256SUMS at or above the path (never the repo root itself)
    if [ -d "$REPO/$q" ]; then d="$q"; else d="$(dirname "$q")"; fi
    while [ -n "$d" ] && [ "$d" != "." ] && [ "$d" != "/" ]; do
      if [ -f "$REPO/$d/SHA256SUMS" ]; then seen_dir["$d"]=1; break; fi
      d="$(dirname "$d")"
    done
  done

  local line name status rc o
  for d in "${!seen_dir[@]}"; do
    o="$(cd "$REPO/$d" && sha256sum -c SHA256SUMS 2>&1)"; rc=$?
    local sawfail=0
    while IFS= read -r line; do
      if [[ "$line" =~ ^(.*):\ (OK|FAILED\ open\ or\ read|FAILED)$ ]]; then
        name="${BASH_REMATCH[1]}"; status="${BASH_REMATCH[2]}"
        case "$status" in
          OK) S_OK=$((S_OK+1)) ;;
          FAILED)
            S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)" ;;
          *)
            if git -C "$REPO" check-ignore -q -- "$d/$name"; then
              S_UNV=$((S_UNV+1)); echo "UNVERIFIABLE-FROM-CLONE  $d/$name (gitignored, absent/unreadable)"
            else
              S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (missing/unreadable, not gitignored)"
            fi ;;
        esac
      fi
    done <<< "$o"
    if [ "$rc" -ne 0 ] && [ "$sawfail" = 0 ] && ! grep -q 'FAILED open or read' <<< "$o"; then
      S_FAIL=$((S_FAIL+1)); echo "SHAFAIL  $d/SHA256SUMS (sha256sum error: $(head -1 <<< "$o"))"
    fi
  done
}

# ---------------------------------------------------------------- run
case "$CHECK" in
  version) run_versions ;;
  changelog) run_changelog ;;
  all) run_versions; run_changelog ;;
esac

SUMMARY="section=$SECTION_USED version_ok=$V_OK version_fail=$V_FAIL changelog_paths=$C_PATHS miss=$C_MISS skipped=$C_SKIP sha_ok=$S_OK sha_fail=$S_FAIL unverifiable=$S_UNV"
if [ $((V_FAIL + C_MISS + S_FAIL)) -gt 0 ]; then
  echo "VERDICT: FAIL $SUMMARY"; exit 1
fi
if [ -n "$BLINDS" ]; then
  echo "VERDICT: BLIND $SUMMARY reason:$BLINDS"; exit 2
fi
echo "VERDICT: PASS $SUMMARY"
exit 0
