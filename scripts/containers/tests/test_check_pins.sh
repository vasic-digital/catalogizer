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

T="$(mktemp -d "${TMPDIR:-/tmp}/checkpins-test.XXXXXX")" && [ -d "$T" ] || { echo "FAIL: cannot create the scratch directory under ${TMPDIR:-/tmp} (mktemp failed); nothing was run"; exit 2; }
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

# ------------------------------------------------------------------ WF10 review p1 fixes (F1 false positives, F2 false negatives, F3 crash, F4 mutation adequacy)
rows() { bash "$SUT" --root "$T/$1" --list 2>/dev/null; }     # TSV rows of a fixture
nrows() { rows "$1" | wc -l | tr -d ' '; }
# expect_row <label> <fixture> <rule> <line> <reference>: exactly this TSV row is present
expect_row() {
  if rows "$2" | awk -F'\t' -v r="$3" -v l="$4" -v ref="$5" '$1==r && $3==l && $4==ref {f=1} END{exit f?0:1}'; then ok "$1: row $3 line $4 reference '$5'"
  else bad "$1: no row '$3 $4 $5' in: $(rows "$2" | tr '\n\t' '| ' | cut -c1-300)"; fi
}
expect_rows_total() { check "$1: total rows" "$(nrows "$2")" "$3"; }

# --- F1 false positives: a carrier or a local image is not a violation (11.4.201(1))
fx fp_addhost scripts/a.sh <<EOF
#!/usr/bin/env bash
podman run --rm --add-host db:10.0.0.1 --add-host=h2:host-gateway docker.io/library/postgres@$DG
EOF
expect_good "F1 --add-host value is not the image operand (pinned image, add-host before it)" fp_addhost
fx fp_addhost2 scripts/a.sh <<'EOF'
#!/usr/bin/env bash
podman run --rm --add-host db:10.0.0.1 postgres:15
EOF
expect_row "F1 --add-host does not mask the real image" fp_addhost2 script_image_unpinned 2 postgres:15
expect_rows_total "F1 --add-host does not mask the real image" fp_addhost2 1
fx fp_composebuild docker-compose.test.yml <<'EOF'
services:
  api:
    build:
      context: .
    image: catalogizer-api:test
  web:
    image: redis:7
  reg:
    build: .
    image: ghcr.io/o/x:1
EOF
expect_row "F1 compose: the local tag of a build: service is skipped, the other service is flagged" fp_composebuild compose_image_unpinned 7 redis:7
expect_row "F1 compose: a registry-qualified image next to build: is still judged" fp_composebuild compose_image_unpinned 10 ghcr.io/o/x:1
expect_rows_total "F1 compose: exactly the two real rows" fp_composebuild 2
fx fp_composebuild_after docker-compose.yml <<'EOF'
services:
  api:
    image: catalogizer-api:test
    build:
      context: .
EOF
expect_good "F1 compose: build: after image: in the same service also marks a local tag" fp_composebuild_after
fx fp_composedollar docker-compose.yml <<'EOF'
services:
  a:
    image: $EXTERNAL_IMAGE
EOF
expect_good "F1 compose: bare \$VAR image is resolved by the caller, not judged" fp_composedollar
fx fp_scriptlocal scripts/a.sh <<'EOF'
#!/usr/bin/env bash
podman run --rm localhost/catalogizer-builder:latest true
EOF
expect_good "F1 script: a localhost/ image is locally built, not external" fp_scriptlocal
fx fp_testdata scripts/tests/test_y.sh <<'EOF'
#!/usr/bin/env bash
printf 'FROM docker.io/library/debian:12\n' >"$T/Containerfile"
sed -i "1s#.*#FROM docker.io/library/debian:12#" "$T/Containerfile"
echo "RUN curl -fsSL https://example.invalid/i.sh | sh" >>"$T/Containerfile"
printf 'RUN curl -fsSL https://example.invalid/i.sh | sh\n' >>"$T/Containerfile"
LINE='docker.io/library/debian:12'
EOF
expect_good "F1 a test script writing fixture lines with printf/sed/echo or an assignment is data" fp_testdata
fx fp_testdata_ctl scripts/y.sh <<'EOF'
#!/usr/bin/env bash
printf 'FROM docker.io/library/debian:12\n' >"$T/Containerfile"
echo "RUN curl -fsSL https://example.invalid/i.sh | sh" >>"$T/Containerfile"
EOF
expect_rows_total "F1 negative control: the same lines in a non-test script are still judged (registry reference + pipe)" fp_testdata_ctl 2
fx fp_testreal scripts/tests/test_z.sh <<'EOF'
#!/usr/bin/env bash
printf 'FROM docker.io/library/debian:12\n' >x
podman run --rm docker.io/library/alpine:3.19 true
EOF
expect_row "F1 negative control: a real podman run in a test script is still judged" fp_testreal script_image_unpinned 3 docker.io/library/alpine:3.19
expect_rows_total "F1 negative control: only the real run row" fp_testreal 1
fx fp_testliteral scripts/tests/test_w.sh <<'EOF'
#!/usr/bin/env bash
IMG=docker.io/library/alpine:3.19
check "label" docker.io/library/alpine:3.19 "$x"
EOF
expect_good "F1 in a test script a registry literal outside an engine command is data (assignment, helper argument)" fp_testliteral
fx fp_testliteral_ctl scripts/w.sh <<'EOF'
#!/usr/bin/env bash
IMG=docker.io/library/alpine:3.19
EOF
expect_rows_total "F1 negative control: the same assignment in a non-test script is judged" fp_testliteral_ctl 1

# --- F2 false negatives
fx fn_trailpipe scripts/a.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://x.invalid/i.sh |
  sh
EOF
expect_bad "F2 a pipe continued by a trailing | (no backslash) into sh" fn_trailpipe pipe_to_shell 2
fx fn_trailpipe_ok scripts/a.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://x.invalid/f.tgz |
  tar xz -C /opt
EOF
expect_good "F2 control: a trailing | into tar is not a shell" fn_trailpipe_ok
fx fn_argredecl Dockerfile <<EOF
ARG RUNTIME=alpine:3.19
FROM golang@sha256:$D64 AS build
ARG RUNTIME
RUN echo \$RUNTIME
FROM \${RUNTIME}
EOF
expect_bad "F2 stage-local ARG redeclaration does not hide the unpinned global default" fn_argredecl from_arg_unpinned 5
fx fn_argredecl_ok Dockerfile <<EOF
ARG RUNTIME=docker.io/library/alpine@$DG
FROM golang@sha256:$D64 AS build
ARG RUNTIME
FROM \${RUNTIME}
EOF
expect_good "F2 control: stage-local redeclaration of a pinned global default" fn_argredecl_ok
fx fn_multiarg Dockerfile <<'EOF'
ARG A=1 BASE=golang:1.21
FROM ${BASE}
EOF
expect_bad "F2 several ARG on one line" fn_multiarg from_arg_unpinned 2
fx fn_runopts scripts/a.sh <<'EOF'
#!/usr/bin/env bash
podman run --rm --log-opt max-size=10m postgres:15
docker run --dns 8.8.8.8 redis:7
podman run --rm alpine true
podman run --init --someflag=v mysql:8
podman --log-level=error run busybox:1.36
podman image pull docker.io/library/alpine:3.19
docker container run -it --rm nginx:1.25
EOF
for pair in "2 postgres:15" "3 redis:7" "4 alpine" "5 mysql:8" "6 busybox:1.36" "7 docker.io/library/alpine:3.19" "8 nginx:1.25"; do
  set -- $pair; expect_row "F2 run operand line $1" fn_runopts script_image_unpinned "$1" "$2"
done
expect_rows_total "F2 run operand rows" fn_runopts 7
fx fn_second scripts/a.sh <<EOF
#!/usr/bin/env bash
podman pull docker.io/library/alpine@$DG && podman run postgres:15 && podman run docker.io/library/alpine@$DG true
EOF
expect_row "F2 only the first run/pull of a line used to be judged: the second command is flagged" fn_second script_image_unpinned 2 postgres:15
expect_rows_total "F2 second command: the pinned ones are not flagged" fn_second 1
fx fn_quoted scripts/a.sh <<'EOF'
#!/usr/bin/env bash
podman run --rm "postgres:15" true
podman run --rm 'redis:7' true
EOF
expect_row "F2 a double-quoted operand is reported without its quotes" fn_quoted script_image_unpinned 2 postgres:15
expect_row "F2 a single-quoted operand is reported without its quotes" fn_quoted script_image_unpinned 3 redis:7
fx fn_wrapped scripts/a.sh <<'EOF'
#!/usr/bin/env bash
sudo podman run --rm postgres:15
timeout 30 docker run --rm redis:7
ssh host "podman run --rm mysql:8"
sh -c 'docker pull busybox:1.36'
x=$(podman run --rm nginx:1.25 true)
if ! podman run --rm alpine:3.19 true; then :; fi
EOF
for pair in "2 postgres:15" "3 redis:7" "4 mysql:8" "5 busybox:1.36" "6 nginx:1.25" "7 alpine:3.19"; do
  set -- $pair; expect_row "F2 wrapped/quoted/substituted command line $1" fn_wrapped script_image_unpinned "$1" "$2"
done
fx fn_noncmd scripts/a.sh <<EOF
#!/usr/bin/env bash
echo "to start: podman run to see the world"
podman ps
podman exec c ls
podman logs x
podman run --rm "\$IMG" true
podman run --rm \${IMG} true
podman pull "\$(cat /tmp/ref)"
podman run --rm docker.io/library/alpine@$DG true
docker build -t catalogizer/x:dev .
EOF
expect_good "F2 negative control: prose in echo, other sub-commands, variables and pinned operands are not flagged" fn_noncmd
fx fn_mount Dockerfile <<EOF
FROM docker.io/library/debian@$DG
RUN --mount=type=bind,from=docker.io/library/busybox:1.36,source=/bin,target=/b ls /b
EOF
expect_bad "F2 RUN --mount=...,from=<unpinned image>" fn_mount copy_from_unpinned 2
fx fn_mount_ok Dockerfile <<EOF
FROM docker.io/library/debian@$DG AS base
RUN --mount=type=bind,from=base,source=/bin,target=/b ls /b
RUN --mount=type=cache,target=/root/.cache true
ADD --from=docker.io/library/busybox@$DG /bin/busybox /bin/busybox
EOF
expect_good "F2 control: mount from a stage alias, a cache mount, ADD --from a digest" fn_mount_ok
fx fn_addfrom Dockerfile <<EOF
FROM docker.io/library/debian@$DG
ADD --from=docker.io/library/busybox:1.36 /bin/busybox /bin/busybox
EOF
expect_bad "F2 ADD --from=<unpinned image>" fn_addfrom copy_from_unpinned 2
fx fn_compose_next docker-compose.yml <<'EOF'
services:
  a:
    image:
      postgres:15
