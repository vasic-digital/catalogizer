# inventory_qa_classes.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:00:00Z |
| Status | tracked, new (review fix round 2, F4); nothing here writes any file or the register |
| Source | `scripts/register/tests/inventory_qa_classes.py` |

## Purpose

A member inventory of the QA-instrument classifier (`import_tickets.qa_cause_of`, the one function T168 and T172 share) over a ticket directory. It runs the REAL classifier, never a
re-implementation, over every `*.md` ticket and prints the ticket counts by status, the grouped tickets by (status, cause), and the CANDIDATES the classifier does not group (a
terminal-status ticket whose resolution mentions `infrastructure`, `false positive` or `screenshot`, listed by resolution beginning) so a class it misses is visible.

## Usage

```bash
python3 -I scripts/register/tests/inventory_qa_classes.py [ISSUES_DIR]            # default: the live docs/issues (NOT the frozen snapshot: counts are UNCONFIRMED for the freeze)
python3 -I scripts/register/tests/inventory_qa_classes.py --expect-needles-only    # the control needles only
```

Control needles (constitution 11.4.201 (7)(b)) run first: a planted ticket per member class MUST classify (closed, wontfix, a `## Resolution` body statement, `test infrastructure`)
and three non-members MUST NOT (an open ticket, a closed ticket with a product reason, a QA phrase in a `## Notes` section). A failing needle exits 1 before any count is printed: the
instrument would be blind or too broad. Exit 0 otherwise.

## Honest boundary

It reads the live tree when called without a directory; for the T161 freeze run it on the snapshot directory. It proves which tickets the classifier groups, not that a grouped ticket
is really a QA-instrument defect (that stays the statement of the ticket itself, an analysis and not evidence).
