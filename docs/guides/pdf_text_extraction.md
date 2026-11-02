# PDF text extraction (CAT-002)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-11-03 |
| Status | active |

What the catalog-api service does when a PDF is converted to text (`catalog-api/services/pdf_text_extractor.go`, `pdf_text_isolation.go`).

## Production behaviour

1. No PDF parser runs inside the API process. Every parser runs in a child process started through `prlimit` (util-linux) with an address-space limit, a CPU-time limit (100 s), no core files and no file writes. The child is killed when the request context ends (kill, then `WaitDelay` 2 s), its stdout is bounded and its stderr is cut at 4 KiB.
2. Primary engine: poppler `pdftotext` plus `pdfinfo`, when both are installed. The runtime images `catalog-api/Dockerfile` and `catalog-api/Dockerfile.runtime` (and `Dockerfile.dev`) install `poppler-utils` and `util-linux`, so a container built from them uses poppler.
3. Fallback and only engine without poppler: the pure-Go `github.com/ledongthuc/pdf` library, run in a re-exec of the API binary (hidden first argument `__catalogizer_pdftext_child`). It is used when poppler fails for a reason other than a cancelled or expired context; a context error never starts the fallback. The fallback child is capped the same way, so falling through is safe.
4. Without `prlimit` the extractor refuses (error `refusing to run a PDF parser without prlimit`); it never runs a parser uncapped.

## Limit values

Address space per child: `MemAvailable / (2 * 3)` (3 = conversion semaphore slots), clamped to 512 MiB .. 1 GiB for the pure-Go child and 512 MiB .. 4 GiB for poppler; 512 MiB when `/proc/meminfo` cannot be read. Total extracted text: 64 MiB. pdfinfo output: 64 KiB (an overflow is an error).

## Known limits (not fixed)

- The pure-Go engine reads tracked text (large `Tc`) as word breaks and rotated text one character per line; poppler handles both.
- A child killed by its limits yields an error; the document is then not converted (no partial text).
- `RLIMIT_NPROC` is not set: a Go child needs threads, and the limit counts per user, not per process tree.
