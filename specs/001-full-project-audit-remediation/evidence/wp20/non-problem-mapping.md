# NON-PROBLEM mapping decision (T167, docs/21 IC-39)

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Status | DECISION RECORD, owner approval UNCONFIRMED (docs/21 IC-39 is itself UNCONFIRMED there) |

## Question

How is an entry mapped that a scanner records and that turns out not to be a problem: (a) `relation='false_positive_source'` on an item closed by the non-fix chain, or (b) not scanning such entries and stating the rule.

## What this pass implements (an agent's interim choice, reversible)

Option (a)-compatible and lossless: the lead scan **records** every lead line and labels the ones that are not a problem by rule; nothing is dropped silently. The label is the disposition in `raw_status`:

- `NON-PROBLEM:marker text in document or pattern` - the line names the marker word (`todo`, `fixme`) only as the argument of a shell search command (`grep`, `rg`, `awk`, `sed` before the word, or a command line such as `$ git grep TODO` inside a code fence) and holds no other lead word. A line with another lead word, a bare `- TODO: ...`, or a search command that comes after the marker word is NOT suppressed (WF23 finding C6; tests in `test_lead_scan.py`).
- `duplicate of structured source` - a structured source already holds a row that covers this very line; `raw_severity` names it (`covered_by=<source> :: <entry>`).
- `lead` - everything else.

The importer (T168) maps a `NON-PROBLEM` row with `relation='false_positive_source'` and a `duplicate of structured source` row with `relation='duplicate_of'` to the covering row; that mapping is T168's, not decided here.

## Measured (git archive HEAD scratch snapshot, 2026-10-08, population 2,717 files)

12 NON-PROBLEM rows; 2,718 duplicate rows, every one naming an existing structured row (`test_real_snapshot.py`); 4,056 lead rows. Total 6,786 (independent `grep -i` word-boundary instrument: the same line set).
