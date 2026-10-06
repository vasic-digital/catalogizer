#!/usr/bin/env bash
# test_check_pins.sh - T104 (check_pins.sh: unpinned external image references and pipe-to-shell installs).
# Every fixture is written into a temporary directory at run time (deliberate-violation fixtures are never committed).
# Oracles: the exit status of the script under test, AND its own report lines (`VIOLATION <rule> <path>:<line>: ...`) which must
# name the exact rule and the exact physical line of the planted violation (two independent readings of one run).
# Groups: golden-bad (each violation class is flagged), golden-good (digest references pass), carriers (comments and documentation
# that merely MENTION trivy:latest or `curl | sh` must not fire, 11.4.201), negative control (one file holding a carrier and a real
# violation reports only the real line), determinism, usage errors.
# Paired mutations (G-GATE): copies of the script with one behaviour changed (exact text replaced, the replacement must differ from the
# original so a vanished anchor cannot yield a silent no-op mutant); the fixture body is re-run against each copy (CHECKPINS_SUT=<copy>,
# CHECKPINS_TEST_MUTANT=1) and every copy must make it FAIL. The record goes to $CHECKPINS_MUTATION_RECORD (default: scratch).
# Usage: test_check_pins.sh                       run the fixtures and the mutations
#        CHECKPINS_TEST_NO_MUTATIONS=1 ...        fixtures only
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUT="${CHECKPINS_SUT:-$HERE/../check_pins.sh}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 is required by this test"; exit 2; }
[ -f "$SUT" ] || { echo "FAIL: check_pins.sh not found at $SUT"; exit 1; }

T="$(mktemp -d "${TMPDIR:-/tmp}/checkpins-test.XXXXXX")"
trap 'rm -rf "$T"' EXIT
D64="$(printf 'a%.0s' $(seq 64))"
DG="sha256:$D64"

# fx <name> <relative path> : create fixture dir $T/<name> with the file content read from stdin
fx() { mkdir -p "$T/$1/$(dirname "$2")"; cat >"$T/$1/$2"; }
# run <name> : runs the SUT on the fixture dir, sets OUT and RC
run() { OUT="$(bash "$SUT" --root "$T/$1" 2>&1)"; RC=$?; }
# expect_bad <label> <name> <rule> <line> : exit 1 and a VIOLATION line naming rule + physical line
expect_bad() {
  run "$2"
  check "$1: exit status 1" "$RC" "1"
  if printf '%s\n' "$OUT" | grep -qE "^VIOLATION $3 [^ ]+:$4: "; then ok "$1: reports $3 at line $4"
  else bad "$1: no 'VIOLATION $3 <path>:$4:' line in: $(printf '%s' "$OUT" | head -5 | tr '\n' '|')"; fi
}
expect_good() {
  run "$2"
  check "$1: exit status 0" "$RC" "0"
  if printf '%s\n' "$OUT" | grep -q '^VIOLATION '; then bad "$1: unexpected VIOLATION: $(printf '%s' "$OUT" | grep '^VIOLATION' | head -3 | tr '\n' '|')"; else ok "$1: no violation reported"; fi
}

# ------------------------------------------------------------------ golden-bad
fx bad_latest docker-compose.security.yml <<'EOF'
services:
  trivy-scanner:
    image: docker.io/aquasec/trivy:latest
    container_name: x
EOF
expect_bad "G-BAD compose :latest" bad_latest compose_image_unpinned 3

fx bad_notag docker-compose.yml <<'EOF'
services:
  web:
    image: nginx
EOF
expect_bad "G-BAD compose no tag no digest" bad_notag compose_image_unpinned 3

fx bad_mutable docker-compose.dev.yml <<'EOF'
services:
  db:
    image: "docker.io/library/postgres:15-alpine"   # mutable tag
EOF
expect_bad "G-BAD compose mutable tag (quoted, trailing comment)" bad_mutable compose_image_unpinned 3

fx bad_shortdigest docker-compose.yml <<'EOF'
services:
  web:
    image: docker.io/library/nginx@sha256:abc123
EOF
expect_bad "G-BAD compose short digest" bad_shortdigest compose_image_unpinned 3

fx bad_envdefault docker-compose.yml <<'EOF'
services:
  web:
    image: ${WEB_IMAGE:-docker.io/library/nginx:latest}
EOF
expect_bad "G-BAD compose variable default unpinned" bad_envdefault compose_image_unpinned 3

