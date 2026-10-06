# DR-E1 (T048): the shipped chain deviates from the REUSE decision, and 11.4.268 asks for reuse - OWNER QUESTION OPEN

Date: 2026-10-05 (fix round 3 of T050). Review finding I13 of `WF2-REVIEW-wp05-evrec`. This file records the state. It does NOT decide.

## State, stated plainly

1. `T048-reuse-decision.md` concludes REUSE of continuum's chain primitive ("Do NOT build a second chain") and makes that conditional on four owner decisions
   (hash construction, record schema, exit codes, anchor file format; its section 4) that were not answered before T049 and T050 were built.
2. Constitution 11.4.268 says a project "MUST NOT stand up a parallel, divergent chain implementation" where a content-addressed store already satisfies the
   anchor's properties.
3. T050 shipped `tools/evidence/evcore.py` with the chain exactly as the frozen contract `ev/1` and docs/06 section 7 define it
   (`entry_hash = sha256(prev_hash || canonical_json(entry without entry_hash))`, genesis prev = 64 zeros, keys sorted, compact, UTF-8). That is a SECOND chain
   implementation and is byte-incompatible with continuum's construction. **The shipped tool therefore deviates from the T048 REUSE decision and is in tension
   with 11.4.268.** It is an open deviation, not a closed decision. Nothing in this fix round changes that: round 3 hardened the shipped chain
   (golden vectors, blob and schema verification, canonical-form check) and did not move it toward or away from continuum.
4. T048's deliverable path differs between tasks.md (`$EV/wp05/DR-E1.md`) and what exists (`T048-reuse-decision.md`). Not reconciled here.

## What the open question costs to answer either way (corrected: the earlier note understated it)

The earlier `T050-implementation.md` said that switching to continuum's record would change only `chain_walk` and `compute_entry_hash` (plus the schema). That is too
small. If the owner chooses continuum's Record plus digest, ALL of these change together:

- the `ev/1` record shape (`prev_hash`, `entry_hash` and `seq` are required fields of the contract; T048a and the revision 7 schema pin them);
- the construction in `compute_entry_hash` and `chain_walk`, and every hand-hashed fixture (`test_evrec_more.sh`, `test_evrec_r3.sh` golden-good cases built with
  jq | sha256sum, the pinned vectors in `test_evcore_golden.py`);
- the exit-code map (verify 0/1/2/3 and the evrec refusal codes 64-77);
- the anchor file format and its writer (T055), which does not exist yet;
- every ledger or verdict consumer that reads `entry_hash` (the verdict deriver T054, the register importer).
Keeping the shipped chain instead means the owner accepts a second implementation against 11.4.268 and records why.

## Owner question (NOT in `decisions/owner-request-list.md` or `owner-decisions.yaml`: those files are outside this change's scope, so the item below must be filed there by the conductor)

> DR-E1: keep the evidence chain as shipped (docs/06 section 7 construction, `ev/1` with `prev_hash`/`entry_hash`), or adopt continuum's chain primitive as T048
> concluded? If kept, record the 11.4.268 reuse-clause exception and its reason; if adopted, schedule the change set listed above before T054 and T055 build on it.

UNCONFIRMED: the owner's answer; whether continuum's Record + digest can carry the `ev/1` fields without loss (T048 section 4 lists the mismatches but no probe of a
real continuum binary was possible: none exists on this host and a Go build needs the container path of 11.4.173, which does not exist yet).
