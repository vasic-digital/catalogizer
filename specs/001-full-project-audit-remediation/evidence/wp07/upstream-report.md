# T078 upstream report: constitution_index.yaml is stale (and its generator cannot run) at the new constitution tips
Identity: catalogizer / 001-full-project-audit-remediation / WP-07 T078 | Revision 1 | 2026-10-06 | draft, UNREVIEWED | agent wp07 | git HEAD 87909608416f9c13b6b9e116866b6efbdc827cad | for the constitution repository (HelixDevelopment/HelixConstitution, all 8 remotes at a71b1767)
Filing: the register item that T078 also asks for is NOT filed here: scripts/register is out of this agent's file list and T069 (the register schema it depends on) is not confirmed done. A ready text is in section 6. Reporting to the constitution repository itself (FR-006 route: fix in the shared module, push to its own upstreams, with review) is an owner or later-WP act, not done here.

## 1. Defect (measured, EXECUTED read-only on trees extracted with git archive)
The machine index `constitution_index.yaml` records the sha256 of the `Constitution.md` it was generated from. At both candidate tips that hash is the hash of the OLD canon:
| rev | Constitution.md sha256 | index source_sha256 | index sha256 (file) | index generated_at | index generated_from.commit | anchors in index |
|---|---|---|---|---|---|---|
| 10b7a06 (current pin) | d915a5c10f46041b5ba8020c685d845a6a3e3c5fca75384a1a807a98eed90c72 | d915a5c10f46041b5ba8020c685d845a6a3e3c5fca75384a1a807a98eed90c72 | c6998ff640094653299318ec4f28459d25e3ca2f2cef18455c6c2aa1f2c2163f | 2026-09-26T14:41:19.328355+00:00 | 488d213cf9a7e3aedd51d231f2d622f2143b83d0 | 283 |
| e44f22f | 8cc29e0d71989fb7de2c42d1e116a4bf8a2f4bb24fd0aa6f488ca9a6700ccb39 | d915a5c10f46041b5ba8020c685d845a6a3e3c5fca75384a1a807a98eed90c72 | c6998ff640094653299318ec4f28459d25e3ca2f2cef18455c6c2aa1f2c2163f | unchanged | unchanged | 283 |
| a71b1767 (live tip) | 95117cf3d96e4bb144d4e4cfed4031392abd91e22d55c95e6eb5aed940238a4e | d915a5c10f46041b5ba8020c685d845a6a3e3c5fca75384a1a807a98eed90c72 | c6998ff640094653299318ec4f28459d25e3ca2f2cef18455c6c2aa1f2c2163f | 2026-09-26T14:41:19.328355+00:00 | 488d213cf9a7e3aedd51d231f2d622f2143b83d0 | 283 |
The index file is byte-identical across the three revisions (same file sha256 in all rows): no commit of the range touches it, although the range edits Constitution.md (e44f22f: 5 files, §11.4.235 clause D; a71b1767: §11.4.276 new, §11.4.209/§11.4.211/§11.4.230/§11.4.134 edited, header Revision 68 -> 72).
Consequence for consumers: the Spec Kit layer's own freshness check ('Constitution.md sha256 identical to the hash recorded in constitution_index.yaml') cannot pass after a bump; at a71b1767 the index also lacks the anchor 11.4.276 (`### §11.4.276` is present in Constitution.md line 11928 and absent from the index, 283 anchors in the index vs 257 `### §` headings of which 11.4.276 is the only one the index lacks, measured with a heading regex and a set difference; the other ids the index holds and the headings lack are anchors whose heading has a different form, e.g. 11.4.100).

## 2. The upstream generator guard is RED at both tips (and was GREEN at the pin: control)
`bash scripts/gates/gate_constitution_generate_no_drift.sh` (it runs scripts/anchors/constitution_generate.py check against groups/*.md and the index):
- 10b7a06: check: no drift (rc 0)
- e44f22f: FATAL: committed constitution_index.yaml diverges from a fresh generate — diverged field(s): ["anchors: fields differ for id(s): ['11.4.235']", "generated_from: sub-field(s) differ: ['source_sha256']"] (rc 1)
- a71b1767: FATAL: anchor '11.4.276' matches no group rule (rc 6)
So (a) at e44f22f the guard fails on the stale hash and the changed anchor 11.4.235, and (b) at a71b1767 the generator CANNOT regenerate: it stops with 'FATAL: anchor 11.4.276 matches no group rule' (rc 6, UnclassifiedAnchorError) because scripts/anchors/anchor_lib.py ID_RANGE_GROUPS has no range for 276 (the group governance-and-constitution-meta holds (272, 275), line 325 of the file at a71b1767). The guard is a 'standing regression guard' (§11.4.135) but was not run, or was ignored, when the corpus was last edited: UNCONFIRMED whether it is wired into any upstream commit or release step (a search for its name in the tree finds only the script itself).

## 3. Proposed fix in the constitution repository (not applied, this agent does not edit it)
1. Add a group range for 11.4.276 in scripts/anchors/anchor_lib.py (candidates: widen governance-and-constitution-meta (272, 275) to (272, 276), or code-review-and-quality; the group choice is the maintainers', UNCONFIRMED).
2. Regenerate: `python3 scripts/anchors/constitution_generate.py generate --source Constitution.md --groups-dir groups --index-out constitution_index.yaml` (flags as used by the guard script; subcommands generate and check exist at constitution_generate.py lines 480 and 485), then re-run the guard until 'check: no drift', then commit the regenerated groups/*.md and index together with the corpus change.
3. Make the guard part of the step that edits Constitution.md (the existing propagation/ledger gates run in the same family), so the index cannot lag.
4. A review of that change under the constitution's own rules (11.4.142) before its push, fast-forward to every upstream (11.4.113).

## 4. What this project does meanwhile
`scripts/governance/regen_speckit_catalogue.py` (T076) records both hashes and states 'the index lags this pin', naming anchors present in Constitution.md but absent from the index (11.4.276 at a71b1767), in .specify/memory/constitution.md and in the Canon pin row of the appendix. The catalogue it generates lists only what the index knows, so a catalogue regenerated from this index at a71b1767 would omit 11.4.276 until upstream regenerates; the appendix (hand-written digests) would need a hand-written digest of 11.4.276 either way (owed to T081).

## 5. Limits
* All statements come from read-only commands on extracted trees and objects (output reproduced in lockstep.txt section D and in this file's tables); no upstream repository was written.
* UNCONFIRMED: that the maintainers regenerate the index by the command in section 3 without further steps (the flags were read from the guard script, not executed in generate mode, because generate mode writes files into the tree and a scratch run was not done).
* UNCONFIRMED: whether the index hash drift existed before 10b7a06 (the pin is consistent: control rc 0).

## 6. Text for the register item (to be filed after T069, by the owner of the register)
Title: Constitution repository: constitution_index.yaml stale at e44f22f and a71b1767 (source_sha256 of the old canon; anchor 11.4.276 missing; generator rc 6)
Type: Bug | Component: submodules/constitution (upstream repository) | Severity: major | Evidence: evidence/wp07/upstream-report.md, evidence/wp07/lockstep.txt section D
Acceptance: after the fix, gate_constitution_generate_no_drift.sh exits 0 at the new tip, index source_sha256 equals sha256(Constitution.md), all 11.4.N headings are in the index; Spec Kit freshness check passes without a lag note.