fx bad_from Dockerfile <<'EOF'
FROM golang:1.25
RUN go version
EOF
expect_bad "G-BAD Dockerfile FROM tag only" bad_from from_unpinned 1

fx bad_from_alias build/containers/x/Containerfile <<'EOF'
# header
FROM --platform=linux/amd64 docker.io/library/debian:bookworm-slim AS base
RUN true
EOF
expect_bad "G-BAD Containerfile FROM with --platform and alias" bad_from_alias from_unpinned 2

fx bad_arg Dockerfile.arg <<'EOF'
ARG BASE=docker.io/library/debian:12
FROM ${BASE}
EOF
expect_bad "G-BAD FROM \${ARG} whose default is unpinned" bad_arg from_arg_unpinned 2

fx bad_copyfrom Dockerfile <<EOF
FROM docker.io/library/debian@$DG AS build
FROM docker.io/library/debian@$DG
COPY --from=docker.io/library/busybox:1.36 /bin/busybox /bin/busybox
EOF
expect_bad "G-BAD COPY --from external image tag only" bad_copyfrom copy_from_unpinned 3

fx bad_script_reg scripts/scan.sh <<'EOF'
#!/usr/bin/env bash
podman run --rm docker.io/aquasec/trivy:latest fs .
EOF
expect_bad "G-BAD script podman run registry image" bad_script_reg script_image_unpinned 2

fx bad_script_short scripts/t.sh <<'EOF'
#!/usr/bin/env bash
docker run --rm -v "$PWD":/src -w /src golang:1.25 go test ./...
EOF
expect_bad "G-BAD script docker run short image" bad_script_short script_image_unpinned 2

fx bad_pipe_cf Containerfile <<'EOF'
FROM docker.io/library/debian@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
RUN curl -fsSL https://example.invalid/install.sh | sh
EOF
expect_bad "G-BAD Containerfile download piped into sh" bad_pipe_cf pipe_to_shell 2

fx bad_pipe_bash scripts/i.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://example.invalid/i.sh | sudo bash -s -- --yes
EOF
expect_bad "G-BAD script download piped into sudo bash" bad_pipe_bash pipe_to_shell 2

fx bad_pipe_wget scripts/w.sh <<'EOF'
#!/usr/bin/env bash
wget -qO- https://example.invalid/i.sh | sh
EOF
expect_bad "G-BAD script wget -qO- piped into sh" bad_pipe_wget pipe_to_shell 2

