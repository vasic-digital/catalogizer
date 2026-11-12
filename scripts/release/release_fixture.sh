#!/usr/bin/env bash
# Fixture builder shared by check_release_consistency.sh --selftest and
# test_check_release_consistency.sh. Source it; call: make_fixture <empty-dir>.
# Builds a tiny git repo shaped like Catalogizer (version 2.4.0, tag fx-prev
# with versionCode 6, HEAD versionCode 7). Test doubles of the product are NOT
# involved: this is input data for the checker (a fixture INPUT, 11.4.289(C)(3)).

make_fixture() {
  local d="${1:?make_fixture needs a directory}"
  mkdir -p "$d" || return 1
  (
    set -e
    cd "$d"
    git init -q .
    git config user.email fx@example.invalid
    git config user.name fx
    git config commit.gpgsign false
    mkdir -p docs/api docs/qa/run1 docs/qa/run2 catalog-web catalogizer-desktop/src-tauri \
      installer-wizard/src-tauri catalogizer-android/app catalogizer-androidtv/app scripts
    printf '{\n  "schema_version": 1,\n  "global": {"major": 2, "minor": 4, "patch": 0, "build_number": 25},\n  "components": {"x": {"version": "9.9.9"}}\n}\n' > versions.json
    cat > docs/api/openapi.yaml <<'Y'
openapi: 3.0.3
info:
  title: Fixture API
  description: |
    Text mentioning
    version: 9.9.9
    inside a block scalar.
  version: 2.4.0
paths:
  /x:
    get:
      parameters:
        - name: v
          schema:
            version: 7.7.7
Y
    local c
    for c in catalog-web catalogizer-desktop installer-wizard; do
      printf '{\n  "name": "%s",\n  "version": "2.4.0",\n  "dependencies": {"react": "^2.3.0", "lodash": "1.0.0"}\n}\n' "$c" > "$c/package.json"
      printf '{\n  "name": "%s",\n  "version": "2.4.0",\n  "lockfileVersion": 3,\n  "packages": {\n    "": {"name": "%s", "version": "2.4.0"},\n    "node_modules/react": {"version": "2.3.0"}\n  }\n}\n' "$c" "$c" > "$c/package-lock.json"
    done
    for c in catalogizer-desktop installer-wizard; do
      printf '{\n  "productName": "%s",\n  "version": "2.4.0",\n  "plugins": {"v": {"version": "1.2.3"}}\n}\n' "$c" > "$c/src-tauri/tauri.conf.json"
      printf '[package]\nname = "%s"\nversion = "2.4.0"\nedition = "2021"\n\n[dependencies]\nserde = { version = "1.0.0", features = ["derive"] }\ntokio = "1.2.3"\n' "$c" > "$c/src-tauri/Cargo.toml"
    done
    for c in catalogizer-android catalogizer-androidtv; do
      printf 'android {\n    defaultConfig {\n        applicationId = "x.%s"\n        versionCode = 6\n        versionName = "2.4.0"\n    }\n}\ndependencies {\n    implementation("a:b:1.0.0")\n}\n' "$c" > "$c/app/build.gradle.kts"
    done
    echo '*.png' > .gitignore
    echo evidence-a > docs/qa/run1/a.txt
    echo evidence-b > docs/qa/run1/b.txt
    ( cd docs/qa/run1 && sha256sum a.txt b.txt > SHA256SUMS && printf '%s  shot.png\n' "$(printf shot | sha256sum | cut -d' ' -f1)" >> SHA256SUMS )
    echo evidence-c > docs/qa/run2/c.txt
    ( cd docs/qa/run2 && sha256sum c.txt > SHA256SUMS )
    echo '#!/bin/sh' > scripts/tool.sh
    cat > docs/CHANGELOG.md <<'M'
# Changelog

## [Unreleased]
- nothing, `docs/unreleased/never_checked.txt`

## [2.4.0] - pre-release

### VERIFIED (live evidence)
- Run one: `docs/qa/run1/a.txt` and the directory `docs/qa/run1/`; also `docs/qa/run2/c.txt:12`.
- Tool `scripts/tool.sh`; a command `go test ./...` and a url `https://example.invalid/docs/x` are not paths. Dependency `react@2.3.0`.

### PROVISIONAL (not verified)
- `docs/qa/provisional/not_there.txt` is not checked.

### REOPENED
- None.

## [2.4.0-dev.20260414] - 2026-04-14 (historical)

### VERIFIED
- `docs/qa/dev/not_there_either.txt`

## [2.3.0] - 2026-04-04

### VERIFIED
- `docs/qa/old/not_there_old.txt`
M
    git add -A
    git commit -q -m fixture-prev
    git tag fx-prev
    sed -i 's/versionCode = 6/versionCode = 7/' catalogizer-android/app/build.gradle.kts catalogizer-androidtv/app/build.gradle.kts
    git add -A
    git commit -q -m fixture-head
  )
}
