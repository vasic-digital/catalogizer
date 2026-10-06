# IMG-TESTUTIL

Test and check tools for the P0 tests and the CPA checks: bash, git, sqlite3, python3 (jsonschema, PyYAML, pytest), jq, and the hash-pinned CPA tools in `/opt/cpa-tools` (`requirements.txt`, `pip --require-hashes`).
Built from the T006 `Containerfile` on the pinned Debian bookworm-slim and the 2026-09-18 snapshot (evidence `wp09/build-testutil.txt`). Run through `scripts/containers/run_pinned.sh IMG-TESTUTIL -- <command>`.
This README and `digests.lock` were added by T106; the `Containerfile` and `requirements.txt` are T006's, unchanged.
