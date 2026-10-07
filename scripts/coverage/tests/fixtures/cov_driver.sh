#!/usr/bin/env bash
# drives the fixtures the way a test suite drives scripts (through child bash processes)
d="$(cd "$(dirname "$0")" && pwd)"
bash "$d/cov_fixture.sh" >/dev/null
bash "$d/cov_fixture2.sh" >/dev/null
bash "$d/vendor/cov_excluded.sh" >/dev/null
