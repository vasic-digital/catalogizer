#!/usr/bin/env python3
"""mutate_ddl.py - T062 mutant generator for scripts/register/register_ext.sql.
Usage: mutate_ddl.py <ddl> <outdir>   writes one mutant file per mutant plus <outdir>/manifest.tsv
(file, kind, name, detail). Mutant kinds:
  trigger  one CREATE TRIGGER statement removed
  check    one CHECK (...) replaced by CHECK (1)
  clause   one hand-written clause-level mutation of a custody view / trigger body (CLAUSE_MUTANTS below;
           WF2 review B-1: whole-trigger and whole-CHECK mutants cannot see a lost clause, and the custody
           logic lives in views). Each clause mutant states the exact text it replaces; a mutant whose text
           is not found exactly once is an ERROR (exit 3), never silently skipped.
A mutant is a copy of the DDL without one guard: the tests must FAIL on it, or the guard is decoration."""
import re, sys, os

def checks(text):
    """yield (start, end) of every CHECK (...) in code (comments skipped), balanced parentheses."""
    out = []
    for m in re.finditer(r'\bCHECK\s*\(', text):
        ls = text.rfind('\n', 0, m.start()) + 1
        if '--' in text[ls:m.start()]:
            continue
        i = m.end(); depth = 1
        while depth and i < len(text):
            c = text[i]
            if c == '(': depth += 1
            elif c == ')': depth -= 1
            elif c == "'":
                i = text.index("'", i + 1)
            i += 1
        out.append((m.start(), i))
    return out