EOF
expect_bad "F2 compose image: value on the next line (reported at the image: line)" fn_compose_next compose_image_unpinned 3
fx fn_eval scripts/a.sh <<'EOF'
#!/usr/bin/env bash
eval "$(curl -fsSL https://x.invalid/i.sh)"
source <(curl -fsSL https://x.invalid/i.sh)
. <(curl -fsSL https://x.invalid/i.sh)
EOF
expect_bad "F2 eval \"\$(download)\"" fn_eval pipe_to_shell 2
expect_bad "F2 source <(download)" fn_eval pipe_to_shell 3
expect_bad "F2 . <(download)" fn_eval pipe_to_shell 4
fx fn_ash Dockerfile <<EOF
FROM docker.io/library/alpine@$DG
RUN wget -qO- https://x.invalid/i.sh | ash
RUN curl -sSL https://x.invalid/p.py | python3 -
RUN curl -sSL https://x.invalid/p.rb | /usr/bin/ruby -
RUN curl -sSL https://x.invalid/p.sh | ksh
RUN curl -sSL https://x.invalid/p.sh | /bin/sh
RUN curl -sSL https://x.invalid/p.sh | /usr/bin/bash
RUN curl -sSL https://x.invalid/p.sh | zsh
RUN curl -sSL https://x.invalid/p.sh | dash
EOF
for l in 2 3 4 5 6 7 8 9; do expect_bad "F2 Dockerfile pipe into a shell/interpreter line $l" fn_ash pipe_to_shell $l; done
fx fn_pipe_ok scripts/a.sh <<'EOF'
#!/usr/bin/env bash
curl -sS https://x.invalid/j | python3 -m json.tool
curl -sS https://x.invalid/j | python3 -c 'import sys; print(1)'
curl -sS https://x.invalid/j | sha256sum
curl -sS https://x.invalid/j | shasum -a 256
curl -sS https://x.invalid/j | ssh host cat
EOF
expect_good "F2 negative control: a download piped into json.tool, python -c, sha256sum, shasum, ssh is not a shell install" fn_pipe_ok
mkdir -p "$T/fn_bom"; printf '\xef\xbb\xbfFROM golang:1.21\n' >"$T/fn_bom/Dockerfile"
expect_bad "F2 a UTF-8 BOM before FROM" fn_bom from_unpinned 1
fx fn_skopeo scripts/a.sh <<'EOF'
#!/usr/bin/env bash
skopeo copy docker://docker.io/library/alpine:3.19 oci:/tmp/a
EOF
expect_row "F2 skopeo docker:// transport reference" fn_skopeo script_image_unpinned 2 docker.io/library/alpine:3.19
fx fn_arith scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
x=$((1<<4))
podman run --rm docker.io/library/alpine:3.19 true
echo "cat <<EOF"
podman run --rm docker.io/library/alpine:3.20 true
EOF
expect_row "F2 arithmetic << in a test script is not a here-document opener (line 3 scanned)" fn_arith script_image_unpinned 3 docker.io/library/alpine:3.19
expect_row "F2 a << inside a quoted string is not a here-document opener (line 5 scanned)" fn_arith script_image_unpinned 5 docker.io/library/alpine:3.20
fx fn_heredoc_real scripts/tests/t.sh <<'OUTER'
#!/usr/bin/env bash
cat >x <<'EOF'
podman run --rm docker.io/library/alpine:3.19 true
EOF
podman run --rm docker.io/library/alpine:3.20 true
OUTER
expect_rows_total "F2 control: a real here-document in a test script is still data; the line after it is scanned" fn_heredoc_real 1
expect_row "F2 control: the real run after the here-document is the one row" fn_heredoc_real script_image_unpinned 5 docker.io/library/alpine:3.20
fx fn_reg_ok scripts/a.sh <<'EOF'
#!/usr/bin/env bash
echo "see docker.io/library/alpine for the docs"
EOF
expect_good "F2 control: a registry path with no tag is not a tagged reference" fn_reg_ok

# --- F4 (own mutants of the rewritten scanner): each fixture below removes one behaviour of the new code
fx x_compose_next_ref docker-compose.yml <<'EOF2'
services:
  a:
    image:
      postgres:15
EOF2
expect_row "X compose: the next-line image value is the reported reference" x_compose_next_ref compose_image_unpinned 3 postgres:15
fx x_compose_next_ok docker-compose.yml <<EOF2
services:
  a:
    image:
      docker.io/library/postgres@$DG
EOF2
expect_good "X compose: a pinned next-line image value passes" x_compose_next_ok
fx x_compose_deeper_build docker-compose.yml <<'EOF2'
services:
  a:
    image: redis:7
    deploy:
      build: x
EOF2
expect_row "X compose: a build: key nested deeper than image: is not a sibling" x_compose_deeper_build compose_image_unpinned 3 redis:7
fx x_wrappers scripts/a.sh <<'EOF2'
#!/usr/bin/env bash
sudo -u root podman run --rm postgres:15
env -i FOO=1 podman run --rm redis:7
nice -n 5 docker pull mysql:8
echo "$(podman run --rm nginx:1.25 true)"
echo "`podman run --rm busybox:1.36 true`"
FOO=bar podman create --name c alpine:3.19
buildah from alpine:3.19
podman --log-level error run --rm httpd:2.4
podman run --rm >/dev/null 2>&1 traefik:v3
podman run --frobnicate 8.8.8.8 haproxy:2.9
podman run --frobnicate k=v memcached:1.6
EOF2
for pair in "2 postgres:15" "3 redis:7" "4 mysql:8" "5 nginx:1.25" "6 busybox:1.36" "7 alpine:3.19" "8 alpine:3.19" "9 httpd:2.4" "10 traefik:v3" "11 haproxy:2.9" "12 memcached:1.6"; do
  set -- $pair; expect_row "X wrapper/substitution/global-option/unknown-option line $1" x_wrappers script_image_unpinned "$1" "$2"
done
expect_rows_total "X wrappers: exactly the eleven real rows" x_wrappers 11
fx x_valueopts scripts/a.sh <<EOF2
#!/usr/bin/env bash
podman run --rm --name web --network host -u root -w /w -v /a:/b docker.io/library/alpine@$DG true
EOF2
expect_good "X known value options hide their values from the operand search (--name web is not the image)" x_valueopts
fx x_shellish_ok scripts/a.sh <<'EOF2'
#!/usr/bin/env bash
sh -c 'echo hello world'
ssh host "ls -l /tmp"
EOF2
expect_good "X control: a shell -c string without an engine command is not flagged" x_shellish_ok

# --- F3 a crash is not a violation: exit 3, no rows, a message on stderr (11.4.201(6): silence is not a verdict)
CRASH_SUT="$T/check_pins_crash.sh"
python3 -I - "$SUT" "$CRASH_SUT" <<'PYEND'
import sys
src = open(sys.argv[1]).read()
old = "def scan_dockerfile(path, text):\n"
if src.count(old) != 1:
    sys.exit(3)
open(sys.argv[2], "w").write(src.replace(old, old + "    raise RuntimeError('injected crash')\n"))
PYEND
[ -f "$CRASH_SUT" ] && ok "F3 crash copy of the script written" || bad "F3 crash copy could not be written (anchor def scan_dockerfile missing)"
COUT="$(bash "$CRASH_SUT" --root "$T/bad_from" --list 2>"$T/crash.err")"; CRC=$?
check "F3 an internal exception exits 3 (not 1)" "$CRC" "3"
check "F3 an internal exception prints no violation rows on stdout" "$COUT" ""
grep -q 'internal error' "$T/crash.err" && ok "F3 the crash is reported on stderr" || bad "F3 no 'internal error' on stderr: $(head -3 "$T/crash.err")"
bash "$CRASH_SUT" --root "$T/good_all" >/dev/null 2>&1; check "F3 negative control: the crash copy exits 0 on a tree that has a compose file only (no Dockerfile scanned)" "$?" "0"
bash "$SUT" --root "$T/good_all" >/dev/null 2>&1; check "F3 negative control: the real script exits 0 on a clean tree" "$?" "0"

# --- F4 mutation adequacy: one fixture per behaviour the reviewer's surviving mutants removed
fx m_lowerfrom Dockerfile <<'EOF'
from golang:1.21
EOF
expect_bad "F4 lowercase from keyword" m_lowerfrom from_unpinned 1
fx m_dotbash scripts/x.bash <<'EOF'
#!/usr/bin/env bash
podman run --rm postgres:15
EOF
expect_bad "F4 a .bash file is scanned" m_dotbash script_image_unpinned 2
fx m_yaml docker-compose.yaml <<'EOF'
services:
  a:
    image: postgres:15
EOF
expect_bad "F4 a compose .yaml file is scanned" m_yaml compose_image_unpinned 3
fx m_inlinedefault Dockerfile <<'EOF'
FROM ${BASE:-golang:1.21}
EOF
expect_bad "F4 inline \${VAR:-default} FROM default is judged" m_inlinedefault from_arg_unpinned 1
fx m_inlinedefault_ok Dockerfile <<EOF
FROM \${BASE:-docker.io/library/golang@$DG}
EOF
expect_good "F4 control: inline default with a digest" m_inlinedefault_ok
fx m_quotehash scripts/q.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL "https://x.invalid/i.sh" -H "X: a #b" | sh
EOF
expect_bad "F4 a # inside a quoted string does not start a comment (the pipe after it is seen)" m_quotehash pipe_to_shell 2
fx m_trailcomment scripts/q.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://x.invalid/f -o /tmp/f # | sh
EOF
expect_good "F4 control: a real trailing comment is removed" m_trailcomment
fx m_order Dockerfile <<'EOF'
ARG X=1
RUN curl -fsSL https://x.invalid/i.sh | sh
RUN true
FROM golang:1.21
EOF
ORD="$(bash "$SUT" --root "$T/m_order" --list | cut -f3 | tr '\n' ' ')"
check "F4 the output is sorted by line (the pipe at line 2 before the FROM at line 4)" "$ORD" "2 4 "
mkdir -p "$T/m_excl/third"; printf 'FROM golang:1.21\n' >"$T/m_excl/third/Dockerfile"; printf 'FROM scratch\n' >"$T/m_excl/Dockerfile"
bash "$SUT" --root "$T/m_excl" >/dev/null 2>&1; check "F4 control: a violation under third/ is found without --exclude-dir (exit 1)" "$?" "1"
bash "$SUT" --root "$T/m_excl" --exclude-dir third >/dev/null 2>&1; check "F4 --exclude-dir skips the directory in the default file set (exit 0)" "$?" "0"
bash "$SUT" --root "$T/m_excl" --exclude-dir third . >/dev/null 2>&1; check "F4 --exclude-dir skips the directory when a PATH is walked (exit 0)" "$?" "0"
bash "$SUT" --root "$T/m_excl" . >/dev/null 2>&1; check "F4 control: the walked PATH finds it without --exclude-dir (exit 1)" "$?" "1"
# --recurse: a submodule checkout's files are scanned only with --recurse
GITQ=(env GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@e.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@e.invalid git -c protocol.file.allow=always -c commit.gpgsign=false -c init.defaultBranch=main)
if command -v git >/dev/null 2>&1; then
  mkdir -p "$T/subsrc" "$T/recur"
  ( cd "$T/subsrc" && "${GITQ[@]}" init -q . && printf 'FROM golang:1.21\n' >Dockerfile && "${GITQ[@]}" add Dockerfile && "${GITQ[@]}" commit -q -m s ) >/dev/null 2>&1
  ( cd "$T/recur" && "${GITQ[@]}" init -q . && printf 'FROM scratch\n' >Dockerfile && "${GITQ[@]}" add Dockerfile && "${GITQ[@]}" commit -q -m m \
      && "${GITQ[@]}" submodule add -q "$T/subsrc" subm && "${GITQ[@]}" commit -q -m sub ) >/dev/null 2>&1
  if [ -f "$T/recur/subm/Dockerfile" ]; then
    bash "$SUT" --root "$T/recur" >/dev/null 2>&1; check "F4 submodule files are not scanned without --recurse (exit 0)" "$?" "0"
    RL="$(bash "$SUT" --root "$T/recur" --recurse --list 2>&1)"; RRC=$?
    check "F4 --recurse scans the submodule (exit 1)" "$RRC" "1"
    check "F4 --recurse reports the submodule path" "$(printf '%s' "$RL" | cut -f2)" "subm/Dockerfile"
  else bad "F4 the submodule fixture could not be created (git submodule add failed)"; fi
else bad "F4 git is required for the --recurse fixture"; fi

# ------------------------------------------------------------------ WF13 round 3 (11.4.276 structural round): the classes behind N1-N10 (docs/scripts/check_pins.md "Classes")
# Each class is exercised over its ENUMERATED members, not over the instances the reviewer reported: engine-command recognition over
# wrapper/prefix shapes and over EVERY option of the engine's own help text; parameter-expansion operators; the compose pull_policy values;
# pipe-reader stages; unreadable inputs; test-script classification; arithmetic forms. Control needles: parser needles on the option snapshot,
# a clean control next to every flagged class, and the RED/GREEN record for each class (docs/scripts/check_pins.md).
CN=0
line_case() { # <label> <script line> <want ref | none>   (the line is line 2 of scripts/a.sh)
  CN=$((CN+1)); local n="lc$CN" got
  mkdir -p "$T/$n/scripts"; printf '#!/usr/bin/env bash\n%s\n' "$2" >"$T/$n/scripts/a.sh"
  got="$(rows "$n" | cut -f1,3,4 | tr '\n\t' ';,')"
  if [ "$3" = none ]; then check "$1 (no row)" "$got" ""; else check "$1" "$got" "script_image_unpinned,2,$3;"; fi
}
pipe_case() { # <label> <script line> <yes|no>
  CN=$((CN+1)); local n="pc$CN" got
  mkdir -p "$T/$n/scripts"; printf '#!/usr/bin/env bash\n%s\n' "$2" >"$T/$n/scripts/a.sh"
  got="$(rows "$n" | cut -f1,3 | tr '\n\t' ';,')"
  if [ "$3" = no ]; then check "$1 (no row)" "$got" ""; else check "$1" "$got" "pipe_to_shell,2;"; fi
}
# --- class A: an engine command is recognised wherever it is a command, not only after a closed wrapper list (N1)
line_case "R3-A timeout with a signal option"            'timeout -s KILL 30 podman run --rm postgres:15' postgres:15
line_case "R3-A \$SUDO prefix"                             '$SUDO podman run --rm postgres:15' postgres:15
line_case "R3-A \${SUDO} prefix"                           '${SUDO} docker pull redis:7' redis:7
line_case "R3-A helper function with an argument"          'retry 3 podman pull postgres:16' postgres:16
line_case "R3-A helper function wrapper"                   'log_run podman run --rm mysql:8' mysql:8
line_case "R3-A unquoted ssh remote command"               'ssh host podman run --rm nginx:1.25' nginx:1.25
line_case "R3-A unquoted eval"                             'eval podman run --rm busybox:1.36' busybox:1.36
line_case "R3-A env with a value option"                   'env -u FOO podman run --rm alpine:3.19' alpine:3.19
line_case "R3-A sudo long value option"                    'sudo --user root podman run --rm httpd:2.4' httpd:2.4
line_case "R3-A helper with a quoted argument"             'run_remote "$HOST" podman pull haproxy:2.9' haproxy:2.9
line_case "R3-A engine in a variable \$DOCKER"             '$DOCKER run --rm memcached:1.6' memcached:1.6
line_case "R3-A engine in \${CONTAINER_ENGINE}"            '${CONTAINER_ENGINE} pull traefik:v3' traefik:v3
line_case "R3-A whole command line quoted into a helper"   'bash_wrapper "docker pull varnish:7"' varnish:7
line_case "R3-A nice with a value option"                  'nice -n 5 podman run --rm redis:6' redis:6
line_case "R3-A ionice and stdbuf stacked"                 'ionice -c 3 stdbuf -oL podman pull mongo:7' mongo:7
line_case "R3-A busybox-style chroot wrapper word"         'chroot /r podman run --rm influxdb:2' influxdb:2
line_case "R3-A control: echo of a command line"           'echo podman run --rm junk:1' none
line_case "R3-A control: grep for the words"               'grep docker run notes.txt' none
line_case "R3-A control: man page"                         'man docker run' none
line_case "R3-A control: a message that starts with the words" 'log "docker pull of foo failed"' none
line_case "R3-A control: podman ps / build / logs"         'podman ps -a; docker build -t x:1 .; podman logs c' none
line_case "R3-A control: a pinned operand through a prefix" "\$SUDO podman run --rm docker.io/library/postgres@$DG true" none
line_case "R3-A control: a bare variable engine with no verb" 'if [ -x "$DOCKER" ]; then :; fi' none
# --- class A (N5): the option tables ARE the engine help text (podman 5.7.0 parsed + the docker reference), checked option by option
ENGOPT="$HERE/engine_options.tsv"
[ -r "$ENGOPT" ] || bad "R3-A engine_options.tsv is missing"
grep -qP '^run\t--add-host\tvalue\t' "$ENGOPT" && ok "R3-A parser needle: --add-host is a value option in the snapshot" || bad "R3-A parser needle --add-host(value) missing"
grep -qP '^run\t--rm\tbool\t' "$ENGOPT" && ok "R3-A parser needle: --rm is boolean in the snapshot" || bad "R3-A parser needle --rm(bool) missing"
grep -qP '^pull\t-a\tbool\t' "$ENGOPT" && ok "R3-A parser needle: pull -a (--all-tags) is boolean in the snapshot" || bad "R3-A parser needle pull -a(bool) missing"
DUMP="$(bash "$SUT" --dump-engine-options 2>&1 | LC_ALL=C sort)"
WANT="$(awk -F'\t' '!/^#/ {print $1"\t"$2"\t"$3}' "$ENGOPT" | LC_ALL=C sort)"
check "R3-A the option tables of check_pins (value AND boolean options) equal the snapshot exactly (no missing option, no extra entry, no wrong kind)" "$(diff <(printf '%s\n' "$DUMP") <(printf '%s\n' "$WANT") | head -6 | tr '\n' '|')" ""
mkdir -p "$T/o_run/scripts" "$T/o_runb/scripts" "$T/o_pull/scripts" "$T/o_pullb/scripts" "$T/o_glob/scripts" "$T/o_globb/scripts"
{ echo '#!/usr/bin/env bash'; awk -F'\t' -v d="$D64" '!/^#/ && $1=="run" && $3=="value" {printf "podman run --rm %s val docker.io/library/postgres@sha256:%s true\n", $2, d}' "$ENGOPT"; } >"$T/o_run/scripts/a.sh"
{ echo '#!/usr/bin/env bash'; awk -F'\t' '!/^#/ && $1=="run" && $3=="bool" && $2!="--rm" {printf "podman run --rm %s postgres:15\n", $2}' "$ENGOPT"; } >"$T/o_runb/scripts/a.sh"
{ echo '#!/usr/bin/env bash'; awk -F'\t' -v d="$D64" '!/^#/ && $1=="pull" && $3=="value" {printf "podman pull %s val docker.io/library/postgres@sha256:%s\n", $2, d}' "$ENGOPT"; } >"$T/o_pull/scripts/a.sh"
{ echo '#!/usr/bin/env bash'; awk -F'\t' '!/^#/ && $1=="pull" && $3=="bool" {printf "podman pull %s postgres:15\n", $2}' "$ENGOPT"; } >"$T/o_pullb/scripts/a.sh"
{ echo '#!/usr/bin/env bash'; awk -F'\t' -v d="$D64" '!/^#/ && $1=="global" && $3=="value" {printf "podman %s val run --rm docker.io/library/postgres@sha256:%s true\n", $2, d}' "$ENGOPT"; } >"$T/o_glob/scripts/a.sh"
{ echo '#!/usr/bin/env bash'; awk -F'\t' '!/^#/ && $1=="global" && $3=="bool" && $2!="--help" {printf "podman %s run --rm postgres:15\n", $2}' "$ENGOPT"; } >"$T/o_globb/scripts/a.sh"
for pair in "o_run run value options:every run/create value option hides its value" "o_pull pull value options:every pull value option hides its value" "o_glob global value options:every global value option hides its value"; do
  n="${pair%% *}"; lab="${pair#* }"; lab="${lab#*:}"
  check "R3-A $lab (offending values: none)" "$(rows "$n" | cut -f4 | sort -u | head -5 | tr '\n' ' ')" ""
done
for pair in "o_runb run" "o_pullb pull" "o_globb global"; do
  n="${pair%% *}"; k="${pair#* }"; want="$(grep -c . "$T/$n/scripts/a.sh")"; want=$((want-1))
  check "R3-A every $k boolean option leaves the operand visible (one row per line)" "$(nrows "$n")" "$want"
done
# the snapshot is regenerated from a live podman of the same version when there is one (drift detector); otherwise an honest SKIP, never a pass
SNAPVER="$(grep -o 'podman-version-[0-9.]*' "$ENGOPT" | head -1)"
LIVEVER="$(command -v podman >/dev/null 2>&1 && podman --version 2>/dev/null | awk '{print "podman-version-"$3}')"
if [ -n "$SNAPVER" ] && [ "$SNAPVER" = "$LIVEVER" ]; then
  bash "$HERE/gen_engine_options.sh" "$T/engine_live.tsv" >/dev/null 2>&1
  check "R3-A the snapshot equals a regeneration from the live $LIVEVER help" "$(diff "$ENGOPT" "$T/engine_live.tsv" | head -4 | tr '\n' '|')" ""
else echo "SKIP: R3-A live drift check (snapshot ${SNAPVER:-none}, live ${LIVEVER:-none}): not the same podman, the snapshot comparison above still ran"; fi
# --- class B: a registry reference inside a parameter-expansion default is a reference (N2); every operator of the expansion grammar
for op in ':-' '-' ':=' '='; do
  line_case "R3-B script assignment with \${IMG${op}default}" "IMG=\"\${IMG${op}docker.io/library/postgres:15}\"" docker.io/library/postgres:15
done
line_case "R3-B ghcr default inside a command operand"     'podman run --rm ${IMG:-ghcr.io/o/tool:1.0} true' ghcr.io/o/tool:1.0
line_case "R3-B quoted default operand, short name"        'podman run --rm "${IMG:-redis:7}" true' redis:7
line_case "R3-B default operand through a prefix"          '$SUDO docker run --rm ${IMG-mysql:8}' mysql:8
line_case "R3-B control: a pinned default"                 "podman run --rm \${IMG:-docker.io/library/redis@sha256:$D64} true" none
line_case "R3-B control: a default that is another variable" 'podman run --rm ${IMG:-$OTHER} true' none
line_case "R3-B control: a bare variable operand"          'podman run --rm ${IMG} true' none
line_case "R3-B control: a registry reference after = and a quote still flagged" 'echo x --image=docker.io/library/redis:7' docker.io/library/redis:7
# --- class C: compose image next to build: x pull_policy (N3). Matrix over the policy values of the compose spec (build.md / services.md)
comp_case() { # <label> <image> <policy|-> <flag|skip>   (image: is line 4)
  CN=$((CN+1)); local n="cc$CN" got
  mkdir -p "$T/$n"; { printf 'services:\n  a:\n    build: .\n    image: %s\n' "$2"; [ "$3" = - ] || printf '    pull_policy: %s\n' "$3"; } >"$T/$n/docker-compose.yml"
  got="$(rows "$n" | cut -f1,3 | tr '\n\t' ';,')"
  if [ "$4" = skip ]; then check "$1 (skipped)" "$got" ""; else check "$1 (flagged)" "$got" "compose_image_unpinned,4;"; fi
}
comp_case "R3-C namespaced image, build, no policy"               someorg/tool:1.0 - flag
comp_case "R3-C namespaced image, build, pull_policy always"      someorg/tool:1.0 always flag
comp_case "R3-C namespaced image, build, pull_policy missing"     someorg/tool:1.0 missing flag
comp_case "R3-C namespaced image, build, pull_policy if_not_present" someorg/tool:1.0 if_not_present flag
comp_case "R3-C namespaced image, build, pull_policy daily"       someorg/tool:1.0 daily flag
comp_case "R3-C namespaced image, build, pull_policy every_24h"   someorg/tool:1.0 every_24h flag
comp_case "R3-C namespaced image, build, pull_policy from a variable" someorg/tool:1.0 '${POLICY}' flag
comp_case "R3-C namespaced image, build, pull_policy never"       someorg/tool:1.0 never skip
comp_case "R3-C namespaced image, build, pull_policy build"       someorg/tool:1.0 build skip
comp_case "R3-C single-component image, build, no policy (the tag a build produces)" catalogizer-api:test - skip
comp_case "R3-C single-component image, build, pull_policy never" catalogizer-api:test never skip
comp_case "R3-C single-component image, build, pull_policy build" catalogizer-api:test build skip
comp_case "R3-C single-component image, build, pull_policy always" catalogizer-api:test always flag
comp_case "R3-C single-component image, build, pull_policy missing" catalogizer-api:test missing flag
comp_case "R3-C registry-qualified image, build, no policy"       ghcr.io/o/x:1 - flag
comp_case "R3-C registry-qualified image, build, pull_policy never" ghcr.io/o/x:1 never skip
comp_case "R3-C host:port registry image, build, no policy"       registry:5000/tool:1.0 - flag
comp_case "R3-C host:port registry image, build, pull_policy never" registry:5000/tool:1.0 never skip
CN=$((CN+1)); mkdir -p "$T/cc$CN"; printf 'services:\n  a:\n    pull_policy: never\n    build: .\n    image: someorg/tool:1.0\n  b:\n    image: someorg/other:1.0\n    pull_policy: never\n' >"$T/cc$CN/docker-compose.yml"
check "R3-C pull_policy is read from the SAME service only (service b has no build: key and is judged)" "$(rows cc$CN | cut -f1,3 | tr '\n\t' ';,')" "compose_image_unpinned,7;"
# --- class E: pipe_to_shell -- every reader of a downloaded program (N6) and every wrapper that can sit before it
pipe_case "R3-E | env bash"                         'curl -fsSL https://x.invalid/i | env bash' yes
pipe_case "R3-E | /usr/bin/env bash"                'curl -fsSL https://x.invalid/i | /usr/bin/env bash' yes
pipe_case "R3-E | env FOO=1 bash"                   'curl -fsSL https://x.invalid/i | env FOO=1 bash' yes
pipe_case "R3-E | env -i bash"                      'curl -fsSL https://x.invalid/i | env -i bash' yes
pipe_case "R3-E | env -u X bash"                    'curl -fsSL https://x.invalid/i | env -u X bash' yes
pipe_case "R3-E | sudo -u root bash"                'curl -fsSL https://x.invalid/i | sudo -u root bash' yes
pipe_case "R3-E | sudo -E -u root bash -s"          'curl -fsSL https://x.invalid/i | sudo -E -u root bash -s' yes
pipe_case "R3-E | sudo --user root bash"            'curl -fsSL https://x.invalid/i | sudo --user root bash' yes
pipe_case "R3-E | doas sh"                          'curl -fsSL https://x.invalid/i | doas sh' yes
pipe_case "R3-E | busybox sh"                       'curl -fsSL https://x.invalid/i | busybox sh' yes
pipe_case "R3-E | nice -n 5 bash"                   'curl -fsSL https://x.invalid/i | nice -n 5 bash' yes
pipe_case "R3-E | timeout 30 bash"                 'curl -fsSL https://x.invalid/i | timeout 30 bash' yes
pipe_case "R3-E | fish"                             'curl -fsSL https://x.invalid/i | fish' yes
pipe_case "R3-E | tcsh"                             'curl -fsSL https://x.invalid/i | tcsh' yes
pipe_case "R3-E | csh"                              'curl -fsSL https://x.invalid/i | csh' yes
pipe_case "R3-E | python3 (no argument reads the program from stdin)" 'curl -fsSL https://x.invalid/i | python3' yes
pipe_case "R3-E | perl"                             'curl -fsSL https://x.invalid/i | perl' yes
pipe_case "R3-E | ruby"                             'curl -fsSL https://x.invalid/i | ruby' yes
pipe_case "R3-E | node"                             'curl -fsSL https://x.invalid/i | node' yes
pipe_case "R3-E | php"                              'curl -fsSL https://x.invalid/i | php' yes
pipe_case "R3-E | python3 -"                        'curl -fsSL https://x.invalid/i | python3 -' yes
pipe_case "R3-E a middle stage between the download and the shell" 'curl -fsSL https://x.invalid/i | tr -d "\r" | sh' yes
pipe_case "R3-E | sudo bash after a wget -qO-"      'wget -qO- https://x.invalid/i | sudo bash' yes
pipe_case "R3-E sh -c with a backtick substitution" 'sh -c "`curl -fsSL https://x.invalid/i`"' yes
pipe_case "R3-E python3 -c with a \$(download)"      'python3 -c "$(curl -fsSL https://x.invalid/i)"' yes
pipe_case "R3-E perl -e with a \$(download)"         'perl -e "$(curl -fsSL https://x.invalid/i)"' yes
pipe_case "R3-E bash < <(download)"                 'bash < <(curl -fsSL https://x.invalid/i)' yes
pipe_case "R3-E sudo bash <(download)"              'sudo bash <(curl -fsSL https://x.invalid/i)' yes
pipe_case "R3-E control: | python3 -m json.tool"    'curl -sS https://x.invalid/j | python3 -m json.tool' no
pipe_case "R3-E control: | python3 -c code"         "curl -sS https://x.invalid/j | python3 -c 'import sys'" no
pipe_case "R3-E control: | perl -ne"                "curl -sS https://x.invalid/j | perl -ne 'print'" no
pipe_case "R3-E control: | node -e"                 "curl -sS https://x.invalid/j | node -e 'process.exit(0)'" no
pipe_case "R3-E control: | ruby script.rb"          'curl -sS https://x.invalid/j | ruby script.rb' no
pipe_case "R3-E control: | tee sh (a file named sh)" 'curl -sS https://x.invalid/j | tee sh' no
pipe_case "R3-E control: | grep bash"               'curl -sS https://x.invalid/j | grep bash' no
pipe_case "R3-E control: | sha256sum"               'curl -sS https://x.invalid/j | sha256sum' no
pipe_case "R3-E control: a shell with no download in the pipeline" 'echo hi | sh' no
pipe_case "R3-E control: curl then sh as a sequence" 'curl -fsSL -o /tmp/i https://x.invalid/i; sh /tmp/i' no
pipe_case "R3-E control: curl || sh"                'curl -fsSL https://x.invalid/i || sh' no
pipe_case "R3-E control: curl | tee f && sh f"      'curl -fsSL https://x.invalid/i | tee /tmp/f && sh /tmp/f' no
# --- class F: an input that cannot be read cannot be judged: exit 3, never "0 violations" (N7)
mkdir -p "$T/unr"; printf 'FROM golang:1.21\n' >"$T/unr/Dockerfile"; chmod 000 "$T/unr/Dockerfile"
if [ -r "$T/unr/Dockerfile" ]; then echo "SKIP: R3-F unreadable-file check (this user can read a mode-000 file, e.g. root)"
else
  UOUT="$(bash "$SUT" --root "$T/unr" 2>"$T/unr.err")"; URC=$?
  check "R3-F an unreadable Dockerfile exits 3 (not 0)" "$URC" "3"
  check "R3-F an unreadable Dockerfile prints no 'N violations' summary" "$(printf '%s' "$UOUT" | grep -c 'violations in')" "0"
  grep -q 'Dockerfile' "$T/unr.err" && grep -q 'check_pins: cannot read' "$T/unr.err" && ok "R3-F stderr names the unreadable file" || bad "R3-F stderr does not name the file: $(head -2 "$T/unr.err")"
  chmod 600 "$T/unr/Dockerfile"; bash "$SUT" --root "$T/unr" >/dev/null 2>&1; check "R3-F control: the same file, readable, is flagged (exit 1)" "$?" "1"
fi
# --- class G: test-script classification and here-document detection (N8, N10)
fx g_opnamed scripts/test_infra_up.sh <<'EOF'
#!/usr/bin/env bash
IMG=docker.io/library/postgres:16
podman run --rm "$IMG" true
EOF
expect_row "R3-G a test_*-named script OUTSIDE a tests directory is operational code: the registry literal is flagged" g_opnamed script_image_unpinned 2 docker.io/library/postgres:16
fx g_opexport scripts/test_up.sh <<'EOF'
#!/usr/bin/env bash
export IMAGE=docker.io/library/redis:7
podman run --rm "$IMAGE" true
EOF
expect_row "R3-G export IMAGE=<registry ref> in a test_*-named operational script is flagged" g_opexport script_image_unpinned 2 docker.io/library/redis:7
fx g_intests scripts/tests/test_up.sh <<'EOF'
#!/usr/bin/env bash
IMG=docker.io/library/postgres:16
podman run --rm "$IMG" true
EOF
expect_good "R3-G control: the same lines inside a tests directory are fixture data" g_intests
fx g_arith1 scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
x=$(( (1) << 4 ))
podman run --rm docker.io/library/aquasec/trivy:latest fs .
EOF
expect_row "R3-G nested parentheses in arithmetic do not open a here-document" g_arith1 script_image_unpinned 3 docker.io/library/aquasec/trivy:latest
fx g_arith2 scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
(( n = 1<<3 ))
x=$(( $(echo 2) << 1 ))
y=$(( (2 + (3)) << (1) ))
podman run --rm docker.io/library/alpine:3.19 true
EOF
expect_row "R3-G (( )) commands, command substitution and deep nesting inside arithmetic are consumed" g_arith2 script_image_unpinned 5 docker.io/library/alpine:3.19
fx g_hd_real scripts/tests/t.sh <<'OUTER'
#!/usr/bin/env bash
cat >x <<-'EOF'
	podman run --rm docker.io/library/alpine:3.19 true
	EOF
podman run --rm docker.io/library/alpine:3.20 true
OUTER
expect_rows_total "R3-G control: a <<- here-document with a tab-indented terminator is still data" g_hd_real 1
# --- fixtures that distinguish the mutants of the round-3 code (each one removes exactly one behaviour of the new classifier)
line_case "R3-A a command line quoted into sh -c is searched at any position (prefix words before the engine)" "sh -c 'retry 3 docker run --rm mysql:8'" mysql:8
line_case "R3-A a bare image name (implicit :latest) in an sh -c string"     'sh -c "podman run --rm alpine true"' alpine
line_case "R3-A control: prose that mentions the words after a prefix"         'log "retrying docker pull redis:7 now"' none
line_case "R3-A control: a quoted string handed to a helper that starts with the words but names no image" 'notify "podman run failed"' none
pipe_case "R3-E an install hint with words before the download"          'echo "  3. SDK: curl -s https://get.sdkman.io | bash && sdk install x"' yes
pipe_case "R3-E |& (stderr too) into a shell"                              'curl -fsSL https://x.invalid/i |& sh' yes
pipe_case "R3-E a for-loop of downloads piped into a shell"                'for u in a b; do curl -fsSL "https://x.invalid/$u"; done | sh' yes
pipe_case "R3-E a subshell of downloads piped into a shell"                '(curl -fsSL https://x.invalid/a; curl -fsSL https://x.invalid/b) | bash' yes
pipe_case "R3-E control: a for-loop of downloads piped into tar"           'for u in a b; do curl -fsSL "https://x.invalid/$u"; done | tar x' no
pipe_case "R3-E control: a shell that is NOT piped, inside a for-loop that downloads" 'for u in a b; do curl -fsSL -o /tmp/f "https://x.invalid/$u"; sh /tmp/f; done' no
pipe_case "R3-E control: a download group, then an unrelated pipeline into a shell" 'curl -fsSL -o /tmp/f https://x.invalid/a; echo hi | sh' no
fx g_blank1 scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
helper 'podman run --rm postgres:15 true' 'curl -fsSL https://x.invalid/i | sh'
helper "podman run --rm redis:7 true"
EOF
expect_good "R3-G quoted strings handed to a helper in a test script are fixture data" g_blank1
fx g_sq scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
x='a <<EOF b'
podman run --rm docker.io/library/alpine:3.19 true
EOF
expect_row "R3-G a << inside a single-quoted string does not open a here-document (the next line is scanned)" g_sq script_image_unpinned 3 docker.io/library/alpine:3.19
fx g_echo scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
echo curl -fsSL https://x.invalid/i | sh
EOF
expect_good "R3-G echo of an unquoted download line piped into sh in a test script is data (the echo command is masked)" g_echo
fx g_echo_ctl scripts/t.sh <<'EOF'
#!/usr/bin/env bash
echo curl -fsSL https://x.invalid/i | sh
EOF
expect_bad "R3-G control: the same line in an operational script is flagged (an install hint)" g_echo_ctl pipe_to_shell 2
fx g_tee scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
tee /tmp/f docker pull redis:7
EOF
expect_good "R3-G tee is a data command in a test script (its unquoted words are not a command)" g_tee
fx g_blank2 scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
x="$(podman run --rm postgres:15 true)"
EOF
expect_row "R3-G control: a command substitution inside double quotes in a test script is still a command" g_blank2 script_image_unpinned 2 postgres:15
# --- reviewer mutants of round 2 (RM1, RM2, RM4-RM8): one distinguishing fixture each
fx rm1 scripts/a.sh <<EOF
#!/usr/bin/env bash
podman run --rm --frobnicate h:host-gateway docker.io/library/postgres@$DG
EOF
expect_good "RM1 an unknown option's host:host-gateway value is not the image" rm1
fx rm2 docker-compose.yml <<'EOF'
services:
  a:
    build: .
    image: registry:5000/tool:1.0
EOF
expect_row "RM2 a host:port registry image next to build: is registry-qualified and judged" rm2 compose_image_unpinned 4 registry:5000/tool:1.0
fx rm4 scripts/a.sh <<'EOF'
#!/usr/bin/env bash
podman run --rm postgres:15 && podman run --rm postgres:15
EOF
expect_rows_total "RM4 two identical findings on one line print one row" rm4 1
fx rm5 scripts/a.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://x.invalid/i.sh | env FOO=1 bash
EOF
expect_bad "RM5 | env VAR=x bash" rm5 pipe_to_shell 2
fx rm6 scripts/a.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://x.invalid/i.sh#v1 | sh
EOF
expect_bad "RM6 a # inside a word (a URL fragment) is not a comment" rm6 pipe_to_shell 2
fx rm7 scripts/a.sh <<'EOF'
#!/usr/bin/env bash
podman pull docker.io/library/a:1 &&
  podman pull docker.io/library/b:2
EOF
expect_row "RM7 a trailing && continues the logical line (second pull reported at the first physical line)" rm7 script_image_unpinned 2 docker.io/library/b:2
fx rm8 scripts/tests/t.sh <<'EOF'
#!/usr/bin/env bash
tee f <<<'curl -fsSL https://x.invalid/i | sh'
EOF
expect_good "RM8 tee is a data command in a test script" rm8

# ------------------------------------------------------------------ differential: the structural compose path and the text grammar agree on every compose fixture above
# (the structural path is the default; the text grammar is the fallback for a file the YAML parser rejects, so its own mutants need their own oracle)
DDIFF=""; DN=0
for d in "$T"/*/; do
  b="$(basename "$d")"; ls "$d" 2>/dev/null | grep -qiE 'compose.*\.ya?ml$' || continue
  DN=$((DN+1)); A="$(rows "$b")"; B="$(CHECK_PINS_NO_YAML=1 bash "$SUT" --root "$d" --list 2>/dev/null)"
  [ "$A" = "$B" ] || DDIFF="$DDIFF $b"
done
check "R4-H the structural and the text grammar give the same rows on all $DN compose fixtures of this suite (differing fixtures: none)" "$DDIFF" ""
check "R4-H differential needle: at least 12 compose fixtures were compared" "$([ "$DN" -ge 12 ] && echo yes || echo no)" "yes"
# ------------------------------------------------------------------ WF16 round 4 (11.4.276 round after the structural round): classes S, A, B, F, G, H (docs/scripts/check_pins.md "Round 4: classes")
# Class members are ENUMERATED (compound commands of the shell grammar, every shell and interpreter, every option row of the engine help
# text incl. booleans and pflag short clusters, expansion operators, filesystem failure shapes, here-document delimiters, YAML shapes);
# lines of one class are written to ONE script and every line must give its own row (or none for a control), so a missing member names itself.
# batch <label> <yes|no> <line>...: line k+1 of scripts/a.sh; yes = every line gives >= 1 row, no = no line gives a row
batch() {
  local lab="$1" mode="$2"; shift 2; CN=$((CN+1)); local n="bt$CN" i=1 miss="" got hit l
  mkdir -p "$T/$n/scripts"; { echo '#!/usr/bin/env bash'; for l in "$@"; do printf '%s\n' "$l"; done; } >"$T/$n/scripts/a.sh"
  got=" $(rows "$n" | cut -f3 | sort -n | uniq | tr '\n' ' ')"
  for l in "$@"; do
    i=$((i+1)); case "$got" in *" $i "*) hit=1;; *) hit=0;; esac
    if [ "$mode" = yes ] && [ "$hit" = 0 ]; then miss="$miss [$l]"; fi
    if [ "$mode" = no ] && [ "$hit" = 1 ]; then miss="$miss [$l]"; fi
  done
  check "$lab ($# lines, every one $mode; offending lines: none)" "$miss" ""
}
# batch_ref <label> <want reference> <line>...: every line gives exactly one row with this reference, at its own line number
batch_ref() {
  local lab="$1" want="$2"; shift 2; CN=$((CN+1)); local n="br$CN" i=1 exp="" l
  mkdir -p "$T/$n/scripts"; { echo '#!/usr/bin/env bash'; for l in "$@"; do printf '%s\n' "$l"; done; } >"$T/$n/scripts/a.sh"
  for l in "$@"; do i=$((i+1)); exp="$exp$i:$want;"; done
  check "$lab ($# lines)" "$(rows "$n" | awk -F'\t' '{printf "%s:%s;", $3, $4}')" "$exp"
}
DL='curl -fsSL https://x.invalid/i'
# --- class S: compound commands of the shell grammar (bash(1) "Compound Commands"): a download inside any of them, piped as a whole (I1)
batch "R4-S compound commands piped into a shell" yes \
  "for u in a b; do $DL; done | sh" \
  "while read -r u; do $DL; done < urls.txt | sh" \
  "until $DL; do sleep 1; done | sh" \
  "if true; then $DL; fi | bash" \
  "case x in x) $DL ;; esac | sh" \
  "select u in a b; do $DL; break; done | sh" \
  "{ $DL; } | sh" \
  "( $DL ) | sh" \
  "{ for u in a; do $DL; done; } | sh" \
  "( { $DL; } ) | sh" \
  "if true; then for u in a; do $DL; done; fi | sh" \
  "for ((i=0;i<2;i++)); do $DL; done | sh" \
  "for u in a b; do $DL; done |& sh" \
  "if true; then $DL; else true; fi | sh" \
  "if false; then true; elif true; then $DL; fi | sh" \
  "while true; do $DL; done | sudo bash" \
  "{ $DL; } | env -i bash -s" \
  "case x in (x) $DL ;; esac | sh" \
  "if true; then { $DL; }; fi | sh" \
  "while read u; do { $DL; }; done | sh" \
  "( $DL; echo done ) | sh" \
  "{ $DL; echo done; } | sh" \
  "for u in a; do $DL; echo x; done | sh"
batch "R4-S control: the same compound commands piped into a non-reader, or a shell that is not piped" no \
  "for u in a b; do $DL; done | tar x" \
  "{ $DL; } | tar x" \
  "( $DL ) | tar x" \
  "if true; then $DL; fi | tee /tmp/f" \
  "case x in x) $DL ;; esac | sha256sum" \
  "while read u; do $DL -o /tmp/f; sh /tmp/f; done < urls.txt" \
  "for u in a b; do $DL; sh /tmp/x; done" \
  "if true; then $DL -o /tmp/f; fi; echo hi | sh" \
  "{ $DL -o /tmp/f; }; echo hi | sh" \
  "case x in x) $DL -o f ;; esac; echo hi | sh" \
  "( $DL -o /tmp/f ); echo x | sh" \
  "until $DL -o /tmp/f; do sleep 1; done; echo ok | sh" \
  "echo hi | { sh; }" \
  "for u in a b; do echo \$u; done | sh" \
  "for u in a b; do $DL -o /tmp/f; sh -s; done" \
  "{ $DL -o /tmp/f; bash; }" \
  "( $DL -o /tmp/f; bash -s )"
# the same compound forms in a Dockerfile RUN (the reviewer's I1 probe: round 2 reported 2 rows, round 3 none)
fx r4_df Dockerfile <<EOF
FROM docker.io/library/debian@sha256:$D64
RUN { curl -fsSL https://x.invalid/i; } | sh
RUN while read -r u; do curl -fsSL "\$u"; done < /urls | sh
RUN if true; then curl -fsSL https://x.invalid/i; fi | bash
RUN case x in x) curl -fsSL https://x.invalid/i ;; esac | sh
RUN until curl -fsSL https://x.invalid/i; do sleep 1; done | sh
RUN for u in a b; do curl -fsSL https://x.invalid/\$u; done | sh
RUN curl -fsSL -o /tmp/f https://x.invalid/f && { sh /tmp/f; }
RUN { curl -fsSL https://x.invalid/i; } | tar x
EOF
check "R4-S Dockerfile RUN lines with a compound command piped into a shell: one row each, the two controls give none" "$(rows r4_df | cut -f1,3 | tr '\n\t' ';,')" "pipe_to_shell,2;pipe_to_shell,3;pipe_to_shell,4;pipe_to_shell,5;pipe_to_shell,6;pipe_to_shell,7;"
# --- class S: every shell word, every interpreter, with and without a wrapper or a path
SHELL_LIST="sh bash zsh dash ash ksh mksh csh tcsh fish rbash"
INTERP_LIST="python python3 python3.12 perl ruby node nodejs php lua deno bun Rscript"
SL=(); for s in $SHELL_LIST; do SL+=("$DL | $s" "$DL | /usr/bin/$s" "$DL | sudo $s -s" "$DL | env FOO=1 $s"); done
batch "R4-S every shell word is a program reader (also by path, behind sudo, behind env)" yes "${SL[@]}"
IL=(); for s in $INTERP_LIST; do IL+=("$DL | $s" "$DL | /usr/bin/$s" "$DL | sudo $s -"); done
batch "R4-S every stdin interpreter is a program reader when it has no script and no inline code" yes "${IL[@]}"
batch "R4-S shell/interpreter options that take a VALUE do not hide the stdin program" yes \
  "$DL | bash -o pipefail" "$DL | bash +o history" "$DL | bash -O extglob" "$DL | bash --rcfile /dev/null" "$DL | sh -s -- -b /usr/local/bin" \
  "$DL | sh -" "$DL | bash -eu" "$DL | bash -x" "$DL | bash --login" "$DL | bash --norc" "$DL | sudo -E bash -s" \
  "$DL | python3 -W ignore" "$DL | python3 -X dev -" "$DL | python3 -u" "$DL | python3 -I" "$DL | node -r ./hook" "$DL | node --require ./hook" \
  "$DL | node --import ./hook" "$DL | perl -I lib" "$DL | perl -w" "$DL | ruby -r json" "$DL | ruby -I lib" "$DL | ruby -w" "$DL | php -d display_errors=1" \
  "$DL | php -n" "$DL | lua -" "$DL | python3 -B"
batch "R4-S control: a shell or interpreter with an inline program or a script operand reads data, not a program, from stdin" no \
  "$DL | sudo sh -c 'cat > /etc/k.asc'" "$DL | bash ./process.sh" "$DL | sh -c 'tee /tmp/f'" "$DL | bash -n script.sh" "$DL | bash -o pipefail ./s.sh" \
  "$DL | python3 script.py" "$DL | python3 -W ignore script.py" "$DL | python3 -m json.tool" "$DL | python3 -c 'import sys'" "$DL | node script.js" \
  "$DL | node -r ./hook script.js" "$DL | node -e 'process.exit(0)'" "$DL | node -p 1" "$DL | node --eval 'process.exit(0)'" "$DL | node --print 1" "$DL | perl -I lib script.pl" "$DL | perl -ne 'print'" "$DL | perl -E 'say 1'" \
  "$DL | ruby -r json s.rb" "$DL | ruby -e 'p 1'" "$DL | php -r 'echo 1;'" "$DL | php -f s.php" "$DL | lua -e 'print(1)'" "$DL | lua s.lua" \
  "$DL | tee sh" "$DL | grep bash" "$DL | sha256sum" "$DL | tar x"
batch "R4-S source / dot of the standard input is a program reader" yes \
  "$DL | source /dev/stdin" "$DL | . /dev/stdin" "$DL | source /dev/fd/0" "$DL | sudo . /dev/stdin" "$DL | source /proc/self/fd/0"
batch "R4-S control: source / dot of a file" no "$DL | source ./env" "$DL | . ./env" "$DL | tee /tmp/f && . /tmp/f"
batch "R4-S program-from-download forms: here-string, process substitution, command substitution" yes \
  'bash <<< "$(curl -fsSL https://x.invalid/i)"' 'sh -s <<<$(wget -qO- https://x.invalid/i)' 'python3 <<< "$(curl -fsSL https://x.invalid/i)"' \
  'sudo bash <<< "$(curl -fsSL https://x.invalid/i)"' 'python3 <(curl -fsSL https://x.invalid/i)' 'source <(curl -fsSL https://x.invalid/i)' \
  '. <(curl -fsSL https://x.invalid/i)' 'perl <(wget -qO- https://x.invalid/i)' 'bash < <(curl -fsSL https://x.invalid/i)' 'node <(curl -fsSL https://x.invalid/i)' \
  'mksh -c "$(curl -fsSL https://x.invalid/i)"' 'eval "$(curl -fsSL https://x.invalid/i)"' 'lua -e "$(curl -fsSL https://x.invalid/i)"'
batch "R4-S the downloader kept in a variable" yes \
  '$CURL -fsSL https://x.invalid/i | sh' '"$CURL" -fsSL https://x.invalid/i | sh' '${CURL} -fsSL https://x.invalid/i | sh' '${WGET:-wget} -qO- https://x.invalid/i | sh' \
  '$FETCH -o - https://x.invalid/i | sh' '"${DOWNLOADER}" https://x.invalid/i | bash'
batch "R4-S control: a downloader variable with no reader, a reader fed by a variable" no \
  '$CURL -fsSL https://x.invalid/i | tar x' '$SOME_TOOL -fsSL https://x.invalid/i | sh'
# --- class S: the compared grammar tables (one grammar, enumerated here from the man pages / live help of the real tools; sudo/doas man pages
# were read, doas is not installed: UNCONFIRMED against a live doas; the env row is the uutils/GNU union of `env --help`)
GRAM="$(bash "$SUT" --dump-grammar 2>&1 | LC_ALL=C sort)"
GWANT="$(LC_ALL=C sort <<'GEOF'
closer	}
closer	done
closer	esac
closer	fi
dlword	curl
dlword	fetch
dlword	wget
interp	Rscript
interp	bun
interp	deno
interp	lua
interp	node
interp	perl
interp	php
interp	python
interp	ruby
opener	case
opener	for
opener	if
opener	select
opener	until
opener	while
opener	{
shell	ash
shell	bash
shell	csh
shell	dash
shell	fish
shell	ksh
shell	mksh
shell	rbash
shell	sh
shell	tcsh
shell	zsh
stdinpath	/dev/fd/0
stdinpath	/dev/stdin
stdinpath	/proc/self/fd/0
wrapper	!
wrapper	builtin
wrapper	busybox
wrapper	command
wrapper	do
wrapper	doas
wrapper	elif
wrapper	else
wrapper	env
wrapper	exec
wrapper	if
wrapper	ionice
wrapper	nice
wrapper	nohup
wrapper	setsid
wrapper	stdbuf
wrapper	sudo
wrapper	then
wrapper	time
wrapper	timeout
wrapper	until
wrapper	watch
wrapper	while
wrapper	xargs
wrapper	{
wrapopt	doas	-C
wrapopt	doas	-u
wrapopt	env	--argv0
wrapopt	env	--chdir
wrapopt	env	--file
wrapopt	env	--split-string
wrapopt	env	--unset
wrapopt	env	-C
wrapopt	env	-S
wrapopt	env	-a
wrapopt	env	-f
wrapopt	env	-u
wrapopt	ionice	--class
wrapopt	ionice	--classdata
wrapopt	ionice	-P
wrapopt	ionice	-c
wrapopt	ionice	-n
wrapopt	ionice	-p
wrapopt	ionice	-u
wrapopt	nice	--adjustment
wrapopt	nice	-n
wrapopt	stdbuf	--error
wrapopt	stdbuf	--input
wrapopt	stdbuf	--output
wrapopt	stdbuf	-e
wrapopt	stdbuf	-i
wrapopt	stdbuf	-o
wrapopt	sudo	--chdir
wrapopt	sudo	--chroot
wrapopt	sudo	--close-from
wrapopt	sudo	--command-timeout
wrapopt	sudo	--group
wrapopt	sudo	--host
wrapopt	sudo	--other-user
wrapopt	sudo	--prompt
wrapopt	sudo	--role
wrapopt	sudo	--type
wrapopt	sudo	--user
wrapopt	sudo	-C
wrapopt	sudo	-D
wrapopt	sudo	-R
wrapopt	sudo	-T
wrapopt	sudo	-U
wrapopt	sudo	-g
wrapopt	sudo	-h
wrapopt	sudo	-p
wrapopt	sudo	-r
wrapopt	sudo	-t
wrapopt	sudo	-u
wrapopt	timeout	--kill-after
wrapopt	timeout	--signal
wrapopt	timeout	-k
wrapopt	timeout	-s
GEOF
)"
check "R4-S the grammar tables of check_pins (wrappers, wrapper value options, shells, interpreters, compound openers/closers, downloaders, stdin paths) equal the enumerated ground truth" "$(diff <(printf '%s\n' "$GRAM") <(printf '%s\n' "$GWANT") | head -6 | tr '\n' '|')" ""
# --- class A: the engine option grammar uses BOTH halves of the help text (value AND boolean options), pflag short clusters, `--`, array expansions (I4)
ENG_RUN_BOOL="$(awk -F'\t' '!/^#/ && $1=="run" && $3=="bool" && $2!="--rm" && $2!="--help" {print $2}' "$ENGOPT")"
ENG_PULL_BOOL="$(awk -F'\t' '!/^#/ && $1=="pull" && $3=="bool" {print $2}' "$ENGOPT")"
ENG_GLOB_BOOL="$(awk -F'\t' '!/^#/ && $1=="global" && $3=="bool" && $2!="--help" && $2!="--version" && $2!="-v" {print $2}' "$ENGOPT")"
EL=(); for o in $ENG_RUN_BOOL; do EL+=("podman run --rm $o \${IMG:=postgres:15}" "docker run $o \${IMG:-postgres:15} true"); done
batch_ref "R4-A every run/create boolean option leaves an expansion operand visible (an expansion operand is not the value of an unknown option)" postgres:15 "${EL[@]}"
EL=(); for o in $ENG_PULL_BOOL; do EL+=("podman pull $o \${IMG:=postgres:15}"); done
batch_ref "R4-A every pull boolean option leaves an expansion operand visible" postgres:15 "${EL[@]}"
EL=(); for o in $ENG_GLOB_BOOL; do EL+=("podman $o run --rm \${IMG:=postgres:15}"); done
batch_ref "R4-A every global boolean option leaves an expansion operand visible" postgres:15 "${EL[@]}"
EL=(); for o in $(awk -F'\t' '!/^#/ && $1=="run" && $3=="value" && $2 ~ /^--/ {print $2}' "$ENGOPT"); do EL+=("podman run --rm $o=v \${IMG:=postgres:15}"); done
batch_ref "R4-A every long value option with an attached =value leaves the operand visible" postgres:15 "${EL[@]}"
# short flags: bools and value letters come from the snapshot; podman parses a cluster left to right, the FIRST value flag takes the rest of
# the cluster (attached) or the next word (ground truth: fix-r4-podman-ground-truth.txt, `podman run --pull=never -dp 127.0.0.1:3999:3999 <image>` names <image>)
SB="$(awk -F'\t' '!/^#/ && $1=="run" && $3=="bool" && length($2)==2 && $2!="-q" {print substr($2,2)}' "$ENGOPT")"
SV="$(awk -F'\t' '!/^#/ && $1=="run" && $3=="value" && length($2)==2 {print substr($2,2)}' "$ENGOPT")"
sval() { case "$1" in p) echo 127.0.0.1:3999:3999;; v) echo /tmp:/x;; m) echo 512m;; e) echo A=b;; w) echo /src;; u) echo root;; l) echo k=v;; c) echo 5;; h) echo host;; a) echo stdout;; *) echo val;; esac; }
EL=(); for b in $SB; do for c in $SB; do [ "$b" = "$c" ] || EL+=("podman run -$b$c postgres:15"); done; done
for b in $SB; do for v in $SV; do V="$(sval "$v")"; EL+=("podman run -$b$v $V postgres:15" "docker run -$b$v$V postgres:15" "podman run -$b$v $V \${IMG:=postgres:15}" "docker run --rm -$b$v=$V postgres:15"); done; done
EL+=("podman run -itd postgres:15" "docker run -dit postgres:15" "podman run -dti -p 80:80 postgres:15" "docker run -itw /src postgres:15 go test ./..." "docker run -dm 512m postgres:15" "docker run -dp 127.0.0.1:3000:3000 postgres:15" "docker run -dv /data:/data postgres:15")
batch_ref "R4-A pflag short-flag clusters (boolean letters, then a value letter with an attached or a separate value)" postgres:15 "${EL[@]}"
batch_ref "R4-A pull clusters" postgres:15 "podman pull -aq postgres:15" "podman pull -qa postgres:15" "docker pull -q postgres:15" "podman pull -q \${IMG:=postgres:15}"
batch_ref "R4-A -- ends the options: the next word is the operand, even when it is shaped like an option value" postgres:15 \
  "podman run --rm -- postgres:15 true" "podman run -- postgres:15" "podman run --rm -d -- postgres:15 sleep 1" "docker run -dp 80:80 -- postgres:15"
batch_ref "R4-A an array or positional expansion before the image is an option list, not the image" postgres:15 \
  'podman run --rm "${ARGS[@]}" postgres:15' 'docker run $@ postgres:15' 'docker run "$@" postgres:15' 'podman run --rm ${OPTS[*]} postgres:15'
batch "R4-A control: prose handed unquoted to a logging helper names no image" no \
  'warn podman pull failed for redis' 'die docker run returned error' 'error podman pull of redis failed' 'log docker pull redis:7 failed' 'info podman run exited'
line_case "R4-A a quoted message handed to an UNKNOWN helper that does not start with the engine names no image" 'handle_event "retrying docker pull redis:7 now"' none
line_case "R4-A control: the same words, starting with the engine, ARE a command line handed to the helper" 'handle_event "docker pull redis:7"' redis:7
batch_ref "R4-A control: a helper that RUNS the command is still searched" postgres:16 'retry 3 podman pull postgres:16' 'log_run podman pull postgres:16' 'run_remote "$H" podman pull postgres:16'
# --- class B: every parameter-expansion operator, nested
line_case "R4-B \${V:+alt} as an operand"                 'podman run --rm ${IMG:+redis:7} true' redis:7
line_case "R4-B \${V+alt} as an operand"                  'podman run --rm ${IMG+redis:7} true' redis:7
line_case "R4-B nested default as an operand"             'podman run --rm ${A:-${B:-postgres:15}} true' postgres:15
line_case "R4-B nested := inside :="                      'podman run --rm ${A:=${B:=redis:7}} true' redis:7
line_case "R4-B nested default with a registry host"      'podman run --rm ${A:-${B:-docker.io/library/redis:7}} true' docker.io/library/redis:7
line_case "R4-B control: a nested expansion whose innermost default is a variable" 'podman run --rm ${A:-${B:-$C}} true' none
line_case "R4-B control: \${V:?message} names no image"   'podman run --rm ${IMG:?} true' none
for h in k8s.gcr.io/pause:3.9 us.gcr.io/p/i:1 eu.gcr.io/p/i:1 nvcr.io/nvidia/cuda:12.4 index.docker.io/library/redis:7 registry-1.docker.io/library/redis:7 registry.k8s.io/pause:3.9; do
  line_case "R4-B registry host $h as a literal"           "IMG=$h" "$h"
done
line_case "R4-B control: the same registry hosts pinned"  "IMG=nvcr.io/nvidia/cuda@sha256:$D64" none
# --- class F: every filesystem failure shape reads as exit 3, never as "0 violations" (m1)
mkdir -p "$T/unr2/sub"; printf 'FROM golang:1.21\n' >"$T/unr2/sub/Dockerfile"; chmod 000 "$T/unr2/sub"
if [ -r "$T/unr2/sub" ]; then echo "SKIP: R4-F unreadable-directory checks (this user can read a mode-000 directory, e.g. root)"
else
  U2="$(bash "$SUT" --root "$T/unr2" 2>"$T/unr2.err")"; U2RC=$?
  check "R4-F an unreadable directory (non-git root) exits 3" "$U2RC" "3"
  check "R4-F ... and prints no 'N violations' summary" "$(printf '%s' "$U2" | grep -c 'violations in')" "0"
  grep -q 'check_pins: cannot read' "$T/unr2.err" && ok "R4-F stderr names the failure" || bad "R4-F stderr: $(head -2 "$T/unr2.err")"
  bash "$SUT" --root "$T/unr2" sub >/dev/null 2>&1; check "R4-F an unreadable directory given as a PATH argument exits 3" "$?" "3"
  bash "$SUT" --root "$T/unr2" sub/Dockerfile >/dev/null 2>&1; check "R4-F a file below an unreadable directory given as a PATH argument exits 3" "$?" "3"
  chmod 755 "$T/unr2/sub"; bash "$SUT" --root "$T/unr2" >/dev/null 2>&1; check "R4-F control: the same tree readable is flagged (exit 1)" "$?" "1"
fi
mkdir -p "$T/unr3/sub"; printf 'FROM golang:1.21\n' >"$T/unr3/sub/Dockerfile"; chmod 644 "$T/unr3/sub"
if [ -r "$T/unr3/sub/Dockerfile" ]; then echo "SKIP: R4-F unsearchable-directory checks (this user can search a mode-644 directory, e.g. root)"
else
  bash "$SUT" --root "$T/unr3" >/dev/null 2>&1; check "R4-F an unsearchable (mode 644) directory exits 3 (non-git root)" "$?" "3"
  chmod 755 "$T/unr3/sub"
fi
mkdir -p "$T/unr4/sub"; printf 'FROM golang:1.21\n' >"$T/unr4/sub/Dockerfile"; git -C "$T/unr4" init -q 2>/dev/null; git -C "$T/unr4" add -A 2>/dev/null; chmod 644 "$T/unr4/sub"
if [ -r "$T/unr4/sub/Dockerfile" ] || ! git -C "$T/unr4" ls-files | grep -q Dockerfile; then echo "SKIP: R4-F git unsearchable-directory check (searchable as this user, or git unavailable)"
else
  bash "$SUT" --root "$T/unr4" >/dev/null 2>&1; check "R4-F a tracked file below an unsearchable directory exits 3 (git root)" "$?" "3"
  chmod 755 "$T/unr4/sub"; bash "$SUT" --root "$T/unr4" >/dev/null 2>&1; check "R4-F control: the same tracked tree searchable is flagged" "$?" "1"
fi
# --- class G: here-document delimiter grammar (the delimiter word is any shell word, quoted or not)
HG=0
hd_case() { # <opener text> <terminator line>
  HG=$((HG+1)); local n="hg$HG"; mkdir -p "$T/$n/scripts/tests"
  { echo '#!/usr/bin/env bash'; printf 'cat >x <<%s\n' "$1"; echo 'podman run --rm docker.io/library/alpine:3.19 true'; printf '%s\n' "$2"; echo 'podman run --rm docker.io/library/alpine:3.20 true'; } >"$T/$n/scripts/tests/t.sh"
  check "R4-G here-document <<$1 ends at its own delimiter (the body is data, the line after it is code)" "$(rows "$n" | cut -f3,4 | tr '\n\t' ';,')" "5,docker.io/library/alpine:3.20;"
}
hd_case 'EOF-X' 'EOF-X'
hd_case '\EOF' 'EOF'
hd_case '"END OF"' 'END OF'
hd_case "'A.B'" 'A.B'
hd_case 'E.O.F' 'E.O.F'
hd_case '-"EOF-Y"' 'EOF-Y'
hd_case '-EOF_Z' 'EOF_Z'
# --- class H: compose structure (quoted key, flow mapping, a file not named compose, YAML the parser rejects)
hcase() { # <label> <file> <want line|none> <content...> ; the content is read from stdin
  CN=$((CN+1)); local n="hc$CN"; mkdir -p "$T/$n"; cat >"$T/$n/$2"
  if [ "$3" = none ]; then check "$1 (no row)" "$(rows "$n" | cut -f1,3 | tr '\n\t' ';,')" ""; else check "$1" "$(rows "$n" | cut -f1,3 | tr '\n\t' ';,')" "compose_image_unpinned,$3;"; fi
}
hcase "R4-H a quoted key (JSON style)" docker-compose.yml 3 <<'EOF'
services:
  a:
    "image": "postgres:15"
EOF
hcase "R4-H a flow mapping service" docker-compose.yml 2 <<'EOF'
services:
  a: {image: postgres:15}
EOF
hcase "R4-H a flow mapping with build and a registry-qualified image" docker-compose.yml 2 <<'EOF'
services:
  a: {build: ., image: ghcr.io/o/x:1}
EOF
hcase "R4-H a flow mapping with build and a local single-component tag is local" docker-compose.yml none <<'EOF'
services:
  a: {build: ., image: catalogizer-api:test}
EOF
hcase "R4-H a compose file with another name (top-level services:)" stack.yml 3 <<'EOF'
services:
  a:
    image: redis:7
EOF
hcase "R4-H control: a YAML file that is no compose file is not scanned" workflow.yml none <<'EOF'
jobs:
  x:
    container:
      image: redis:7
EOF
hcase "R4-H a recursive YAML alias does not loop (the structural walk visits each node once)" docker-compose.yml 5 <<'EOF'
x-loop: &loop
  - *loop
services:
  a:
    image: redis:7
EOF
hcase "R4-H the image value on the next line" docker-compose.yml 3 <<'EOF'
services:
  a:
    image:
      redis:7
EOF
hcase "R4-H a YAML 1.1 anchor and merge key: the anchored mapping is judged once" docker-compose.yml 2 <<'EOF'
x-c: &c
  image: redis:7
services:
  a:
    <<: *c
  b:
    <<: *c
EOF
printf 'services:\n\ta:\n\t\timage: nginx\n' >"$T/hcx.yml"; mkdir -p "$T/hcx"; cp "$T/hcx.yml" "$T/hcx/docker-compose.yml"
check "R4-H YAML the parser rejects (tab indentation) falls back to the text grammar and still flags the image" "$(rows hcx | cut -f1,3 | tr '\n\t' ';,')" "compose_image_unpinned,3;"
check "R4-H CHECK_PINS_NO_YAML=1 forces the text grammar (same verdict on the tab file)" "$(CHECK_PINS_NO_YAML=1 bash "$SUT" --root "$T/hcx" --list 2>/dev/null | cut -f1,3 | tr '\n\t' ';,')" "compose_image_unpinned,3;"
# the pull_policy matrix of class C must give the same verdicts under the text grammar (the structural path is the default)
CN=$((CN+1)); nny="cy$CN"; mkdir -p "$T/$nny"
for pol in never build always missing '"never"' "'build'"; do
  printf 'services:\n  a:\n    build: .\n    image: someorg/tool:1.0\n    pull_policy: %s\n' "$pol" >"$T/$nny/docker-compose.yml"
  case "$pol" in *never*|*build*) want="";; *) want="compose_image_unpinned,4;";; esac
  check "R4-H pull_policy $pol: structural and text grammar agree" "$(rows "$nny" | cut -f1,3 | tr '\n\t' ';,')|$(CHECK_PINS_NO_YAML=1 bash "$SUT" --root "$T/$nny" --list 2>/dev/null | cut -f1,3 | tr '\n\t' ';,')" "$want|$want"
done
# --- registry/expansion/operand members written for the reviewer mutants of round 3 (RV1-RV12): one distinguishing input each
ENGVARS='$DOCKER $PODMAN $NERDCTL $CONTAINER_ENGINE $CONTAINERENGINE $CONTAINER_RUNTIME $CTR_ENGINE $OCI_ENGINE $ENGINE ${DOCKER_BIN} ${PODMAN_CMD} ${CONTAINER_ENGINE:-podman}'
EL=(); for e in $ENGVARS; do EL+=("$e run --rm postgres:15"); done
batch_ref "R4-RV6 every recognised engine variable name is an engine word" postgres:15 "${EL[@]}"
batch "R4-RV7 awk / grep / sed talk about commands: their words are data" no "awk '/x/' docker run notes.txt" "grep -n docker pull notes" "sed -n p docker run x"
batch "R4-RV8 a host:ip value after an unknown option (long or short) is not the image" no "podman run --rm --frobnicate db:10.0.0.1 docker.io/library/postgres@sha256:$D64 true" "podman run --rm -Z db:10.0.0.1 docker.io/library/postgres@sha256:$D64 true"
fx rv9 scripts/a.sh <<'EOF'
#!/usr/bin/env bash
curl -fsSL https://x.invalid/i | doas -u root sh
EOF
expect_bad "R4-RV9 doas -u root sh" rv9 pipe_to_shell 2
fx rv11 Dockerfile <<EOF
FROM docker.io/library/debian@sha256:$D64
COPY --from=localhost/catalogizer-builder:dev /a /a
EOF
expect_good "R4-RV11 COPY --from=localhost/... is a locally built image" rv11
fx rv12 Dockerfile <<'EOF'
FROM localhost/catalogizer-base:dev
EOF
expect_good "R4-RV12 FROM localhost/... is a locally built image" rv12
fx rv5 docker-compose.yml <<'EOF'
services:
  a:
    build: .
    image: catalogizer-api:test
    pull_policy: "never"
EOF
expect_good "R4-RV5 a quoted pull_policy value is read without its quotes" rv5

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
