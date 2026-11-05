# Live S3 server for Catalogizer (CAT-S3-1)

Real Garage v1.0.1 (`docker.io/dxflrs/garage`) under rootless podman, `--memory 1g --memory-swap 1g --cpus 1`, own network `catqa-s3live`,
bound to `192.168.1.115:39320`. Independent of stacks catqa1/2/3.

- `up.sh` starts it, creates bucket `catalogizer-live` and a key; credentials go ONLY to `$S3_STATE/s3.env` (0600, outside git).
- `down.sh [--purge]` removes the container and network (and the state dir with `--purge`).
- Test: `cd catalog-api && set -a; . $S3_STATE/s3.env; set +a; go test -tags s3live -run TestS3Live -count=1 -v ./filesystem/`
  (without the env vars the tests SKIP honestly). Evidence runner: `docs/qa/s3-live-20261105/run.sh`.

Fallbacks if Garage cannot run: SeaweedFS, then versitygw (not needed so far; Garage pulled and ran).
Garage is an S3 implementation, not AWS: limits that AWS enforces (e.g. 1000 keys per DeleteObjects) are not all enforced here.

`S3_PURGE_ROOT` (down.sh) is TEST-ONLY: the sandboxed `test_down_purge.sh` sets it to a mktemp dir. Operators must not set it.
Tests: `test_down_purge.sh` proves the `--purge` guard (each layer, `..` / dirname / readlink, is independently load-bearing; mutation results in `docs/qa/s3-live-20261105/down_purge_mutations.txt`).