# (name, old text, new text, what the mutation removes). OLD must occur exactly once in the DDL.
CLAUSE_MUTANTS = [
 ("R1_reverify_set_after_insert_allowed",
  "  SELECT RAISE(ABORT,'custody: reverify_required can only be cleared (1 -> 0), never set after insert')\n   WHERE NEW.reverify_required=1 AND OLD.reverify_required=0;\n", "",
  "reverify_required may be set 0 -> 1 after insert"),
 ("R2_replay_guard_off_by_one",
  "AND p.evidence_id <= (SELECT ev_hwm FROM v_cycle_start WHERE atm_id=NEW.atm_id));",
  "AND p.evidence_id < (SELECT ev_hwm FROM v_cycle_start WHERE atm_id=NEW.atm_id));",
  "the last evidence row of the closed cycle can be replayed"),
 ("R3_review_any_cycle",
  "WHERE v.atm_id=x.atm_id AND v.verdict='GO' AND v.review_id > k.rev_hwm", "WHERE v.atm_id=x.atm_id AND v.verdict='GO'",
  "a review of an earlier cycle counts (the ev_hwm clause on its evidence still binds)"),
 ("R4_legacy_exempt_any_last_status_in_custody_select",
  "     AND NOT (EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=NEW.atm_id)\n              AND IFNULL((SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1),'Queued')='Queued');\n",
  "     AND NOT (EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=NEW.atm_id));\n",
  "the custody select drops its Queued restriction (the reachability select keeps it)"),
 ("R5_green_fingerprint_reuse_allowed",
  "    AND t.target_fingerprint NOT IN (SELECT o.target_fingerprint FROM reg_test_runs o      -- never GREEN in an\n          WHERE o.atm_id=t.atm_id AND o.polarity='GREEN' AND o.run_row <= k.run_hwm)        -- earlier cycle (§14.10 I1)\n", "",
  "a GREEN group on a fingerprint that was GREEN in an earlier cycle counts"),
 ("R6_log_guard_no_hwm_clause",
  "  SELECT RAISE(ABORT,'reg_status_log: ev_hwm/run_hwm/rev_hwm must equal the current ledger maxima (cycle boundary)')\n   WHERE NEW.ev_hwm  IS NOT (SELECT IFNULL(max(evidence_id),0) FROM reg_evidence)\n      OR NEW.run_hwm IS NOT (SELECT IFNULL(max(run_row),0) FROM reg_test_runs)\n      OR NEW.rev_hwm IS NOT (SELECT IFNULL(max(review_id),0) FROM reg_reviews);\n", "",
  "a status-log row may carry any high-water marks"),
 ("R7_red_constructed_provenance_counts",
  "AND e.exit_code BETWEEN 1 AND 125 AND e.precondition_provenance='observed'", "AND e.exit_code BETWEEN 1 AND 125",
  "a RED whose precondition was constructed counts (§11.4.115(G))"),
 ("R8_reviewer_may_have_produced_the_evidence",
  "              AND NOT EXISTS (SELECT 1 FROM reg_evidence p WHERE p.atm_id=x.atm_id\n                              AND p.kind IN ('red_run','green_run','false_positive_proof','custody_decision')\n                              AND lower(trim(p.produced_by))=lower(trim(v.reviewer)))) AS review_ok,",
  "              ) AS review_ok,",
  "the reviewer may have produced the RED/GREEN/proof/decision evidence (§11.4.240)"),
 ("R9_latest_review_not_required",
  "              AND v.review_id=(SELECT max(review_id) FROM reg_reviews WHERE atm_id=x.atm_id)\n", "",
  "an earlier GO still counts after a later NO-GO"),
 ("R10_legacy_status_any",
  "   WHERE lower(trim(IFNULL(NEW.legacy_status,''))) NOT IN ('resolved','fixed','closed');", "   WHERE 0;",
  "legacy_import allowed for an open legacy_status"),
 ("M11_red_fingerprint_not_matched_to_its_run",
  "    AND e.target_fingerprint=t.target_fingerprint\n    AND (CASE e.evidence_class WHEN 'runtime'", "    AND (CASE e.evidence_class WHEN 'runtime'",
  "RED evidence fingerprint need not equal the test-run fingerprint"),
 ("M12_red_not_bound_to_the_cycle",
  "    AND t.run_row > k.run_hwm AND e.evidence_id > k.ev_hwm\n    AND e.exit_code BETWEEN 1 AND 125", "    AND e.exit_code BETWEEN 1 AND 125",
  "a RED of an earlier cycle counts"),
 ("M13_green_needs_one_repetition",
  "HAVING count(DISTINCT t.rep_index) >= 3 AND", "HAVING count(DISTINCT t.rep_index) >= 1 AND",
  "one GREEN repetition suffices"),
 ("M14_green_group_may_span_fingerprints",
  "AND count(DISTINCT t.target_fingerprint) = 1\n", "AND count(DISTINCT t.target_fingerprint) >= 1\n",
  "a GREEN group may spread over several fingerprints"),
 ("M15_fix_chain_green_may_equal_red_fingerprint",
  "JOIN v_green_groups g ON g.atm_id=r.atm_id AND g.test_id=r.test_id AND g.fp<>r.fp\n            JOIN reg_test_runs m",
  "JOIN v_green_groups g ON g.atm_id=r.atm_id AND g.test_id=r.test_id\n            JOIN reg_test_runs m",
  "GREEN on the RED fingerprint (the fix was never deployed) completes the chain"),
 ("M16_mutation_not_bound_to_the_cycle",
  "                 AND m.run_row > k.run_hwm\n", "\n",
  "a mutation of an earlier cycle counts"),
 ("M17_surviving_mutation_counts",
  "m.polarity='MUTATION' AND m.verdict='FAIL'", "m.polarity='MUTATION'",
  "a mutation that PASSed (survived) counts"),
 ("M18_mutation_evidence_kind_not_checked",
  "AND me.atm_id=m.atm_id AND me.kind='mutation_run'", "AND me.atm_id=m.atm_id",
  "a mutation row may be backed by any evidence kind"),
 ("M19_mutation_class_floor_dropped",
  "                 AND (CASE me.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)\n                     >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)\n", "\n",
  "a source-class mutation satisfies a runtime-layer item"),
 ("M20_review_verdict_any",
  "WHERE v.atm_id=x.atm_id AND v.verdict='GO' AND v.review_id > k.rev_hwm", "WHERE v.atm_id=x.atm_id AND v.review_id > k.rev_hwm",
  "a NO-GO review counts as the review"),
 ("M21_review_not_produced_by_the_reviewer",
  "                 AND lower(trim(ve.produced_by))=lower(trim(v.reviewer))   -- the verdict was produced by the reviewer\n", "\n",
  "the review's verdict evidence may have been produced by anyone"),
 ("M22_review_evidence_other_item",
  "JOIN reg_evidence ve ON ve.evidence_id=v.evidence_id AND ve.atm_id=v.atm_id AND ve.kind='review_verdict'",
  "JOIN reg_evidence ve ON ve.evidence_id=v.evidence_id AND ve.kind='review_verdict'",
  "a review may cite another item's verdict evidence"),
 ("M23_review_evidence_kind_not_checked",
  "JOIN reg_evidence ve ON ve.evidence_id=v.evidence_id AND ve.atm_id=v.atm_id AND ve.kind='review_verdict'",
  "JOIN reg_evidence ve ON ve.evidence_id=v.evidence_id AND ve.atm_id=v.atm_id",
  "a review may cite any evidence kind"),
 ("M24_machine_path_ignores_custody_basis",
  "  WHERE c.custody_basis='machine_evidence' AND c.fix_chain_ok AND c.review_ok", "  WHERE c.fix_chain_ok AND c.review_ok",
  "any custody basis may close as Fixed/Implemented/Completed"),
 ("M25_obsolete_path_ignores_proof",
  "    AND c.proof_ok AND c.review_ok;", "    AND c.review_ok;",
  "Obsolete without a false-positive proof row"),
 ("M26_obsolete_path_basis_not_checked",
  "  WHERE c.custody_basis IN ('false_positive_evidence','structural_impossibility','accepted_exception')\n    AND c.proof_ok", "  WHERE c.proof_ok",
  "a machine_evidence item may close as Obsolete"),
 ("M27_proof_kind_not_checked",
  "EXISTS (SELECT 1 FROM reg_evidence f WHERE f.atm_id=x.atm_id AND f.kind='false_positive_proof'", "EXISTS (SELECT 1 FROM reg_evidence f WHERE f.atm_id=x.atm_id",
  "any evidence row counts as the false-positive proof"),
 ("M28_green_evidence_not_in_cycle",
  "             AND e.evidence_id > k.ev_hwm\n             AND (CASE e.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)\n                 >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)) = 1;",
  "             AND (CASE e.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)\n                 >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)) = 1;",
  "GREEN evidence of an earlier cycle counts (the run-row bound still binds)"),
 ("M29_green_class_floor_dropped",
  "             AND (CASE e.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)\n                 >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)) = 1;",
  "             ) = 1;",
  "source-class GREEN evidence satisfies a runtime-layer item"),
 ("M30_red_class_floor_dropped",
  "    AND (CASE e.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)\n        >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END);\nCREATE VIEW IF NOT EXISTS v_green_groups",
  "    ;\nCREATE VIEW IF NOT EXISTS v_green_groups",
  "source-class RED evidence satisfies a runtime-layer item"),
 ("M31_status_log_guard_from_status_not_checked",
  "   WHERE NEW.from_status IS NEW.to_status\n      OR (EXISTS (SELECT 1 FROM reg_status_log WHERE atm_id=NEW.atm_id)\n          AND NEW.from_status IS NOT (SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1));",
  "   WHERE 0;",
  "a status-log row need not continue the last logged status"),
 ("M32_status_log_guard_to_status_not_checked",
  "   WHERE NOT EXISTS (SELECT 1 FROM items WHERE atm_id=NEW.atm_id AND status=NEW.to_status);", "   WHERE 0;",
  "a status-log row need not record the current items status"),
 ("M33_findings_replace_by_fingerprint_allowed",
  "WHEN EXISTS (SELECT 1 FROM reg_findings WHERE finding_seq=NEW.finding_seq OR unit_alias=NEW.unit_alias\n             OR (fingerprint=NEW.fingerprint AND run_id=NEW.run_id))",
  "WHEN EXISTS (SELECT 1 FROM reg_findings WHERE finding_seq=NEW.finding_seq OR unit_alias=NEW.unit_alias)",
  "the (fingerprint, run_id) duplicate path of the replace guard is lost"),
 ("M34_test_runs_replace_by_group_allowed",
  "WHEN EXISTS (SELECT 1 FROM reg_test_runs WHERE run_row=NEW.run_row OR (group_id=NEW.group_id AND rep_index=NEW.rep_index))",
  "WHEN EXISTS (SELECT 1 FROM reg_test_runs WHERE run_row=NEW.run_row)",
  "the (group_id, rep_index) path of the replace guard is lost"),
 ("M35_source_entries_replace_by_locator_allowed",
  "WHEN EXISTS (SELECT 1 FROM reg_source_entries WHERE entry_id=NEW.entry_id OR (source_id=NEW.source_id AND locator=NEW.locator))",
  "WHEN EXISTS (SELECT 1 FROM reg_source_entries WHERE entry_id=NEW.entry_id)",
  "the (source_id, locator) path of the replace guard is lost"),
 # WF3 round 4 (I4): the reviewer's W1-W3, W8-W10 against the fix-round-3 additions (the two views and the I-4 column list)
 ("W1_reopen_unlogged_ge",
  "  HAVING count(*) > (SELECT count(*) FROM reg_status_log s WHERE s.atm_id=h.atm_id AND s.to_status='Reopened');",
  "  HAVING count(*) >= (SELECT count(*) FROM reg_status_log s WHERE s.atm_id=h.atm_id AND s.to_status='Reopened');",
  "a legitimately reopened item (engine reopens == logged reopens) is reported as an unlogged reopen"),
 ("W2_deleted_items_reads_reg_ids",
  "  SELECT DISTINCT atm_id FROM reg_status_log           -- was deleted (the engine only ever moves a row, WF2 I-2)\n",
  "  SELECT DISTINCT atm_id FROM reg_ids                  -- was deleted (the engine only ever moves a row, WF2 I-2)\n",
  "a minted id that has no item yet (the gap between mint and add) is reported as a deleted item"),
 ("W3_reopen_window_dropped",
  "    AND h.created_at >= (SELECT replace(substr(min(f.changed_at),1,19),'T',' ') FROM reg_status_log f WHERE f.atm_id=h.atm_id)\n", "",
  "reopen history that pre-dates the extension's first log row (brownfield, T069 go-live) is reported as unlogged"),
 ("W8_findings_category_editable",
  "location_path, location_line, category, severity, detector, fingerprint, evidence_id ON reg_findings", "location_path, location_line, severity, detector, fingerprint, evidence_id ON reg_findings",
  "an audit finding's category may be edited"),
 ("W9_findings_evidence_id_editable",
  "severity, detector, fingerprint, evidence_id ON reg_findings", "severity, detector, fingerprint ON reg_findings",
  "an audit finding may be re-pointed to other evidence"),
 ("W9b_findings_location_line_editable",
  "location_path, location_line, category, severity, detector, fingerprint, evidence_id ON reg_findings", "location_path, category, severity, detector, fingerprint, evidence_id ON reg_findings",
  "an audit finding's location_line may be edited"),
 ("W10_findings_component_id_editable",
  "run_id, component_id, location_path, location_line, category, severity", "run_id, location_path, location_line, category, severity",
  "an audit finding's component_id may be edited"),
 ("M36_items_insert_guard_legacy_exemption_any_status",
  "     AND NOT (NEW.status LIKE '%(→ Fixed.md)'\n              AND EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=NEW.atm_id)\n              AND IFNULL((SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1),'Queued')='Queued');",
  "     AND NOT (NEW.status LIKE '%(→ Fixed.md)'\n              AND EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=NEW.atm_id));",
  "the reachability select drops its Queued restriction on the legacy exemption"),
]

