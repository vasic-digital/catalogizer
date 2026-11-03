# PDF text extraction (CAT-002)

| Field | Value |
|---|---|
| Revision | 2 |
| Last modified | 2026-11-03 |
| Status | active |

What the catalog-api service does when a PDF is converted to text (`catalog-api/services/pdf_text_extractor.go`, `pdf_text_isolation.go`).

## Production behaviour

1. No PDF parser runs inside the API process. Every parser runs in a child process started through `prlimit` (util-linux) with a memory limit (address space for poppler, data size for the pure-Go child), a CPU-time limit (20 s), no core files and no file writes, and with a fixed-allowlist environment (`PATH LANG LC_ALL LC_CTYPE TZ TMPDIR`; the API process's secrets never reach a parser). The child is killed when the request context ends (kill, then `WaitDelay` 2 s), its stdout is bounded and its stderr is cut at 4 KiB.
2. Primary engine: poppler `pdftotext` plus `pdfinfo`, when both are installed. The runtime images `catalog-api/Dockerfile` and `catalog-api/Dockerfile.runtime` (and `Dockerfile.dev`) install `poppler-utils` and `util-linux`, so a container built from them uses poppler.
3. Fallback and only engine without poppler: the pure-Go `github.com/ledongthuc/pdf` library, run in a re-exec of the API binary (hidden first argument `__catalogizer_pdftext_child`). It is used when poppler fails for a reason other than a cancelled or expired context; a context error never starts the fallback. The fallback child is capped the same way, so falling through is safe.
4. Without `prlimit` the extractor refuses (error `refusing to run a PDF parser without prlimit`); it never runs a parser uncapped.

## Limit values

- poppler (C++, reserves little virtual memory): `RLIMIT_AS` = `MemAvailable / (2 * 3)` (3 = conversion semaphore slots), clamped to 512 MiB .. 4 GiB; 512 MiB when `/proc/meminfo` cannot be read.
- pure-Go child: `RLIMIT_DATA` (private writable memory) = the same division, clamped to 384 MiB .. 1 GiB; 384 MiB when `/proc/meminfo` cannot be read. The Go runtime is pinned in the child environment: `GOMAXPROCS=2`, `GOMEMLIMIT` = 3/4 of the data limit. The environment can lower the child's total-text cap (test hook) but never raise it.
- CPU time per child: 20 s (ordinary 851-page document: about 4 s pure Go, 2.5 s poppler). A hostile document that loops is killed after at most 20 s CPU instead of holding a conversion slot for minutes (round 4 measured 100 s).
- Total extracted text: 64 MiB. pdfinfo output: 64 KiB (an overflow is an error).

### Why RLIMIT_DATA for the Go child (measured, round 5)

`RLIMIT_AS` counts reserved-but-untouched address space. A cgo Go binary (this one links jemalloc and libstdc++) reserves several hundred MiB at start (page summary, arena map, one 8 MiB stack per thread, growing with `GOMAXPROCS`) before any of our code runs; the child also runs every package `init` of the API binary before the extraction hook. Measured with the production binary built from this tree (16-CPU host, `prlimit ... -- env -i PATH=/usr/bin GOMAXPROCS=n api.bin __catalogizer_pdftext_child ...`):

| Limit | Go runtime | 146-page document | 851-page document |
|---|---|---|---|
| `--as` 512 MiB / 768 MiB | default | fails every run (`failed to reserve page summary memory`, `out of memory`) | n/a |
| `--as` 1 GiB (round 4 value) | GOMAXPROCS 16 / 32 / 64 | 13/15, 8/15, 0/15 ok | OOM at start |
| `--as` 900 / 1000 / 1100 / 1300 / 1600 MiB | GOMAXPROCS=2 | ok / fail / ok / fail / ok (not monotone: address-space layout dependent) | n/a |
| `--data` 128 MiB | GOMAXPROCS=2 | fails | n/a |
| `--data` 192 / 256 / 384 MiB | GOMAXPROCS=2 | 5/5 ok each | 256 / 384 / 512 MiB: 3/3 ok each (about 4 s each; 84 MB RSS) |
| `--data` 256 MiB | GOMAXPROCS=64 | `pthread_create failed` | fails |
| `--data` 512 MiB / 1 GiB | GOMAXPROCS=64 | 5/5 ok | n/a |

So the data-size limit starts reliably at about a sixth of the address-space limit and does not depend on the CPU count once `GOMAXPROCS` is pinned; the 384 MiB floor is 2x the measured minimum (192 MiB). The automated tests build the production binary and run it through the real wiring at the floor limit with a faked low `MemAvailable` and with `GOMAXPROCS=64` in the parent environment, 15 runs per case on a 146-page and an 851-page generated document, zero failures required (`catalog-api/services/pdf_text_startup_test.go`).

### Start failures

If the child's runtime dies before our code runs (no `pdftext-child-started` line on stderr), or the executable is missing, or the API binary was replaced while running (`/proc` reports `... (deleted)`), the error wraps `services.ErrPDFChildStart` (`errors.Is`). A child that started and then failed on the document (memory/CPU limit, malformed file) does not.

## Known limits (not fixed)

- The pure-Go engine reads tracked text (large `Tc`) as word breaks and rotated text one character per line; poppler handles both.
- A child killed by its limits yields an error; the document is then not converted (no partial text).
- `RLIMIT_NPROC` is not set: a Go child needs threads, and the limit counts per user, not per process tree.
- Round 5 measurements are from one 16-CPU host under load (load average 10-19); the 64-CPU case was exercised by setting `GOMAXPROCS=64` (the child ignores it by design), not on a 64-CPU machine.
- The child still runs every package `init` of the API binary (about 80 MB RSS, roughly 0.1-0.2 s) per call. A dedicated tiny child binary would remove that cost; not done.
- The poppler path keeps `RLIMIT_AS`; poppler was measured to start at 512 MiB on every legit file in round 4, not re-measured in round 5.
