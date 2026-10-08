#!/usr/bin/python3 -I
"""verdict_go.py - the ONE predicate for "this review verdict says GO" (WF17-cpa VERDICT-1..5).

Usage   python3 -I verdict_go.py --schema <schema.json> --verdict <verdict.json> [--run-id <id>]
Prints  ok          the verdict validates against the schema, says GO, has blocking_findings of the type integer equal to 0 (a boolean `false` is NOT 0) and, with --run-id, lists that run in covers_runs
        not_covered the same, without the run id in covers_runs
        not_go      valid JSON, valid schema, but not GO / blocking findings present
        invalid     unreadable, not JSON, or not valid against the schema
Exits   0 after printing one of the words; 20 `tool_absent` when jsonschema is not installed: the check is never reduced to "required keys only" (a fail-open fallback would let a
        schema-invalid verdict release a hold; VERDICT-5).
Module  `go(v)` is the schema-free predicate (verdict GO, blocking_findings an int 0) that validate_cheap.sh loads for its table rule.
"""
import json, sys


def go(v):
    if not isinstance(v, dict):
        return False
    bf = v.get('blocking_findings')
    return v.get('verdict') == 'GO' and type(bf) is int and bf == 0


def main(argv):
    schema = verdict = run = None
    i = 0
    while i < len(argv):
        if argv[i] in ('--schema', '--verdict', '--run-id') and i + 1 < len(argv):
            if argv[i] == '--schema': schema = argv[i + 1]
            elif argv[i] == '--verdict': verdict = argv[i + 1]
            else: run = argv[i + 1]
            i += 2
        else:
            sys.stderr.write('verdict_go: usage: --schema F --verdict F [--run-id ID]\n'); return 20
    if not schema or not verdict:
        sys.stderr.write('verdict_go: usage: --schema F --verdict F [--run-id ID]\n'); return 20
    try:
        import jsonschema
    except ImportError:
        sys.stderr.write('verdict_go: tool_absent: jsonschema is required\n'); return 20
    try:
        sc = json.load(open(schema)); v = json.load(open(verdict))
        jsonschema.validate(v, sc)
    except Exception:
        print('invalid'); return 0
    if not go(v):
        print('not_go'); return 0
    if run is None:
        print('ok'); return 0
    print('ok' if any(isinstance(e, dict) and e.get('cpa_run') == run for e in (v.get('covers_runs') or [])) else 'not_covered')
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
