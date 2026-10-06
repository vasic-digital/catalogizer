# IMG-GOTOOLS

`goimports` from `golang.org/x/tools` v0.49.0 on the IMG-GO digest, for the commit-push S3 check `go-imports` (`goimports -l`, check-only). v0.50.0 and later need go 1.26.0, which IMG-GO (go1.25) does not carry.
Existence verdict (11.4.270): `specs/001-full-project-audit-remediation/evidence/wp11/gotools.json`. The `go_imports` row of `scripts/repo/validate_checks.tsv` and its baseline are not changed by this change (other area, owed, T106).
Class `compile`; run through `scripts/containers/run_pinned.sh IMG-GOTOOLS -- goimports -l <paths>` once the lock entry exists.
