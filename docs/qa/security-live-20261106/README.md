# Live security verification, stack 3, 2026-11-06 (scratch branch, not pushed)
Scripts: run_prom.sh, run_pprof.sh, run_ssrf.sh, run_xff.sh/xff_all.sh (+listener.py). Each result file is the raw output; N=3 runs per item.
Hashes (sha256, first 16): prom 3x d391ead7a8122d0b; pprof 3x 78c6528542cb8ee7; ssrf 3x 9bd1946c93d1f276; xff per target 3 identical blocks (xff_normalised.txt).
Secrets: read inside the scripts only, passed to curl via stdin config; a scan of this directory for every value in qa.env/mon.env found only non-secret names (db/user/mode words).
Verdicts: CAT-PROM-AUTH-1 VERIFIED; CAT-PPROF-LIVE-1 VERIFIED (3/3); SSRF via image-proxy, POST /storage/roots, POST /sync/endpoints VERIFIED (3/3);
SSRF-F2..F5 ffmpeg/cap path and dial-time/redirect hops BLOCKED; TRUSTED_PROXIES: front-door attribution FAILED (shared bucket), second-host BLOCKED.