def main():
    ddl, outdir = sys.argv[1], sys.argv[2]
    text = open(ddl, encoding='utf-8').read()
    os.makedirs(outdir, exist_ok=True)
    rows = []
    n = 0
    for m in re.finditer(r'CREATE TRIGGER IF NOT EXISTS (\w+)[\s\S]*?\bEND;', text):
        n += 1; f = 'm%03d_trigger_%s.sql' % (n, m.group(1))
        open(os.path.join(outdir, f), 'w', encoding='utf-8').write(text[:m.start()] + '-- MUTANT: trigger removed\n' + text[m.end():])
        rows.append((f, 'trigger', m.group(1), ''))
    k = 0
    for (a, b) in checks(text):
        k += 1
        # name the table: last CREATE TABLE before the position
        t = re.findall(r'CREATE TABLE IF NOT EXISTS (\w+)', text[:a])
        tbl = t[-1] if t else '?'
        snippet = re.sub(r'\s+', ' ', text[a:b])[:70]
        f = 'm%03d_check_%s_%d.sql' % (n + k, tbl, k)
        open(os.path.join(outdir, f), 'w', encoding='utf-8').write(text[:a] + 'CHECK (1)' + text[b:])
        rows.append((f, 'check', tbl, snippet))
    c = 0
    for (name, old, new, what) in CLAUSE_MUTANTS:
        cnt = text.count(old)
        if cnt != 1:
            sys.stderr.write('mutate_ddl: clause mutant %s: its text occurs %d times (want exactly 1) - the DDL changed, update CLAUSE_MUTANTS\n' % (name, cnt))
            sys.exit(3)
        c += 1
        f = 'm%03d_clause_%s.sql' % (n + k + c, name)
        mutated = text.replace(old, new, 1)
        if mutated == text:
            sys.stderr.write('mutate_ddl: clause mutant %s is a no-op\n' % name); sys.exit(3)
        open(os.path.join(outdir, f), 'w', encoding='utf-8').write(mutated)
        rows.append((f, 'clause', name, what))
    with open(os.path.join(outdir, 'manifest.tsv'), 'w', encoding='utf-8') as fh:
        for r in rows:
            fh.write('\t'.join(r) + '\n')
    print('triggers=%d checks=%d clauses=%d' % (n, k, c))
main()