fx bad_subst scripts/s.sh <<'EOF'
#!/usr/bin/env bash
bash <(curl -fsSL https://example.invalid/i.sh)
EOF
expect_bad "G-BAD script bash <(download) process substitution" bad_subst pipe_to_shell 2

fx bad_subst2 Containerfile <<'EOF'
FROM docker.io/library/debian@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
RUN sh -c "$(curl -fsSL https://example.invalid/i.sh)"
EOF
expect_bad "G-BAD Containerfile sh -c command substitution of a download" bad_subst2 pipe_to_shell 2

# the multi-line form of docker/Dockerfile.builder:66-68: a pipe after a for ... done loop spread over continuation lines
fx bad_pipe_multi docker/Dockerfile.builder <<'EOF'
FROM docker.io/library/debian@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
RUN for i in $(seq 1 10); do \
    curl -fsSL https://example.invalid/setup_18.x && break || (echo "Attempt $i failed, waiting 15s..." && sleep 15); \
    done | bash - && apt-get install -y nodejs && rm -rf /var/lib/apt/lists/*
EOF
expect_bad "G-BAD multi-line for...done | bash - (reported at its first physical line)" bad_pipe_multi pipe_to_shell 2

# the quoted install hint a script prints for the operator (scripts/security-scan-full.sh:40 form)
fx bad_hint scripts/security-scan-full.sh <<'EOF'
#!/usr/bin/env bash
echo "Checking required tools..."
check_tool trivy "curl -sfL https://example.invalid/trivy/install.sh | sh"
EOF
expect_bad "G-BAD quoted install hint (download piped into sh) printed for the operator" bad_hint pipe_to_shell 3

# ------------------------------------------------------------------ golden-good
fx good_all docker-compose.yml <<EOF
services:
  a:
    image: docker.io/library/postgres@$DG
  b:
    image: docker.io/library/redis:7-alpine@$DG
  c:
    image: localhost/catalogizer-testutil:7da5779224fb6c94
  d:
    image: \${D_IMAGE:-docker.io/library/nginx@$DG}
  e:
    image: \${EXTERNAL_IMAGE}
EOF
expect_good "G-GOOD compose digest forms, locally built localhost image, variable with digest default, bare variable" good_all

fx good_from Dockerfile <<EOF
ARG BASE=docker.io/library/debian@$DG
ARG TESTUTIL_REF
FROM \${BASE} AS build
FROM \${TESTUTIL_REF} AS testutil
FROM scratch AS empty
FROM build AS final
COPY --from=build /a /a
COPY --from=0 /b /b
COPY --from=docker.io/library/busybox@$DG /bin/busybox /bin/busybox
FROM docker.io/library/golang:1.25-bookworm@$DG
EOF
expect_good "G-GOOD Dockerfile digest FROM, stage aliases, scratch, ARG without default, COPY --from digest" good_from

fx good_script scripts/ok.sh <<EOF
#!/usr/bin/env bash
podman run --rm -v "\$PWD":/src:ro -p 8080:80 -e FOO=bar:baz docker.io/aquasec/trivy@$DG fs /src
docker run --rm docker.io/library/alpine@$DG true
curl -fsSL -o /tmp/f.tgz https://example.invalid/f.tgz && echo "x  /tmp/f.tgz" | sha256sum -c -
curl -fsSL https://example.invalid/f.tgz | tar xz -C /opt
curl -fsS https://example.invalid/api | jq .
REF="docker.io/example/forged@sha256:\$(printf 'f%.0s' \$(seq 64))"
curl -fsS https://example.invalid/x -o /tmp/x || sh /tmp/fallback.sh
EOF
expect_good "G-GOOD script digest image, volume/port/env tokens, download then sha256sum, pipe into tar and jq" good_script

# ------------------------------------------------------------------ carriers (11.4.201: a mention is not the thing)
fx carrier docker-compose.yml <<EOF
# image: docker.io/aquasec/trivy:latest   <- documentation of the old line
services:
  a:
    image: docker.io/library/postgres@$DG   # was postgres:15-alpine
EOF
expect_good "CARRIER comment and trailing comment in compose" carrier

fx carrier_sh scripts/c.sh <<'EOF'
#!/usr/bin/env bash
# never run: curl -fsSL https://example.invalid/i.sh | sh
# podman run --rm docker.io/aquasec/trivy:latest fs .
true # curl x | bash   (trailing comment)
EOF
expect_good "CARRIER comments in a script" carrier_sh

fx carrier_cf Containerfile <<EOF
# FROM golang:1.25 is forbidden; curl | bash too
FROM docker.io/library/debian@$DG
EOF
expect_good "CARRIER comment in a Containerfile" carrier_cf

fx carrier_docs README.md <<'EOF'
# Docs
The compose file used `image: docker.io/aquasec/trivy:latest`; the old hint was `curl -sfL https://x/install.sh | sh`.
```
FROM golang:1.25
curl https://x | bash
```
EOF
mkdir -p "$T/carrier_docs/docs"; cp "$T/carrier_docs/README.md" "$T/carrier_docs/docs/notes.txt"
cp "$T/carrier_docs/README.md" "$T/carrier_docs/docs/Dockerfile.md"
expect_good "CARRIER documentation (Markdown and text are not scanned)" carrier_docs

fx hd_test scripts/tests/test_x.sh <<'OUTER'
#!/usr/bin/env bash
cat >"$T/fx/Dockerfile" <<'EOF'
FROM golang:1.25
RUN curl -fsSL https://example.invalid/i.sh | sh
EOF
EOF_NOTE=1
OUTER
expect_good "CARRIER here-document body (fixture data) in a script under tests/" hd_test

fx hd_prod scripts/gen.sh <<'OUTER'
#!/usr/bin/env bash
cat >Dockerfile <<'EOF'
RUN curl -fsSL https://example.invalid/i.sh | sh
EOF
OUTER
run hd_prod
check "HEREDOC a here-document body in a production script is judged like code (exit 1)" "$RC" "1"

fx bad_assign scripts/img.sh <<'EOF'
#!/usr/bin/env bash
IMG=docker.io/aquasec/trivy:latest
EOF
expect_bad "G-BAD script registry reference with a tag in an assignment" bad_assign script_image_unpinned 2

# negative control (11.4.201(7)): one file with a carrier on line 1 and the real violation on line 4 reports ONLY line 4
fx negctl docker-compose.yml <<'EOF'
# image: docker.io/aquasec/trivy:latest
services:
  trivy:
    image: docker.io/aquasec/trivy:latest
EOF
run negctl
check "NEGCTL exit 1" "$RC" "1"
check "NEGCTL exactly one VIOLATION line" "$(printf '%s\n' "$OUT" | grep -c '^VIOLATION ')" "1"
if printf '%s\n' "$OUT" | grep -qE '^VIOLATION compose_image_unpinned [^ ]+:4: '; then ok "NEGCTL the one violation is the real line 4"; else bad "NEGCTL wrong line: $OUT"; fi

# ------------------------------------------------------------------ list format, determinism, usage
run bad_latest
OUT1="$OUT"; run bad_latest; check "DETERMINISM two runs print identical output" "$OUT" "$OUT1"
LIST="$(bash "$SUT" --root "$T/bad_latest" --list 2>&1)"; LRC=$?
check "LIST exit status 1 on a violation" "$LRC" "1"
check "LIST one TSV row rule<TAB>path<TAB>line<TAB>reference" "$LIST" "$(printf 'compose_image_unpinned\tdocker-compose.security.yml\t3\tdocker.io/aquasec/trivy:latest')"
run good_all; check "SUMMARY line on a clean tree" "$(printf '%s\n' "$OUT" | tail -1 | sed 's/[0-9][0-9]*/N/g')" "check_pins: N violations in N files scanned"
bash "$SUT" --bogus >/dev/null 2>&1; check "USAGE unknown option exits 2" "$?" "2"
bash "$SUT" --root "$T/does-not-exist" >/dev/null 2>&1; check "USAGE missing root exits 2" "$?" "2"
# a file argument is scanned as given
bash "$SUT" --root "$T/bad_from" Dockerfile >/dev/null 2>&1; check "ARG explicit file path is scanned (violation exit 1)" "$?" "1"

# ------------------------------------------------------------------ paired mutations
if [ -z "${CHECKPINS_TEST_MUTANT:-}" ] && [ -z "${CHECKPINS_TEST_NO_MUTATIONS:-}" ]; then
  MUT_RECORD="${CHECKPINS_MUTATION_RECORD:-$T/check-pins-mutation.txt}"
  { echo "# check-pins-mutation record: paired mutations of scripts/containers/check_pins.sh (G-GATE)"; echo "# run_at: $(date -u +%Y-%m-%dT%H:%M:%SZ) host: $(hostname)"; echo "# sut_sha256: $(sha256sum "$SUT" | cut -d' ' -f1)"; } >"$MUT_RECORD"
  MUTS="$HERE/check_pins_mutations.tsv"
  [ -r "$MUTS" ] || { bad "mutation table $MUTS missing"; MUTS=/dev/null; }
  nm=0
  while IFS=$'\t' read -r name old new; do
    case "$name" in ''|'#'*) continue;; esac
    nm=$((nm+1)); cp_="$T/mutant-$name.sh"
    if ! python3 -I - "$SUT" "$cp_" "$old" "$new" <<'PY'
