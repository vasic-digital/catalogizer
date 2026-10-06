# Lumen recall by language (T024)

Run: lumen_verify.sh --golden audit/lumen_golden_60.json --k 5 against a COPY of this project's Lumen index subdirectory (XDG_DATA_HOME copy; the shared index untouched). Source: audit/lumen-verify/results.tsv, summary.txt.

Summary: PASS=49 FAIL=11 SKIP=0 ERROR=0, recall (PASS over PASS+FAIL of the 50 indexable queries) = 0.780, below the tool default 0.85; known_misses=0 so all 11 are NEW_MISS. This is a measured result and is not hidden: the verdict of the run is FAIL (rc=1).

| gold extension | indexable PASS | indexable FAIL | unsupported, correctly not found | unsupported, found (unexpected) |
|---|---|---|---|---|
| .go | 27 | 6 | 0 | 0 |
| .kt | 0 | 0 | 6 | 0 |
| .py | 1 | 0 | 0 | 0 |
| .rs | 2 | 1 | 0 | 0 |
| .sh | 0 | 0 | 4 | 0 |
| .ts | 9 | 4 | 0 | 0 |

Unsupported (Lumen cannot chunk these): the 10 queries whose gold file is shell, Kotlin or text are reported as **unsupported**, not as misses: all 10 were correctly not found. Kotlin (Android, Android TV) and shell (Build framework, scripts) therefore have NO semantic coverage and use the structural index plus grep (docs/02 2.2).

Indexable FAIL rows (rank - = gold file absent from the top 5 distinct files):

- L60-03 rank - gold catalog-api/middleware/concurrency_limiter.go: gin middleware that caps the number of requests being processed at the same time
- L60-08 rank 6 gold catalog-api/utils/lru_cache.go: thread-safe least recently used cache with time to live
- L60-17 rank 10 gold catalog-api/services/playlist_service.go: business logic for creating playlists for a user
- L60-26 rank - gold catalog-api/internal/cache/cache.go: cache interface types re-exported as type aliases
- L60-27 rank - gold catalog-api/main.go: registration of the health endpoint alias used by QA tooling
- L60-31 rank - gold submodules/watcher/pkg/debounce/debounce.go: coalesce rapid filesystem change events with a generation counter against stale timers
- L60-34 rank 8 gold catalog-web/src/lib/websocket.ts: derive the WebSocket URL from the page origin in development so it goes through the Vite proxy
- L60-37 rank - gold catalog-web/src/hooks/useCoverQuality.ts: hook that probes cover quality response headers with a HEAD request and caches the result in a module-level map
- L60-40 rank - gold catalog-web/src/lib/module-registry.ts: registry of shared vasic-digital React modules next to the local auth provider
- L60-42 rank - gold catalog-web/src/lib/subtitleApi.ts: subtitle search and download API client
- L60-48 rank - gold catalogizer-desktop/src-tauri/src/vlc/commands.rs: Tauri commands bridging the React frontend to the VLC backend

## Measurement caveats (UNCONFIRMED where stated)

- The golden file itself (audit/lumen_golden_60.json, which holds each question verbatim) is inside the indexed tree and appears in the top results (e.g. L60-01); it competes with the gold files and may depress rank for some queries. Whether excluding the audit tree from Lumen would raise recall is UNCONFIRMED (no re-run done). scope.yaml project_excludes already excludes the audit evidence tree, not the audit/ tree.
- The copy was re-indexed by EnsureFresh under the NEW .lumenignore during the run (each search took about 13 s at 100% CPU); the index state used is therefore neither the old shared index nor a refresh recorded by T022. The shared Lumen index stayed Stale: yes.
- Binary: lumen 0.0.42 at ~/.claude/plugins/cache/claude-plugins-official/lumen/0.0.42/bin/lumen-linux-amd64, sha256 30a5796d5e7b5fbbadb984da4d495402f80ffbb282d9ef0ed65270e5e847f1ff.
- Copy size: du -sb of the copied subdirectory 190382160 bytes (shared original 190382160 before, DB 190341120 bytes by index_status); free space of the target filesystem (/tmp) 11814711296 bytes before the copy; the home filesystem had 97.7 GB free. No 12.9 headroom constant was found in the task text, so the margin is judged by size: the copy is 1.6% of the target free space.
