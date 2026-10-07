# index_writer_preflight.sh

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Task | T022 |
| Script | scripts/audit/index_writer_preflight.sh |
| Test | scripts/audit/tests/test_index_writer_preflight.sh |
| Data | config/index/writer_limits.yaml |

Read-only host preflight for the code-index writer. Owner decision 2026-10-07: this host has 30.2 GiB RAM, so the upstream 32 GiB MemAvailable floor can never pass; the consumer floor is 20 GiB, the heap cap is MemTotal x 60 / 100 (about 18 GiB), the single-writer guard stays, run only while no other agents or builds run.

Usage: `scripts/audit/index_writer_preflight.sh [--limits FILE]`. Exit 0 PASS, 1 FAIL, 2 config error. Output: mem_avail_kb, min_mem_avail_kb, heap_mb (MemAvailable/2, at least 8192, at most cap), heap_cap_mb, PASS or FAIL lines. A live writer is detected by the real argv0 of other processes (codegraph or codegraph_safe.sh with index, sync or init), not by substring.

Limit: the upstream writer submodules/constitution/scripts/codegraph/codegraph_safe.sh still hard-codes 32 GiB (lines 317-322) and has no override; this script does not make it pass. An upstream change is needed (see evidence/wp02/floor-0107-report.txt).
