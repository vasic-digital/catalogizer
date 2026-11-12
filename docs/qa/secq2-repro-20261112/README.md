# SECQ-2 / WD-F1 / WD-F1a live re-reproduction (2026-11-12) - PROVISIONAL

Reproduction: result=not-reproducible-with-evidence; surface=live-target (real Yandex WebDAV, chaos class: TCP fault proxy, the real service answers); command=`LIVE_WEBDAV_SANDBOX=/Testing WDF1_REPEAT_A2M=6 go test -tags live -count=1 -run TestLiveYandexWDF1ForcedReset ./filesystem/` (catalog-api, credentials from a 0600 env file read by name); RED evidence: none (nothing failed); GREEN-class evidence: rep6.json / rep6.log.txt; build: main f709b892; symptom signature: `verdict=CORRUPT` or `LOST_ACKED_WRITE` on a real case, or `present_no_retry=true` with absent-after-retry.

Attempts (this run, started 2026-11-12T20:10:03Z, 1250 s): 6 repeats of the exact WD-F1a case (mode A, connection reset mid-body, 2 MiB, one caller retry). 6 of 6 verdict=ok; every case recorded `ReadDetail: len=2097152 want_len=2097152 first_diff=-1 body_err=false`, i.e. the retried file was byte-identical (sha256 matched). Fault fired and put error seen in all 6 (armed fault confirmed).
Positive control needles (verdict logic can fire): ack-then-remote-DELETE needle and same-size-overwrite needle (control needle 2 = CORRUPT) both flagged; test PASS.
Combined with 2026-11-05 evidence (`../wdf1-repro-20261105/`): 61 plain uploads + 12 forced-reset cases + these 6 = no acknowledged write lost, 1 unexplained CORRUPT (2026-11-05 chaos1, detail not recorded then) never seen again in 11 further identical attempts (5 + 6).

Conclusion: WD-F1 (lost acknowledged write) and WD-F1a (same-size hash mismatch) remain not reproduced; no fix written for an unobserved symptom (section 11.4.292). Proposal: close SECQ-2 / WD-F1 / WD-F1a under section 11.4.90 `not-reproducible` once the operator accepts the attempt count; the defensive read-back in `WriteFile` (size-only HEAD) stays labelled defensive hardening. UNCONFIRMED: whether the 2026-11-05 CORRUPT was a read-back glitch; root cause UNCONFIRMED.
Cleanup: the test creates one `/Testing/wdf1c-<unix>` directory and deletes it on exit (deferred DeleteDirectory); remote residue not separately listed (UNCONFIRMED, minimal-calls rule).
Not run: a second full A/B x 3-size sweep (would repeat 2026-11-05 chaos2/3, 15+ min each).

## sha256
a9d2a2296d355bd9319eaf813eee0c66b0fa3902c16a66a2cf642026ad8ddcea  rep6.json
6be49c0254285c6edec965934e5cd156f8cfa49d48d8152162167a6bd9a93ae6  rep6.log.txt
6c1ff7770e5355a12e517d96e733a536384e5561e88fca2ceeff53f3810a02c6  rep6.start
