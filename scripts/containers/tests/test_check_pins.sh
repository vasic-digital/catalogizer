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