import sys
src = open(sys.argv[1]).read()
old = sys.argv[3].encode().decode("unicode_escape"); new = sys.argv[4].encode().decode("unicode_escape")
if old == new or src.count(old) != 1:
    print("anchor count %d (need exactly 1) or identical replacement" % src.count(old)); sys.exit(3)
open(sys.argv[2], "w").write(src.replace(old, new))
PY
    then bad "MUT $name: cannot be applied (anchor missing or not unique)"; echo "MUTANT $name: NOT-APPLIED" >>"$MUT_RECORD"; continue; fi
    mo="$(CHECKPINS_SUT="$cp_" CHECKPINS_TEST_MUTANT=1 QUIET=1 bash "$0" 2>&1)"; mrc=$?
    if [ "$mrc" -ne 0 ]; then ok "MUT $name: caught (fixture body failed: $(printf '%s' "$mo" | grep -c '^FAIL') FAIL lines)"; echo "MUTANT $name: CAUGHT rc=$mrc first_fail=$(printf '%s' "$mo" | grep -m1 '^FAIL')" >>"$MUT_RECORD"
    else bad "MUT $name: SURVIVED (fixture body passed against the mutant)"; echo "MUTANT $name: SURVIVED" >>"$MUT_RECORD"; fi
  done <"$MUTS"
  check "MUT at least one mutation ran" "$([ "$nm" -ge 1 ] && echo yes || echo no)" "yes"
  echo "# mutations: $nm" >>"$MUT_RECORD"
fi

echo "test_check_pins: $PASSES passed, $FAILS failed"
[ "$FAILS" -eq 0 ]
