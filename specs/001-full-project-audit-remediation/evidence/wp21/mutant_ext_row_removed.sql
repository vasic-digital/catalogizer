-- register_ext.sql : Catalogizer problem-register extension layer, v5 (v1 + review fixes 2026-10-03,
--                    second review round 2026-10-03: §14.8, third review round 2026-10-03: §14.9,
--                    fourth review round 2026-10-03: §14.10; v5 = WF2 review fix round 3, 2026-10-05:
--                    I-1 v_reopen_unlogged, I-2 v_deleted_items, I-4 findings custody; the docs/04 section 5
--                    revision owed for v5 is recorded in evidence/wp06/README.md)
-- Applies ON TOP of the constitution workable-items engine schema (meta.schema_version = 7).
-- PRAGMA foreign_keys is per connection and OFF by default in the sqlite3 shell: this line binds only
-- the connection that applies this file. Every writer MUST open with foreign keys ON (the engine does:
-- db.go:44 `_foreign_keys=on`), and the gate runs PRAGMA foreign_key_check and integrity_check (§12.3).
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS reg_meta (
  key TEXT PRIMARY KEY, value TEXT NOT NULL,
  last_modified TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')));
INSERT OR IGNORE INTO reg_meta(key,value) VALUES ('engine_schema_required','7');
-- ext_schema_version is bumped on re-apply (v4 -> v5): an OR IGNORE would leave a stale version beside new objects
INSERT INTO reg_meta(key,value) VALUES ('ext_schema_version','5')
  ON CONFLICT(key) DO UPDATE SET value=excluded.value, last_modified=strftime('%Y-%m-%dT%H:%M:%SZ','now');

-- identity anchor: one row per CAT id, monotone, never reused (spec FR-001, §11.4.54)
CREATE TABLE IF NOT EXISTS reg_ids (
  seq       INTEGER PRIMARY KEY AUTOINCREMENT,
  atm_id    TEXT GENERATED ALWAYS AS ('CAT-' || printf('%03d', seq)) STORED UNIQUE,
  minted_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  minted_by TEXT NOT NULL CHECK (length(minted_by) > 0),
  mint_basis TEXT NOT NULL CHECK (mint_basis IN ('import','audit_finding','reporting_directive','candidate_duplicate','manual'))
);
CREATE TRIGGER IF NOT EXISTS reg_ids_no_update BEFORE UPDATE ON reg_ids
BEGIN SELECT RAISE(ABORT,'reg_ids is append-only (§11.4.54)'); END;
CREATE TRIGGER IF NOT EXISTS reg_ids_no_delete BEFORE DELETE ON reg_ids
BEGIN SELECT RAISE(ABORT,'reg_ids is append-only (§11.4.54)'); END;
-- INSERT OR REPLACE deletes the conflicting row WITHOUT firing DELETE triggers (recursive_triggers is off
-- by default), so every append-only table also refuses an INSERT whose key already exists (§14.9 I2).
CREATE TRIGGER IF NOT EXISTS reg_ids_no_replace BEFORE INSERT ON reg_ids
WHEN EXISTS (SELECT 1 FROM reg_ids WHERE seq=NEW.seq)
BEGIN SELECT RAISE(ABORT,'reg_ids is append-only: key exists (INSERT OR REPLACE refused)'); END;

CREATE TABLE IF NOT EXISTS reg_components (
  component_id TEXT PRIMARY KEY CHECK (component_id = lower(component_id) AND component_id NOT GLOB '*[^a-z0-9_-]*'),
  kind TEXT NOT NULL CHECK (kind IN ('backend','service','web','desktop','mobile','tv','installer','library','website','build','governance','tooling')),
  path_root TEXT NOT NULL, own_repo INTEGER NOT NULL DEFAULT 0 CHECK (own_repo IN (0,1)));

-- item extension: layer/category/component/custody basis
CREATE TABLE IF NOT EXISTS reg_item_ext (
  atm_id TEXT PRIMARY KEY REFERENCES reg_ids(atm_id),
  category TEXT NOT NULL CHECK (category IN ('bug','error','gap','misalignment','shortcoming','weak_spot','danger_zone','documentation','dependency','test_gap','governance')),
  defect_layer TEXT NOT NULL CHECK (defect_layer IN ('runtime','artifact','source')),
  component_id TEXT REFERENCES reg_components(component_id),
  severity TEXT NOT NULL CHECK (severity IN ('critical','high','medium','low','cosmetic')),
  severity_source TEXT,
  custody_basis TEXT NOT NULL DEFAULT 'machine_evidence' CHECK (custody_basis IN ('machine_evidence','legacy_import','false_positive_evidence','structural_impossibility','accepted_exception')),
  reverify_required INTEGER NOT NULL DEFAULT 0 CHECK (reverify_required IN (0,1)),
  legacy_status TEXT);
-- legacy_import is an import-time fact, never a later edit (§14.8 I-1): it may be written only at
-- INSERT, for an id minted with mint_basis='import' that has never had an items row or a status-log
-- row; afterwards custody_basis can never change TO legacy_import, reverify_required can only be
-- cleared (1 -> 0), atm_id never changes, defect_layer can only be raised (it sets the evidence-class
-- floor, §14.9 I1), and an extension row can never be deleted or replaced.
CREATE TRIGGER IF NOT EXISTS reg_item_ext_no_delete BEFORE DELETE ON reg_item_ext
BEGIN SELECT RAISE(ABORT,'reg_item_ext rows are never deleted (custody floor, §14.9 I1)'); END;
CREATE TRIGGER IF NOT EXISTS reg_item_ext_no_replace BEFORE INSERT ON reg_item_ext
WHEN EXISTS (SELECT 1 FROM reg_item_ext WHERE atm_id=NEW.atm_id)
BEGIN SELECT RAISE(ABORT,'reg_item_ext: row exists (INSERT OR REPLACE refused)'); END;
CREATE TRIGGER IF NOT EXISTS trg_item_ext_legacy_insert BEFORE INSERT ON reg_item_ext
WHEN NEW.custody_basis='legacy_import'
BEGIN
  SELECT RAISE(ABORT,'custody: legacy_import only for an id minted with mint_basis=import')
   WHERE NOT EXISTS (SELECT 1 FROM reg_ids WHERE atm_id=NEW.atm_id AND mint_basis='import');
  -- only an entry the legacy source already reported closed (DR-5 closed class) is imported terminal (§14.10 m-a)
  SELECT RAISE(ABORT,'custody: legacy_import only for a legacy entry whose legacy_status is closed-class (resolved, fixed, closed)')
   WHERE lower(trim(IFNULL(NEW.legacy_status,''))) NOT IN ('resolved','fixed','closed');
  SELECT RAISE(ABORT,'custody: legacy_import only before the id ever had an items row (import time)')
   WHERE EXISTS (SELECT 1 FROM items WHERE atm_id=NEW.atm_id)
      OR EXISTS (SELECT 1 FROM reg_status_log WHERE atm_id=NEW.atm_id);
END;
CREATE TRIGGER IF NOT EXISTS trg_item_ext_custody_update BEFORE UPDATE OF atm_id, custody_basis, reverify_required, defect_layer ON reg_item_ext
BEGIN
  SELECT RAISE(ABORT,'custody: reg_item_ext.atm_id is immutable') WHERE NEW.atm_id IS NOT OLD.atm_id;
  SELECT RAISE(ABORT,'custody: defect_layer can only be raised (source < artifact < runtime), never lowered')
   WHERE (CASE NEW.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)
       < (CASE OLD.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END);
  SELECT RAISE(ABORT,'custody: custody_basis can never be changed TO legacy_import (import-time only)')
   WHERE NEW.custody_basis='legacy_import' AND OLD.custody_basis IS NOT 'legacy_import';
  SELECT RAISE(ABORT,'custody: reverify_required can only be cleared (1 -> 0), never set after insert')
   WHERE NEW.reverify_required=1 AND OLD.reverify_required=0;
END;

CREATE TABLE IF NOT EXISTS reg_sources (
  source_id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL CHECK (kind IN ('issue_file','md_tracker','report_doc','qa_bank','qa_results','workable_items_db','external_ticket','constitution_conflict','code_marker')),
  locator TEXT NOT NULL UNIQUE, parser TEXT NOT NULL, content_sha256 TEXT,
  last_scanned_at TEXT, scanned_entry_count INTEGER);

CREATE TABLE IF NOT EXISTS reg_source_entries (
  entry_id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_id INTEGER NOT NULL REFERENCES reg_sources(source_id),
  locator TEXT NOT NULL, legacy_id TEXT, title TEXT, raw_status TEXT, raw_severity TEXT,
  entry_sha256 TEXT NOT NULL CHECK (length(entry_sha256)=64),
  scanned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (source_id, locator));
CREATE INDEX IF NOT EXISTS idx_src_entries_legacy ON reg_source_entries(legacy_id);
-- FR-002 "nothing dropped": an entry, once scanned, is never deleted or replaced (a DELETE of the entry
-- together with its mapping would leave v_unmapped_entries clean, §14.10 m-d)
CREATE TRIGGER IF NOT EXISTS reg_source_entries_no_delete BEFORE DELETE ON reg_source_entries
BEGIN SELECT RAISE(ABORT,'reg_source_entries: a scanned entry is never deleted (FR-002)'); END;
-- RAISE(IGNORE), not ABORT: the Stage 0 rescan uses INSERT OR IGNORE and must stay idempotent (§14.4);
-- a REPLACE of an existing entry is skipped, so the scanned row is kept unchanged
CREATE TRIGGER IF NOT EXISTS reg_source_entries_no_replace BEFORE INSERT ON reg_source_entries
WHEN EXISTS (SELECT 1 FROM reg_source_entries WHERE entry_id=NEW.entry_id OR (source_id=NEW.source_id AND locator=NEW.locator))
BEGIN SELECT RAISE(IGNORE); END;

CREATE TABLE IF NOT EXISTS reg_source_map (
  entry_id INTEGER PRIMARY KEY REFERENCES reg_source_entries(entry_id),
  atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  relation TEXT NOT NULL CHECK (relation IN ('primary','duplicate_of','sibling','refers_to','false_positive_source')),
  match_basis TEXT NOT NULL CHECK (match_basis IN ('ticket','normalised(subject,scope)','manual_operator','import_1to1')),
  severity_governs INTEGER NOT NULL DEFAULT 0 CHECK (severity_governs IN (0,1)),
  mapped_by TEXT NOT NULL, mapped_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')));
CREATE INDEX IF NOT EXISTS idx_source_map_atm ON reg_source_map(atm_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_source_map_severity ON reg_source_map(atm_id) WHERE severity_governs = 1;
-- a mapping may be corrected (UPDATE, or INSERT OR REPLACE of the same entry_id) but never removed
CREATE TRIGGER IF NOT EXISTS reg_source_map_no_delete BEFORE DELETE ON reg_source_map
BEGIN SELECT RAISE(ABORT,'reg_source_map: a mapping is corrected by UPDATE, never deleted (FR-002)'); END;

CREATE TABLE IF NOT EXISTS reg_audit_runs (
  run_id TEXT PRIMARY KEY CHECK (run_id GLOB 'RUN-[0-9]*'),
  started_at TEXT NOT NULL, git_head TEXT NOT NULL CHECK (length(git_head)=40),
  index_attestation_path TEXT NOT NULL, tool_versions TEXT NOT NULL);

-- finding identity: ONE canonical id FND-NNNN minted here (monotone, never reused, like reg_ids);
-- the audit file's unit-local id F-<unit>-NNN is stored as unit_alias (unique, unit-checked). research.md R-12.
CREATE TABLE IF NOT EXISTS reg_findings (
  finding_seq INTEGER PRIMARY KEY AUTOINCREMENT,
  finding_id TEXT GENERATED ALWAYS AS ('FND-' || printf('%04d', finding_seq)) STORED UNIQUE,
  unit_alias TEXT NOT NULL UNIQUE,
  atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  run_id TEXT NOT NULL REFERENCES reg_audit_runs(run_id),
  component_id TEXT NOT NULL REFERENCES reg_components(component_id),
  location_path TEXT NOT NULL CHECK (length(location_path)>0), location_line INTEGER,
  category TEXT NOT NULL CHECK (category IN ('bug','error','gap','misalignment','shortcoming','weak_spot','danger_zone','documentation','dependency','test_gap','governance')),
  severity TEXT NOT NULL CHECK (severity IN ('critical','high','medium','low','cosmetic')),
  detector TEXT NOT NULL, fingerprint TEXT NOT NULL CHECK (length(fingerprint)=64),
  evidence_id INTEGER NOT NULL REFERENCES reg_evidence(evidence_id) DEFERRABLE INITIALLY DEFERRED,
  UNIQUE (fingerprint, run_id),
  CHECK (unit_alias GLOB ('F-' || component_id || '-[0-9][0-9][0-9]*')
         AND substr(unit_alias, length(component_id) + 4) NOT GLOB '*[^0-9]*'));
CREATE TRIGGER IF NOT EXISTS reg_findings_id_no_update BEFORE UPDATE OF finding_seq, unit_alias ON reg_findings
BEGIN SELECT RAISE(ABORT,'reg_findings ids are immutable (§11.4.54)'); END;
CREATE TRIGGER IF NOT EXISTS reg_findings_no_replace BEFORE INSERT ON reg_findings
WHEN EXISTS (SELECT 1 FROM reg_findings WHERE finding_seq=NEW.finding_seq OR unit_alias=NEW.unit_alias
             OR (fingerprint=NEW.fingerprint AND run_id=NEW.run_id))
BEGIN SELECT RAISE(ABORT,'reg_findings: key exists (INSERT OR REPLACE refused, §11.4.54)'); END;
CREATE INDEX IF NOT EXISTS idx_findings_atm ON reg_findings(atm_id);
-- a finding is one recorded audit OBSERVATION: it is never deleted, and no column that states what was
-- observed can be edited (a lowered severity, a re-pointed atm_id or a rewritten fingerprint would silently
-- change the audit's primary output, WF2 review I-4). A correction is a NEW finding plus a recurrence link.
CREATE TRIGGER IF NOT EXISTS reg_findings_no_delete BEFORE DELETE ON reg_findings
BEGIN SELECT RAISE(ABORT,'reg_findings: a finding is never deleted (record a new finding or a recurrence link)'); END;
CREATE TRIGGER IF NOT EXISTS reg_findings_observation_no_update
BEFORE UPDATE OF atm_id, run_id, component_id, location_path, location_line, category, severity, detector, fingerprint, evidence_id ON reg_findings
BEGIN SELECT RAISE(ABORT,'reg_findings: an audit observation is never edited (atm_id, severity, fingerprint, location, detector, evidence)'); END;

CREATE TABLE IF NOT EXISTS reg_evidence (
  evidence_id INTEGER PRIMARY KEY AUTOINCREMENT,
  atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  kind TEXT NOT NULL CHECK (kind IN ('repro','red_run','green_run','mutation_run','artifact','log','screenshot','review_verdict','sibling_search','guard_verdict','tracker_receipt','index_attestation','false_positive_proof','custody_decision')),
  evidence_class TEXT NOT NULL CHECK (evidence_class IN ('runtime','artifact','source')),
  path TEXT NOT NULL CHECK (length(path)>0),
  sha256 TEXT NOT NULL CHECK (length(sha256)=64 AND sha256 NOT GLOB '*[^0-9a-f]*'),
  size_bytes INTEGER NOT NULL CHECK (size_bytes>0),
  target_fingerprint TEXT, polarity TEXT CHECK (polarity IN ('RED','GREEN')),
  exit_code INTEGER, iterations INTEGER CHECK (iterations IS NULL OR iterations>=1),
  precondition_provenance TEXT CHECK (precondition_provenance IN ('observed','constructed')),
  produced_by TEXT NOT NULL, captured_at TEXT NOT NULL,
  CHECK (evidence_class <> 'runtime' OR (target_fingerprint IS NOT NULL AND length(target_fingerprint)>0)),
  CHECK ((kind IN ('red_run','green_run')) = (polarity IS NOT NULL)),
  CHECK (kind <> 'red_run' OR (polarity='RED' AND exit_code IS NOT NULL AND exit_code BETWEEN 1 AND 125)),
  CHECK (kind <> 'green_run' OR (polarity='GREEN' AND exit_code IS NOT NULL AND exit_code=0 AND iterations IS NOT NULL AND iterations>=3)),
  CHECK (kind <> 'mutation_run' OR (exit_code IS NOT NULL AND exit_code BETWEEN 1 AND 125)));  -- the mutant made the test FAIL
CREATE INDEX IF NOT EXISTS idx_evidence_atm ON reg_evidence(atm_id, kind);
CREATE TRIGGER IF NOT EXISTS reg_evidence_no_update BEFORE UPDATE ON reg_evidence
BEGIN SELECT RAISE(ABORT,'reg_evidence is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_evidence_no_delete BEFORE DELETE ON reg_evidence
BEGIN SELECT RAISE(ABORT,'reg_evidence is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_evidence_no_replace BEFORE INSERT ON reg_evidence
WHEN EXISTS (SELECT 1 FROM reg_evidence WHERE evidence_id=NEW.evidence_id)
BEGIN SELECT RAISE(ABORT,'reg_evidence is append-only: key exists (INSERT OR REPLACE refused)'); END;

CREATE TABLE IF NOT EXISTS reg_test_types (type_code TEXT PRIMARY KEY, label TEXT NOT NULL);
INSERT OR IGNORE INTO reg_test_types VALUES ('unit','Unit'),('integration','Integration'),('e2e','End to end'),('full_automation','Full automation'),('security','Security'),('ddos','DDoS'),('scaling','Scaling'),('chaos','Chaos'),('stress','Stress'),('performance','Performance'),('benchmark','Benchmarking'),('ui','UI'),('ux','UX'),('challenge','Challenge'),('helixqa','HelixQA session'),('contract','Contract');

CREATE TABLE IF NOT EXISTS reg_test_runs (
  run_row INTEGER PRIMARY KEY AUTOINCREMENT,
  atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  test_id TEXT NOT NULL, type_code TEXT NOT NULL REFERENCES reg_test_types(type_code),
  group_id TEXT NOT NULL, rep_index INTEGER NOT NULL CHECK (rep_index>=1),
  polarity TEXT NOT NULL CHECK (polarity IN ('RED','GREEN','MUTATION')),
  verdict TEXT NOT NULL CHECK (verdict IN ('PASS','FAIL','BLOCKED')),
  -- closed set = ev/1 blocked_reason enum (contracts/evidence-record.schema.json); for a BLOCKED row the
  -- test command was not run and the evidence row carries the failing precondition probe's exit status
  -- (non-zero) as a kind='log' row, never a red_run or green_run (§14.9 M8)
  blocked_reason TEXT CHECK (blocked_reason IN ('service_unreachable','credential_absent','credential_rejected',
    'device_absent','device_wrong_identity','device_unauthorised','geo_restricted','quota_exhausted',
    'licence_absent','host_resource_unavailable')),
  target_fingerprint TEXT NOT NULL CHECK (length(target_fingerprint)>0),
  container_image_digest TEXT, evidence_id INTEGER NOT NULL REFERENCES reg_evidence(evidence_id),
  started_at TEXT NOT NULL, UNIQUE (group_id, rep_index),
  CHECK ((verdict='BLOCKED') = (blocked_reason IS NOT NULL)),
  CHECK (polarity <> 'RED'   OR verdict IN ('FAIL','BLOCKED')),
  CHECK (polarity <> 'GREEN' OR verdict = 'PASS'));   -- a GREEN repetition is never BLOCKED (ev/1 requires pass)
CREATE TRIGGER IF NOT EXISTS reg_test_runs_no_update BEFORE UPDATE ON reg_test_runs
BEGIN SELECT RAISE(ABORT,'reg_test_runs is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_test_runs_no_delete BEFORE DELETE ON reg_test_runs
BEGIN SELECT RAISE(ABORT,'reg_test_runs is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_test_runs_no_replace BEFORE INSERT ON reg_test_runs
WHEN EXISTS (SELECT 1 FROM reg_test_runs WHERE run_row=NEW.run_row OR (group_id=NEW.group_id AND rep_index=NEW.rep_index))
BEGIN SELECT RAISE(ABORT,'reg_test_runs is append-only: key exists (INSERT OR REPLACE refused)'); END;

CREATE TABLE IF NOT EXISTS reg_reviews (
  review_id INTEGER PRIMARY KEY AUTOINCREMENT,
  atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id), author TEXT NOT NULL, reviewer TEXT NOT NULL,
  model TEXT NOT NULL, effort TEXT NOT NULL, verdict TEXT NOT NULL CHECK (verdict IN ('GO','NO-GO')),
  evidence_id INTEGER NOT NULL REFERENCES reg_evidence(evidence_id), reviewed_at TEXT NOT NULL,
  CHECK (lower(trim(author)) <> lower(trim(reviewer))));   -- 'alice' and ' Alice' are one identity
CREATE TRIGGER IF NOT EXISTS reg_reviews_no_update BEFORE UPDATE ON reg_reviews
BEGIN SELECT RAISE(ABORT,'reg_reviews is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_reviews_no_delete BEFORE DELETE ON reg_reviews
BEGIN SELECT RAISE(ABORT,'reg_reviews is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_reviews_no_replace BEFORE INSERT ON reg_reviews
WHEN EXISTS (SELECT 1 FROM reg_reviews WHERE review_id=NEW.review_id)
BEGIN SELECT RAISE(ABORT,'reg_reviews is append-only: key exists (INSERT OR REPLACE refused)'); END;

CREATE TABLE IF NOT EXISTS reg_recurrence_links (
  link_id INTEGER PRIMARY KEY AUTOINCREMENT,
  head_atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  reported_entry_id INTEGER REFERENCES reg_source_entries(entry_id),
  reported_finding_id TEXT REFERENCES reg_findings(finding_id),
  intake_path TEXT NOT NULL CHECK (intake_path IN ('reporting-directive','gate-failure','manual-qa','audit-rerun','import')),
  verdict TEXT NOT NULL CHECK (verdict IN ('SAME_DEFECT','DISTINCT','UNDECIDED')),
  match_basis TEXT NOT NULL CHECK (match_basis IN ('ticket','normalised(subject,scope)','manual_operator')),
  new_atm_id TEXT REFERENCES reg_ids(atm_id), reopened INTEGER NOT NULL DEFAULT 0 CHECK (reopened IN (0,1)),
  -- decision point: the head's last reg_status_log id when the link was written (checked by the guard below);
  -- v_recurrence_violations compares log ids against it, never the self-declared decided_at (§14.10 I3)
  head_log_id INTEGER NOT NULL,
  decided_by TEXT NOT NULL, decided_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK (reported_entry_id IS NOT NULL OR reported_finding_id IS NOT NULL),
  CHECK (verdict <> 'SAME_DEFECT' OR new_atm_id IS NULL),
  CHECK (verdict <> 'UNDECIDED' OR new_atm_id IS NOT NULL),
  CHECK (new_atm_id IS NULL OR new_atm_id <> head_atm_id));
-- a recurrence decision is a record, not a setting: never updated, deleted or replaced. A changed decision is
-- a NEW row and every row binds (a SAME_DEFECT row on a terminal head keeps requiring the reopen, §8).
CREATE TRIGGER IF NOT EXISTS reg_recurrence_links_no_update BEFORE UPDATE ON reg_recurrence_links
BEGIN SELECT RAISE(ABORT,'reg_recurrence_links is append-only (§11.4.214)'); END;
CREATE TRIGGER IF NOT EXISTS reg_recurrence_links_no_delete BEFORE DELETE ON reg_recurrence_links
BEGIN SELECT RAISE(ABORT,'reg_recurrence_links is append-only (§11.4.214)'); END;
CREATE TRIGGER IF NOT EXISTS reg_recurrence_links_no_replace BEFORE INSERT ON reg_recurrence_links
WHEN EXISTS (SELECT 1 FROM reg_recurrence_links WHERE link_id=NEW.link_id)
BEGIN SELECT RAISE(ABORT,'reg_recurrence_links is append-only: key exists (INSERT OR REPLACE refused)'); END;

CREATE TABLE IF NOT EXISTS reg_status_transitions (from_status TEXT NOT NULL, to_status TEXT NOT NULL, PRIMARY KEY (from_status,to_status));
INSERT OR IGNORE INTO reg_status_transitions VALUES
 ('Queued','In progress'),('Queued','Operator-blocked'),('Queued','Obsolete (→ Fixed.md)'),
 ('In progress','Ready for testing'),('In progress','Operator-blocked'),('In progress','Queued'),
 ('Ready for testing','In testing'),('Ready for testing','In progress'),
 ('In testing','Fixed (→ Fixed.md)'),('In testing','Implemented (→ Fixed.md)'),('In testing','Completed (→ Fixed.md)'),('In testing','In progress'),
 ('Operator-blocked','Queued'),('Operator-blocked','In progress'),
 ('Fixed (→ Fixed.md)','Reopened'),('Implemented (→ Fixed.md)','Reopened'),('Completed (→ Fixed.md)','Reopened'),('Obsolete (→ Fixed.md)','Reopened'),
 ('Reopened','In progress'),('Reopened','Operator-blocked'),('Reopened','Obsolete (→ Fixed.md)');

-- ev_hwm, run_hwm, rev_hwm: the highest reg_evidence / reg_test_runs / reg_reviews id that existed when the
-- row was written. The ids are AUTOINCREMENT and those ledgers are append-only, so "id > hwm of the item's
-- last Reopened row" means "recorded after the reopen": that is the cycle boundary the custody views use
-- (§14.9 B1). SQLite cannot assign NEW columns in a BEFORE trigger, so the writer supplies the values and
-- the guard below refuses any other value.
CREATE TABLE IF NOT EXISTS reg_status_log (
  log_id INTEGER PRIMARY KEY AUTOINCREMENT, atm_id TEXT NOT NULL, from_status TEXT, to_status TEXT NOT NULL,
  changed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')), session_actor TEXT,
  ev_hwm INTEGER NOT NULL DEFAULT 0, run_hwm INTEGER NOT NULL DEFAULT 0, rev_hwm INTEGER NOT NULL DEFAULT 0);
CREATE TRIGGER IF NOT EXISTS reg_status_log_no_update BEFORE UPDATE ON reg_status_log
BEGIN SELECT RAISE(ABORT,'reg_status_log is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_status_log_no_delete BEFORE DELETE ON reg_status_log
BEGIN SELECT RAISE(ABORT,'reg_status_log is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_status_log_no_replace BEFORE INSERT ON reg_status_log
WHEN EXISTS (SELECT 1 FROM reg_status_log WHERE log_id=NEW.log_id)
BEGIN SELECT RAISE(ABORT,'reg_status_log is append-only: key exists (INSERT OR REPLACE refused)'); END;
-- the log is trusted by trg_items_insert_guard, so a raw INSERT must not be able to forge it (§14.8):
-- a row is legal only when it records the CURRENT items status, continues the last logged status,
-- is a real change, and carries the current ledger high-water marks (the items triggers below are the
-- only writers that satisfy all four).
CREATE TRIGGER IF NOT EXISTS reg_status_log_insert_guard BEFORE INSERT ON reg_status_log
BEGIN
  SELECT RAISE(ABORT,'reg_status_log: to_status must equal the current items status of the id')
   WHERE NOT EXISTS (SELECT 1 FROM items WHERE atm_id=NEW.atm_id AND status=NEW.to_status);
  SELECT RAISE(ABORT,'reg_status_log: from_status must equal the last logged to_status, and differ from to_status')
   WHERE NEW.from_status IS NEW.to_status
      OR (EXISTS (SELECT 1 FROM reg_status_log WHERE atm_id=NEW.atm_id)
          AND NEW.from_status IS NOT (SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1));
  SELECT RAISE(ABORT,'reg_status_log: ev_hwm/run_hwm/rev_hwm must equal the current ledger maxima (cycle boundary)')
   WHERE NEW.ev_hwm  IS NOT (SELECT IFNULL(max(evidence_id),0) FROM reg_evidence)
      OR NEW.run_hwm IS NOT (SELECT IFNULL(max(run_row),0) FROM reg_test_runs)
      OR NEW.rev_hwm IS NOT (SELECT IFNULL(max(review_id),0) FROM reg_reviews);
END;
-- a reopened legacy item is no longer "imported closed, awaiting re-verification": the reopen is the
-- re-verification outcome. It becomes an ordinary machine-evidence item, so its next closure needs a
-- full chain recorded after the reopen (§14.9 B2).
CREATE TRIGGER IF NOT EXISTS trg_status_log_reopen_legacy AFTER INSERT ON reg_status_log
WHEN NEW.to_status='Reopened'
BEGIN
  UPDATE reg_item_ext SET custody_basis='machine_evidence', reverify_required=0
   WHERE atm_id=NEW.atm_id AND custody_basis='legacy_import';
END;
-- start of the item's current fix cycle: its last Reopened log row (0 when never reopened)
CREATE VIEW IF NOT EXISTS v_cycle_start AS
  SELECT r.atm_id, IFNULL(l.log_id,0) AS log_id, IFNULL(l.ev_hwm,0) AS ev_hwm,
         IFNULL(l.run_hwm,0) AS run_hwm, IFNULL(l.rev_hwm,0) AS rev_hwm
  FROM reg_ids r LEFT JOIN reg_status_log l
    ON l.log_id = (SELECT max(log_id) FROM reg_status_log WHERE atm_id=r.atm_id AND to_status='Reopened');
-- replay guard (§14.10 I1): the cycle boundary compares ids, so a NEW row that is a copy of an earlier
-- cycle's file (same path, same sha256) would count as current-cycle evidence. A custody-bearing evidence
-- row whose (atm_id, sha256) already exists at or below the item's cycle mark is refused: the file was
-- already part of a consumed cycle. Within one cycle a repeated sha256 is allowed (it cannot cross the mark).
CREATE TRIGGER IF NOT EXISTS reg_evidence_no_replay BEFORE INSERT ON reg_evidence
WHEN NEW.kind IN ('red_run','green_run','mutation_run','review_verdict','custody_decision','false_positive_proof')
BEGIN
  SELECT RAISE(ABORT,'custody: evidence file (same sha256) already recorded for this item in an earlier fix cycle (replay refused)')
   WHERE EXISTS (SELECT 1 FROM reg_evidence p WHERE p.atm_id=NEW.atm_id AND p.sha256=NEW.sha256
                 AND p.evidence_id <= (SELECT ev_hwm FROM v_cycle_start WHERE atm_id=NEW.atm_id));
END;
CREATE TRIGGER IF NOT EXISTS reg_recurrence_links_head_guard BEFORE INSERT ON reg_recurrence_links
BEGIN
  SELECT RAISE(ABORT,'recurrence: head_log_id must equal the head''s current last reg_status_log id (decision point)')
   WHERE NEW.head_log_id IS NOT (SELECT IFNULL(max(log_id),0) FROM reg_status_log WHERE atm_id=NEW.head_atm_id);
END;
-- the import-time legacy exemption, defined once (§14.10 m-a): a legacy_import row still awaiting
-- re-verification whose item was never Reopened and never moved into work (In progress, Ready for
-- testing, In testing, Operator-blocked). An item imported open and later worked on is an ordinary item.
CREATE VIEW IF NOT EXISTS v_legacy_exempt AS
  SELECT x.atm_id FROM reg_item_ext x
  WHERE x.custody_basis='legacy_import' AND x.reverify_required=1
    AND NOT EXISTS (SELECT 1 FROM reg_status_log s WHERE s.atm_id=x.atm_id
                    AND s.to_status IN ('Reopened','In progress','Ready for testing','In testing','Operator-blocked'));

CREATE TABLE IF NOT EXISTS reg_closure_decisions (
  decision_id INTEGER PRIMARY KEY AUTOINCREMENT, atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  to_status TEXT NOT NULL CHECK (to_status IN ('Fixed (→ Fixed.md)','Implemented (→ Fixed.md)','Completed (→ Fixed.md)','Obsolete (→ Fixed.md)')),
  decision TEXT NOT NULL CHECK (decision IN ('ACCEPTED','REFUSED')),
  decision_json_evidence_id INTEGER NOT NULL REFERENCES reg_evidence(evidence_id),
  decided_at TEXT NOT NULL, consumed_at TEXT);
CREATE TRIGGER IF NOT EXISTS reg_closure_decisions_no_delete BEFORE DELETE ON reg_closure_decisions
BEGIN SELECT RAISE(ABORT,'reg_closure_decisions is append-only'); END;
CREATE TRIGGER IF NOT EXISTS reg_closure_decisions_no_replace BEFORE INSERT ON reg_closure_decisions
WHEN EXISTS (SELECT 1 FROM reg_closure_decisions WHERE decision_id=NEW.decision_id)
BEGIN SELECT RAISE(ABORT,'reg_closure_decisions: key exists (INSERT OR REPLACE refused; a consumed decision cannot be revived)'); END;
CREATE TRIGGER IF NOT EXISTS reg_closure_decisions_consume_only BEFORE UPDATE ON reg_closure_decisions
WHEN NOT (OLD.consumed_at IS NULL AND NEW.consumed_at IS NOT NULL AND NEW.decision_id=OLD.decision_id
          AND NEW.atm_id=OLD.atm_id AND NEW.to_status=OLD.to_status AND NEW.decision=OLD.decision
          AND NEW.decision_json_evidence_id=OLD.decision_json_evidence_id AND NEW.decided_at=OLD.decided_at)
BEGIN SELECT RAISE(ABORT,'reg_closure_decisions: only a single consumption (consumed_at NULL -> set) is allowed'); END;

-- ---------- custody chain views (consumed by the triggers below and by the gate) ----------
-- class rank: runtime 3 > artifact 2 > source 1; an evidence row satisfies an item only when its
-- class rank >= the rank of the item's defect_layer (§11.4.226 floor).
-- cycle rule (§14.9 B1): every test run, evidence row and review counted below was recorded AFTER the
-- item's last Reopened log row (id > the high-water mark stored on that row, v_cycle_start), so rows of an
-- earlier, already-consumed fix cycle do not count. A new row that copies an earlier cycle's file is refused
-- by reg_evidence_no_replay (same sha256), and a GREEN group on a fingerprint that was GREEN in an earlier
-- cycle does not count (§14.10 I1). A re-recording with altered bytes and a new self-declared fingerprint is
-- NOT detectable in SQL (producer = verifier residual, §5 limitation 3).
CREATE VIEW IF NOT EXISTS v_red_runs AS            -- genuine RED: test FAILED (exit 1..125) on the pre-fix artifact
  SELECT t.atm_id, t.test_id, t.target_fingerprint AS fp, e.produced_by
  FROM reg_test_runs t
  JOIN reg_evidence e ON e.evidence_id=t.evidence_id AND e.atm_id=t.atm_id
  JOIN reg_item_ext x ON x.atm_id=t.atm_id
  JOIN v_cycle_start k ON k.atm_id=t.atm_id
  WHERE t.polarity='RED' AND t.verdict='FAIL' AND e.kind='red_run' AND e.polarity='RED'
    AND t.run_row > k.run_hwm AND e.evidence_id > k.ev_hwm
    AND e.exit_code BETWEEN 1 AND 125 AND e.precondition_provenance='observed'
    AND e.target_fingerprint=t.target_fingerprint
    AND (CASE e.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)
        >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END);
CREATE VIEW IF NOT EXISTS v_green_groups AS        -- >=3 repetitions, all PASS, one fingerprint, class floor met
  SELECT t.atm_id, t.test_id, t.group_id, min(t.target_fingerprint) AS fp,
         group_concat(DISTINCT e.produced_by) AS producers
  FROM reg_test_runs t
  JOIN reg_evidence e ON e.evidence_id=t.evidence_id AND e.atm_id=t.atm_id
  JOIN reg_item_ext x ON x.atm_id=t.atm_id
  JOIN v_cycle_start k ON k.atm_id=t.atm_id
  WHERE t.polarity='GREEN' AND t.run_row > k.run_hwm
    AND t.target_fingerprint NOT IN (SELECT o.target_fingerprint FROM reg_test_runs o      -- never GREEN in an
          WHERE o.atm_id=t.atm_id AND o.polarity='GREEN' AND o.run_row <= k.run_hwm)        -- earlier cycle (§14.10 I1)
  GROUP BY t.atm_id, t.test_id, t.group_id
  HAVING count(DISTINCT t.rep_index) >= 3 AND count(DISTINCT t.target_fingerprint) = 1
     AND min(t.verdict='PASS' AND e.kind='green_run' AND e.exit_code=0 AND e.target_fingerprint=t.target_fingerprint
             AND e.evidence_id > k.ev_hwm
             AND (CASE e.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)
                 >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)) = 1;
CREATE VIEW IF NOT EXISTS v_closure_chain AS       -- one row per extended item, one flag per custody link
  SELECT x.atm_id, x.custody_basis,
    EXISTS (SELECT 1 FROM v_red_runs r WHERE r.atm_id=x.atm_id) AS red_ok,
    EXISTS (SELECT 1 FROM v_red_runs r JOIN v_green_groups g ON g.atm_id=r.atm_id AND g.test_id=r.test_id AND g.fp<>r.fp
            WHERE r.atm_id=x.atm_id) AS green_ok,
    EXISTS (SELECT 1 FROM v_red_runs r
            JOIN v_green_groups g ON g.atm_id=r.atm_id AND g.test_id=r.test_id AND g.fp<>r.fp
            JOIN reg_test_runs m ON m.atm_id=r.atm_id AND m.test_id=r.test_id AND m.polarity='MUTATION' AND m.verdict='FAIL'
                 AND m.run_row > k.run_hwm
            JOIN reg_evidence me ON me.evidence_id=m.evidence_id AND me.atm_id=m.atm_id AND me.kind='mutation_run'
                 AND me.exit_code BETWEEN 1 AND 125 AND me.evidence_id > k.ev_hwm
                 AND (CASE me.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)
                     >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)
            WHERE r.atm_id=x.atm_id) AS fix_chain_ok,      -- same test_id for RED, GREEN and the caught mutation;
                                                           -- the mutation's evidence is a mutation_run of THIS item
    EXISTS (SELECT 1 FROM reg_reviews v
            JOIN reg_evidence ve ON ve.evidence_id=v.evidence_id AND ve.atm_id=v.atm_id AND ve.kind='review_verdict'
                 AND lower(trim(ve.produced_by))=lower(trim(v.reviewer))   -- the verdict was produced by the reviewer
                 AND ve.evidence_id > k.ev_hwm
            WHERE v.atm_id=x.atm_id AND v.verdict='GO' AND v.review_id > k.rev_hwm   -- reviewed in this cycle
              AND lower(trim(v.author))<>lower(trim(v.reviewer))
              AND v.review_id=(SELECT max(review_id) FROM reg_reviews WHERE atm_id=x.atm_id)
              AND NOT EXISTS (SELECT 1 FROM reg_evidence p WHERE p.atm_id=x.atm_id
                              AND p.kind IN ('red_run','green_run','false_positive_proof','custody_decision')
                              AND lower(trim(p.produced_by))=lower(trim(v.reviewer)))) AS review_ok,
                              -- latest review is GO and recorded in this cycle, its verdict evidence is the
                              -- reviewer's, and the reviewer produced none of the item's RED/GREEN/proof/decision
                              -- evidence in ANY cycle (deliberately stricter than the cycle rule; case- and
                              -- space-insensitive)
    EXISTS (SELECT 1 FROM reg_evidence f WHERE f.atm_id=x.atm_id AND f.kind='false_positive_proof'
            AND f.evidence_id > k.ev_hwm
            AND (CASE f.evidence_class WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)
                >= (CASE x.defect_layer WHEN 'runtime' THEN 3 WHEN 'artifact' THEN 2 ELSE 1 END)) AS proof_ok
  FROM reg_item_ext x JOIN v_cycle_start k ON k.atm_id=x.atm_id;
CREATE VIEW IF NOT EXISTS v_closure_ready AS       -- (item, terminal status) pairs whose chain is complete
  SELECT c.atm_id, s.to_status FROM v_closure_chain c
  JOIN (SELECT 'Fixed (→ Fixed.md)' AS to_status UNION ALL SELECT 'Implemented (→ Fixed.md)' UNION ALL SELECT 'Completed (→ Fixed.md)') s
  WHERE c.custody_basis='machine_evidence' AND c.fix_chain_ok AND c.review_ok
  UNION ALL
  SELECT c.atm_id, 'Obsolete (→ Fixed.md)' FROM v_closure_chain c
  WHERE c.custody_basis IN ('false_positive_evidence','structural_impossibility','accepted_exception')
    AND c.proof_ok AND c.review_ok;
CREATE VIEW IF NOT EXISTS v_live_decisions AS      -- an ACCEPTED, unconsumed decision that is backed by a custody_decision
  SELECT d.decision_id, d.atm_id, d.to_status     -- evidence row of the same item AND by a complete chain
  FROM reg_closure_decisions d
  JOIN reg_evidence e ON e.evidence_id=d.decision_json_evidence_id AND e.atm_id=d.atm_id AND e.kind='custody_decision'
  JOIN v_cycle_start k ON k.atm_id=d.atm_id AND e.evidence_id > k.ev_hwm      -- decided in this cycle
  JOIN v_closure_ready r ON r.atm_id=d.atm_id AND r.to_status=d.to_status
  WHERE d.decision='ACCEPTED' AND d.consumed_at IS NULL;
CREATE TRIGGER IF NOT EXISTS trg_closure_decision_guard BEFORE INSERT ON reg_closure_decisions
WHEN NEW.decision='ACCEPTED'
BEGIN
  SELECT RAISE(ABORT,'custody: decision_json_evidence_id must be a custody_decision evidence row of the same item, recorded in the current cycle')
   WHERE NOT EXISTS (SELECT 1 FROM reg_evidence e WHERE e.evidence_id=NEW.decision_json_evidence_id
                     AND e.atm_id=NEW.atm_id AND e.kind='custody_decision'
                     AND e.evidence_id > (SELECT ev_hwm FROM v_cycle_start WHERE atm_id=NEW.atm_id));
  SELECT RAISE(ABORT,'custody: chain incomplete for ACCEPTED decision (RED, GREEN x3, caught mutation, independent GO review; FR-008, §11.4.115(F)/.226/.240)')
   WHERE NOT EXISTS (SELECT 1 FROM v_closure_ready r WHERE r.atm_id=NEW.atm_id AND r.to_status=NEW.to_status);
END;

-- identity guards on the engine's items table (§11.4.54, FR-001). The engine's own PK
-- (atm_id, current_location, representation) admits the same id in Issues and Fixed (engine tombstones);
-- in the register every id is ONE row. The engine's close/reopen move with DELETE-then-INSERT, so a
-- BEFORE INSERT check that no row with the id exists does not disturb them (executed, §14.7).
CREATE TRIGGER IF NOT EXISTS trg_items_require_mint BEFORE INSERT ON items
BEGIN
  SELECT RAISE(ABORT,'identity: atm_id has no reg_ids row; mint first (§11.4.54)')
   WHERE NOT EXISTS (SELECT 1 FROM reg_ids WHERE atm_id=NEW.atm_id);
  SELECT RAISE(ABORT,'identity: atm_id already has an items row (one row per register id across Issues/Fixed)')
   WHERE EXISTS (SELECT 1 FROM items WHERE atm_id=NEW.atm_id);
END;
CREATE TRIGGER IF NOT EXISTS trg_items_identity_update BEFORE UPDATE OF atm_id, current_location, representation ON items
BEGIN
  SELECT RAISE(ABORT,'identity: atm_id has no reg_ids row; mint first (§11.4.54)')
   WHERE NOT EXISTS (SELECT 1 FROM reg_ids WHERE atm_id=NEW.atm_id);
  SELECT RAISE(ABORT,'identity: atm_id already has another items row')
   WHERE EXISTS (SELECT 1 FROM items WHERE atm_id=NEW.atm_id AND rowid<>OLD.rowid);
END;

-- transition + custody enforcement on the engine's items table.
-- The engine moves a row Issues->Fixed with DELETE+INSERT (crud.go:354,381), so BOTH
-- UPDATE-of-status and INSERT of a terminal status are guarded.
CREATE TRIGGER IF NOT EXISTS trg_items_transition BEFORE UPDATE OF status ON items
WHEN OLD.status <> NEW.status
BEGIN
  SELECT RAISE(ABORT,'status transition not in reg_status_transitions')
   WHERE NOT EXISTS (SELECT 1 FROM reg_status_transitions WHERE from_status=OLD.status AND to_status=NEW.status);
  SELECT RAISE(ABORT,'custody: no live decision (ACCEPTED, unconsumed, custody_decision evidence, complete chain) (§11.4.146(D3))')
   WHERE NEW.status LIKE '%(→ Fixed.md)'
     AND NOT EXISTS (SELECT 1 FROM v_live_decisions d WHERE d.atm_id=NEW.atm_id AND d.to_status=NEW.status);
END;
-- every insert (not only a terminal one) must be reachable from the last logged status, because the engine
-- (and a raw writer) moves rows with DELETE + INSERT and DELETE on items cannot be guarded (§5 limitation 6,
-- §14.9 I3). The import-time legacy exemption (v_legacy_exempt) applies only to a closed-class legacy entry
-- whose item was never Reopened (§14.9 B2) and never worked on, and only from Queued (§14.10 m-a).
CREATE TRIGGER IF NOT EXISTS trg_items_insert_guard BEFORE INSERT ON items
BEGIN
  SELECT RAISE(ABORT,'transition: insert not reachable from the last logged status')
   WHERE (SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1) IS NOT NULL
     AND (SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1) <> NEW.status
     AND NOT EXISTS (SELECT 1 FROM reg_status_transitions t WHERE t.to_status=NEW.status
          AND t.from_status=(SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1))
     AND NOT (NEW.status LIKE '%(→ Fixed.md)'
              AND EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=NEW.atm_id)
              AND IFNULL((SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1),'Queued')='Queued');
  SELECT RAISE(ABORT,'custody: terminal insert without live decision (custody_decision evidence + complete chain in the current cycle) or import-time legacy basis (§11.4.146(D3))')
   WHERE NEW.status LIKE '%(→ Fixed.md)'
     AND (SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1) IS NOT NEW.status
     AND NOT EXISTS (SELECT 1 FROM v_live_decisions d WHERE d.atm_id=NEW.atm_id AND d.to_status=NEW.status)
     AND NOT (EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=NEW.atm_id)
              AND IFNULL((SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1),'Queued')='Queued');
     -- v2 also exempted Obsolete when an obsolete_details row existed; that exemption was unreachable through
     -- the engine and open to a raw writer, and was removed in the second review round (§14.8 I-4).
END;
CREATE TRIGGER IF NOT EXISTS trg_items_status_log AFTER UPDATE OF status ON items
WHEN OLD.status <> NEW.status
BEGIN
  INSERT INTO reg_status_log(atm_id,from_status,to_status,ev_hwm,run_hwm,rev_hwm)
  VALUES (NEW.atm_id,OLD.status,NEW.status,(SELECT IFNULL(max(evidence_id),0) FROM reg_evidence),
          (SELECT IFNULL(max(run_row),0) FROM reg_test_runs),(SELECT IFNULL(max(review_id),0) FROM reg_reviews));
  UPDATE reg_closure_decisions SET consumed_at=strftime('%Y-%m-%dT%H:%M:%SZ','now')
   WHERE atm_id=NEW.atm_id AND to_status=NEW.status AND decision='ACCEPTED' AND consumed_at IS NULL;
END;
CREATE TRIGGER IF NOT EXISTS trg_items_insert_log AFTER INSERT ON items
WHEN IFNULL((SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1),'') <> NEW.status
BEGIN
  INSERT INTO reg_status_log(atm_id,from_status,to_status,ev_hwm,run_hwm,rev_hwm)
  VALUES (NEW.atm_id,(SELECT to_status FROM reg_status_log WHERE atm_id=NEW.atm_id ORDER BY log_id DESC LIMIT 1),NEW.status,
          (SELECT IFNULL(max(evidence_id),0) FROM reg_evidence),(SELECT IFNULL(max(run_row),0) FROM reg_test_runs),
          (SELECT IFNULL(max(review_id),0) FROM reg_reviews));
  UPDATE reg_closure_decisions SET consumed_at=strftime('%Y-%m-%dT%H:%M:%SZ','now')
   WHERE atm_id=NEW.atm_id AND to_status=NEW.status AND decision='ACCEPTED' AND consumed_at IS NULL;
END;

CREATE TABLE IF NOT EXISTS reg_trackers (
  tracker_id TEXT PRIMARY KEY CHECK (tracker_id=lower(tracker_id)), kind TEXT NOT NULL,
  command_ref TEXT, required_env TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)));
CREATE TABLE IF NOT EXISTS reg_tracker_sync_log (
  sync_id INTEGER PRIMARY KEY AUTOINCREMENT,
  tracker_id TEXT NOT NULL REFERENCES reg_trackers(tracker_id), atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id),
  status TEXT NOT NULL CHECK (status IN ('SYNCED','SKIPPED','FAILED')),
  skip_reason TEXT CHECK (skip_reason IN ('credentials_absent','tracker_client_absent','unreachable','not_configured','disabled_by_operator')),
  missing_env_names TEXT, exit_code INTEGER, remote_ref TEXT, evidence_id INTEGER REFERENCES reg_evidence(evidence_id),
  item_revision TEXT NOT NULL, attempted_at TEXT NOT NULL,
  CHECK ((status='SKIPPED') = (skip_reason IS NOT NULL)),
  CHECK (status <> 'SYNCED' OR (exit_code=0 AND remote_ref IS NOT NULL AND evidence_id IS NOT NULL)),
  CHECK (status <> 'FAILED' OR (exit_code IS NOT NULL AND exit_code<>0)));
CREATE INDEX IF NOT EXISTS idx_sync_tracker_atm ON reg_tracker_sync_log(tracker_id, atm_id, sync_id);

CREATE TABLE IF NOT EXISTS reg_export_runs (
  export_id INTEGER PRIMARY KEY AUTOINCREMENT, db_fingerprint TEXT NOT NULL, engine_version TEXT NOT NULL,
  container_image_digest TEXT, started_at TEXT NOT NULL, verdict TEXT NOT NULL CHECK (verdict IN ('OK','STALE','FAILED')));
CREATE TABLE IF NOT EXISTS reg_export_files (
  export_id INTEGER NOT NULL REFERENCES reg_export_runs(export_id), path TEXT NOT NULL,
  format TEXT NOT NULL CHECK (format IN ('md','html','pdf','docx')), sha256 TEXT NOT NULL CHECK (length(sha256)=64),
  source_path TEXT NOT NULL, status TEXT NOT NULL CHECK (status IN ('written','skipped_tool_absent')), PRIMARY KEY (export_id,path));

-- ---------- views ----------
CREATE VIEW IF NOT EXISTS v_open_items AS SELECT * FROM items WHERE current_location='Issues';
CREATE VIEW IF NOT EXISTS v_closed_items AS SELECT * FROM items WHERE current_location='Fixed';
CREATE VIEW IF NOT EXISTS v_issues_summary AS
  SELECT type, status, count(*) AS n FROM items WHERE current_location='Issues' GROUP BY type, status;
CREATE VIEW IF NOT EXISTS v_fixed_summary AS
  SELECT type, status, count(*) AS n FROM items WHERE current_location='Fixed' GROUP BY type, status;
CREATE VIEW IF NOT EXISTS v_unmapped_entries AS
  SELECT e.entry_id, s.locator AS source, e.locator FROM reg_source_entries e JOIN reg_sources s USING(source_id)
  WHERE e.entry_id NOT IN (SELECT entry_id FROM reg_source_map);
CREATE VIEW IF NOT EXISTS v_reconciliation AS
  SELECT s.kind, s.locator AS source, e.locator AS entry_locator, e.legacy_id, m.relation, m.atm_id, i.status
  FROM reg_source_entries e JOIN reg_sources s USING(source_id)
  LEFT JOIN reg_source_map m USING(entry_id) LEFT JOIN items i ON i.atm_id=m.atm_id AND i.current_location IN ('Issues','Fixed');
CREATE VIEW IF NOT EXISTS v_legacy_id_collisions AS
  SELECT legacy_id, count(*) AS entries, count(DISTINCT title) AS distinct_titles FROM reg_source_entries
  WHERE legacy_id IS NOT NULL GROUP BY legacy_id HAVING count(*)>1;
CREATE VIEW IF NOT EXISTS v_reopen_counts AS
  SELECT atm_id, count(*) AS reopens FROM reg_status_log WHERE to_status='Reopened' GROUP BY atm_id ORDER BY reopens DESC;
CREATE VIEW IF NOT EXISTS v_findings_without_item AS
  SELECT f.finding_id FROM reg_findings f WHERE f.atm_id NOT IN (SELECT atm_id FROM items);
CREATE VIEW IF NOT EXISTS v_custody_violations AS   -- terminal items whose chain is not complete (Seam B sweep)
  SELECT i.atm_id, i.status FROM items i
  WHERE i.status LIKE '%(→ Fixed.md)'
    AND NOT EXISTS (SELECT 1 FROM v_legacy_exempt x WHERE x.atm_id=i.atm_id)
    AND NOT EXISTS (SELECT 1 FROM v_closure_ready r WHERE r.atm_id=i.atm_id AND r.to_status=i.status);
CREATE VIEW IF NOT EXISTS v_stale_tracker_sync AS
  SELECT t.tracker_id, i.atm_id,
         (SELECT status FROM reg_tracker_sync_log l WHERE l.tracker_id=t.tracker_id AND l.atm_id=i.atm_id ORDER BY sync_id DESC LIMIT 1) AS last_status
  FROM reg_trackers t CROSS JOIN (SELECT DISTINCT atm_id FROM items) i WHERE t.enabled=1;
CREATE VIEW IF NOT EXISTS v_reverify_queue AS
  SELECT x.atm_id, x.legacy_status, i.status, x.severity FROM reg_item_ext x JOIN items i ON i.atm_id=x.atm_id
  WHERE x.reverify_required=1 ORDER BY CASE x.severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, x.atm_id;
CREATE VIEW IF NOT EXISTS v_recurrence_violations AS
  -- verified against reg_status_log, never against the self-reported `reopened` flag, and positioned by the
  -- guarded head_log_id (log ids), never by the self-declared decided_at (§14.10 I3):
  -- (a) SAME_DEFECT whose head had a terminal log row at or before the decision point and no later Reopened row;
  -- (b) a link that claims reopened=1 without any Reopened log row after the last closure before the decision point.
  SELECT l.link_id, l.head_atm_id, 'terminal head not reopened' AS problem
  FROM reg_recurrence_links l
  WHERE l.verdict='SAME_DEFECT'
    AND (SELECT max(log_id) FROM reg_status_log s WHERE s.atm_id=l.head_atm_id
          AND s.to_status LIKE '%(→ Fixed.md)' AND s.log_id <= l.head_log_id) IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM reg_status_log s2 WHERE s2.atm_id=l.head_atm_id AND s2.to_status='Reopened'
          AND s2.log_id > (SELECT max(log_id) FROM reg_status_log s WHERE s.atm_id=l.head_atm_id
                            AND s.to_status LIKE '%(→ Fixed.md)' AND s.log_id <= l.head_log_id))
  UNION ALL
  SELECT l.link_id, l.head_atm_id, 'reopened flag not backed by status log'
  FROM reg_recurrence_links l
  WHERE l.reopened=1 AND NOT EXISTS (SELECT 1 FROM reg_status_log s WHERE s.atm_id=l.head_atm_id AND s.to_status='Reopened'
          AND s.log_id > IFNULL((SELECT max(log_id) FROM reg_status_log t WHERE t.atm_id=l.head_atm_id
                                 AND t.to_status LIKE '%(→ Fixed.md)' AND t.log_id <= l.head_log_id), 0));
CREATE VIEW IF NOT EXISTS v_replayed_evidence AS     -- second line for reg_evidence_no_replay (a dropped trigger):
  SELECT n.evidence_id, n.atm_id, n.kind, o.evidence_id AS earlier_evidence_id   -- custody-bearing row whose file
  FROM reg_evidence n JOIN reg_evidence o                                        -- was already recorded before a
    ON o.atm_id=n.atm_id AND o.sha256=n.sha256 AND o.evidence_id < n.evidence_id -- Reopened row that precedes it
  WHERE n.kind IN ('red_run','green_run','mutation_run','review_verdict','custody_decision','false_positive_proof')
    AND EXISTS (SELECT 1 FROM reg_status_log r WHERE r.atm_id=n.atm_id AND r.to_status='Reopened'
                AND o.evidence_id <= r.ev_hwm AND n.evidence_id > r.ev_hwm);
CREATE VIEW IF NOT EXISTS v_duplicate_item_ids AS      -- same register id in more than one items row
  SELECT atm_id, count(*) AS rows_n, group_concat(current_location) AS locations FROM items GROUP BY atm_id HAVING count(*) > 1;
CREATE VIEW IF NOT EXISTS v_items_without_mint AS      -- items row whose id was never minted in reg_ids
  SELECT atm_id, current_location FROM items WHERE atm_id NOT IN (SELECT atm_id FROM reg_ids);
CREATE VIEW IF NOT EXISTS v_deleted_items AS           -- an id that has status-log rows but no items row: the item
  SELECT DISTINCT atm_id FROM reg_status_log           -- was deleted (the engine only ever moves a row, WF2 I-2)
  WHERE atm_id NOT IN (SELECT atm_id FROM items);
CREATE VIEW IF NOT EXISTS v_reopen_unlogged AS         -- the engine recorded a Reopened event (item_history) that the
  SELECT h.atm_id, count(*) AS engine_reopens,         -- register's own log lacks: a log writer was silenced (WF2 I-1).
         (SELECT count(*) FROM reg_status_log s WHERE s.atm_id=h.atm_id AND s.to_status='Reopened') AS logged_reopens
  FROM item_history h                                  -- only events at or after the id's first log row count, so
  WHERE h.event_type='Reopened'                        -- reopen history that pre-dates the extension is not a violation
    AND EXISTS (SELECT 1 FROM reg_status_log f WHERE f.atm_id=h.atm_id)
    AND h.created_at >= (SELECT replace(substr(min(f.changed_at),1,19),'T',' ') FROM reg_status_log f WHERE f.atm_id=h.atm_id)
  GROUP BY h.atm_id
  HAVING count(*) > (SELECT count(*) FROM reg_status_log s WHERE s.atm_id=h.atm_id AND s.to_status='Reopened');
CREATE VIEW IF NOT EXISTS v_ids_without_item AS        -- minted id never inserted (reported, never recycled)
  SELECT atm_id FROM reg_ids WHERE atm_id NOT IN (SELECT atm_id FROM items);
CREATE VIEW IF NOT EXISTS v_illegal_logged_edges AS   -- logged status changes that are not edges of the graph
  SELECT l.log_id, l.atm_id, l.from_status, l.to_status FROM reg_status_log l   -- (a raw DELETE + INSERT, §14.9 I3)
  WHERE l.from_status IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM reg_status_transitions t WHERE t.from_status=l.from_status AND t.to_status=l.to_status)
    AND NOT (l.to_status LIKE '%(→ Fixed.md)' AND l.from_status='Queued'   -- the import-time legacy closure
             AND EXISTS (SELECT 1 FROM reg_ids r WHERE r.atm_id=l.atm_id AND r.mint_basis='import')  -- (Queued -> terminal)
             AND NOT EXISTS (SELECT 1 FROM reg_status_log p WHERE p.atm_id=l.atm_id AND p.log_id<l.log_id   -- before any
                             AND p.to_status IN ('Reopened','In progress','Ready for testing','In testing','Operator-blocked')));  -- reopen or work
CREATE VIEW IF NOT EXISTS v_legacy_import_unbacked AS  -- legacy basis that the import did not produce
  SELECT x.atm_id, r.mint_basis FROM reg_item_ext x JOIN reg_ids r ON r.atm_id=x.atm_id
  WHERE x.custody_basis='legacy_import'
    AND (r.mint_basis <> 'import'
         OR NOT EXISTS (SELECT 1 FROM reg_source_map m WHERE m.atm_id=x.atm_id AND m.relation='primary'));

-- escape-ratchet records (constitution §11.4.238 extension; designed in docs/12 §12, canonical DDL here)
CREATE TABLE IF NOT EXISTS reg_cycle (
  cycle_id TEXT PRIMARY KEY, manual_qa_ran INTEGER NOT NULL CHECK (manual_qa_ran IN (0,1)));
CREATE TABLE IF NOT EXISTS reg_discovery (
  finding_id TEXT PRIMARY KEY REFERENCES reg_findings(finding_id),
  cycle_id   TEXT NOT NULL REFERENCES reg_cycle(cycle_id),
  channel    TEXT NOT NULL CHECK (channel IN ('automated_seam','manual_qa','operator','end_user','agent_inspection')),
  should_have_been_caught_by TEXT NOT NULL CHECK (length(should_have_been_caught_by)>0),
  none_justification TEXT,
  recorded_by TEXT NOT NULL, producer TEXT NOT NULL,
  CHECK (lower(trim(recorded_by)) <> lower(trim(producer))),
  CHECK (should_have_been_caught_by <> 'none' OR length(replace(replace(replace(coalesce(none_justification,''),' ',''),char(9),''),char(10),''))>=20));
CREATE TABLE IF NOT EXISTS reg_escape_baseline (
  cycle_id TEXT PRIMARY KEY REFERENCES reg_cycle(cycle_id), escapes INTEGER NOT NULL CHECK (escapes>=0));
CREATE VIEW IF NOT EXISTS v_escapes AS
  SELECT c.cycle_id, c.manual_qa_ran,
    sum(d.channel<>'automated_seam' AND d.should_have_been_caught_by<>'none') AS escapes,
    sum(d.channel<>'automated_seam' AND d.should_have_been_caught_by='none')  AS escapes_none_tally,
    sum(d.channel='automated_seam') AS discovered_by_qa
  FROM reg_cycle c LEFT JOIN reg_discovery d USING(cycle_id) GROUP BY c.cycle_id;

-- gate registry: the closure / register gate (scripts/register/gate.sh, proposed) reads this table;
-- every view_empty row must return 0 rows, every trigger_present row must exist in sqlite_master.
-- view_not_done: a row is open work; the register gate passes, the feature completion gate (§13.3) counts
-- every row as NOT done.
CREATE TABLE IF NOT EXISTS reg_gate_checks (
  name TEXT PRIMARY KEY, kind TEXT NOT NULL CHECK (kind IN ('view_empty','trigger_present','view_report','view_not_done')));
INSERT OR IGNORE INTO reg_gate_checks VALUES
 ('v_duplicate_item_ids','view_empty'),('v_items_without_mint','view_empty'),('v_custody_violations','view_empty'),
 ('v_recurrence_violations','view_empty'),('v_findings_without_item','view_empty'),('v_unmapped_entries','view_empty'),
 ('v_ids_without_item','view_report'),('v_legacy_import_unbacked','view_empty'),('v_escapes','view_report'),
 ('reg_ids_no_update','trigger_present'),('reg_ids_no_delete','trigger_present'),
 ('reg_evidence_no_update','trigger_present'),('reg_evidence_no_delete','trigger_present'),
 ('reg_status_log_no_update','trigger_present'),('reg_status_log_no_delete','trigger_present'),
 ('reg_findings_id_no_update','trigger_present'),
 ('reg_closure_decisions_no_delete','trigger_present'),('reg_closure_decisions_consume_only','trigger_present'),
 ('trg_closure_decision_guard','trigger_present'),('trg_items_require_mint','trigger_present'),
 ('trg_items_identity_update','trigger_present'),('trg_items_transition','trigger_present'),
 ('trg_items_insert_guard','trigger_present'),('trg_items_status_log','trigger_present'),('trg_items_insert_log','trigger_present'),
 ('trg_item_ext_legacy_insert','trigger_present'),('trg_item_ext_custody_update','trigger_present'),
 ('reg_status_log_insert_guard','trigger_present'),
 ('reg_test_runs_no_update','trigger_present'),('reg_test_runs_no_delete','trigger_present'),
 ('reg_reviews_no_update','trigger_present'),('reg_reviews_no_delete','trigger_present'),
 ('v_illegal_logged_edges','view_empty'),
 ('reg_ids_no_replace','trigger_present'),('reg_item_ext_no_delete','trigger_present'),('reg_item_ext_no_replace','trigger_present'),
 ('reg_findings_no_replace','trigger_present'),('reg_evidence_no_replace','trigger_present'),('reg_test_runs_no_replace','trigger_present'),
 ('reg_reviews_no_replace','trigger_present'),('reg_status_log_no_replace','trigger_present'),
 ('reg_closure_decisions_no_replace','trigger_present'),('trg_status_log_reopen_legacy','trigger_present'),
 ('reg_evidence_no_replay','trigger_present'),('reg_recurrence_links_head_guard','trigger_present'),
 ('reg_recurrence_links_no_update','trigger_present'),('reg_recurrence_links_no_delete','trigger_present'),
 ('reg_recurrence_links_no_replace','trigger_present'),('reg_source_entries_no_delete','trigger_present'),
 ('reg_source_entries_no_replace','trigger_present'),('reg_source_map_no_delete','trigger_present'),
 ('v_replayed_evidence','view_empty'),('v_legacy_exempt','view_report'),
 ('reg_findings_no_delete','trigger_present'),('reg_findings_observation_no_update','trigger_present'),
 ('v_deleted_items','view_empty'),('v_reopen_unlogged','view_empty');
CREATE VIEW IF NOT EXISTS v_gate_missing_objects AS    -- a registered check whose object is absent (dropped trigger/view)
  SELECT g.name, g.kind FROM reg_gate_checks g
  WHERE NOT EXISTS (SELECT 1 FROM sqlite_master m WHERE m.name=g.name
                    AND m.type = CASE g.kind WHEN 'trigger_present' THEN 'trigger' ELSE 'view' END);
