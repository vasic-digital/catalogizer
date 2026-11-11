# Live pprof verification (2026-11-11) - CAT-QA-PPROF-1

Target: stack 3 API `catqa3-api`, image `localhost/catqa3-api:main-70d10721` (health reports build 70d10721dde373d70e6e4d171cced36c81f99ee9), on this host.
Method: `api_recreate.sh on|off` recreates ONLY catqa3-api with the stack's own `podman run` recipe (HELIX_PPROF_ENABLED set / unset); `pprof_live.py on N` drives the real HTTP API with real login (credentials only from the 0600 qa.env, a throwaway non-admin user is registered and deleted per run).
- RED baseline (flag off): `red_off_1.txt` - admin GET /debug/pprof/ returns 404.
- GREEN (flag on): `res_on_1..3.txt/json` - 3 identical runs, each 34 checks, 34 PASS / 0 FAIL: anonymous -> 401 on all six endpoints, scrape token rejected 401, non-admin -> 403, admin -> 200, seconds 0/-1/abc/61/3600 -> 400, 2 s profile ~2.0 s, concurrent second sampler -> 429, slot released afterwards, 60 s cap accepted and bounded (60.0 s).
- Flag restored off afterwards; health 200 with build 70d10721 and no HELIX_PPROF env in the container.
Caveat: one board/host only; pprof output bytes vary per run (hashes differ by design).
Root cause of the earlier start failure: see docs/qa/stack3 lifecycle work (DBUS_SESSION_BUS_ADDRESS ',guid=' suffix breaks crun's sd-bus call).
