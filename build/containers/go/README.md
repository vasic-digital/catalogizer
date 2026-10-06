# IMG-GO

Go build and test image: `docker.io/library/golang:1.25-bookworm` (glibc, because CGO links `docx.c` against glibc fortify symbols; `go.mod` requires 1.25.7, the image carries go1.25.14 as probed 2026-10-06).
Lock entry `IMG-GO` is the upstream image by its index digest `sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437`; class is not yet written (T121b backfills `compile`).
Pulled, not built. Run through `scripts/containers/run_pinned.sh IMG-GO -- <command>`.
