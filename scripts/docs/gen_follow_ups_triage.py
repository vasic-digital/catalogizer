#!/usr/bin/env python3
"""gen_follow_ups_triage.py - machine-derived triage table of docs/FOLLOW_UPS_NEXT_PHASE.md.

Purpose  Reads the follow-up register (bullet items led by an id), plus `git log` of a ref,
         and prints one table row per id: id, layer, severity, release blocker (yes/no + reason),
         root-cause cluster (id-family + section, NOT a proven root cause), status
         (open / landed / duplicate-of) with the evidence used.
Usage    gen_follow_ups_triage.py [--doc PATH] [--ref REF] [--repo DIR] [--out PATH]
         gen_follow_ups_triage.py --selftest       control needles against the real doc
Rules    Severity is only what the entry (or its section heading) states; otherwise UNKNOWN,
         never guessed (constitution 11.4.6). A layer marked with a trailing `*` is derived from
         keywords or the Type word, not stated. Ids reused in several places are listed per line
         and get no git evidence (ambiguous). Control needles (11.4.273): a known-present id must
         be found, a fabricated id must not; a failed needle exits 2 and writes nothing.
Exit     0 ok, 2 needle failure or unreadable input.
"""
import argparse
import collections
import re
import subprocess
import sys

ID_ONE = r'[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*(?<=\d)[a-z]?'
LEAD = re.compile(r'^(\s*)- (?:\*\*|`)?(' + ID_ONE + r'(?:(?: / |, | and )' + ID_ONE + r')*)(?=[\s:(,.*`)]|$)')
SEV_RE = collections.OrderedDict([
    ('critical', re.compile(r'(?<![\w-])(critical|CRITICAL)(?![\w-])')),
    ('important', re.compile(r'\b(important|IMPORTANT|Important)\b')),
    ('minor', re.compile(r'\b(minor|MINOR|Minor)\b')),
    ('nit', re.compile(r'\b(nits?|NIT)\b')),
])
LAYER_STATED = collections.OrderedDict([
    ('source-defect', re.compile(r'source[- ]defect')),
    ('test-instrumentation', re.compile(r'test[- ]instrumentation')),
    ('process-doc', re.compile(r'process[- ]doc')),
])
DONE_RE = re.compile(r'\((?:[^)]*\b)?(DONE|CLOSED|Closed|closed|fixed|FIXED|landed|LANDED|resolved|RESOLVED|superseded|withdrawn)\b')
DONE_LEAD = re.compile(r'^\s*(?:\(|—|-|:)?\s*(DONE|CLOSED|Closed|closed|LANDED|landed|fixed|FIXED|resolved)\b')
FULL_DONE = re.compile(r'\*\*(?:FIXED|CLOSED|DONE|LANDED)\b|\b(?:CLOSED|FIXED|LANDED|landed|closed) (?:in|by|on|at)\b|\(CLOSED|\(FIXED|\(DONE')
LATER_DONE = re.compile(r'\b(CLOSED|DONE|FIXED|landed|LANDED)\b')
DUP_RE = re.compile(r'(?:duplicate of|same as|merged into|dup of)\s+`?(' + ID_ONE + ')')
SEC_RE = re.compile(r'security|SSRF|authz|CSRF|injection|bypass|privilege|credential|secret|data loss|leak', re.I)
BLOCK_RE = re.compile(r'release[- ]block|blocks the release|\bBLOCKER\b|\bblocking\b', re.I)
REGISTER_COMMIT = re.compile(r'register|follow-ups? (?:registered|added)|record(?:ed)? follow', re.I)


def parse(doc_text):
    """Return list of item dicts for bullet lines led by an id."""
    items, heading, cur = [], '', None
    for n, line in enumerate(doc_text.split('\n'), 1):
        if line.startswith('#'):
            heading, cur = line.lstrip('#').strip(), None
            continue
        m = LEAD.match(line)
        if m:
            ids = re.split(r' / |, | and ', m.group(2))
            cur = {'line': n, 'ids': ids, 'text': line[m.end(1) + 2:].strip(), 'heading': heading}
            items.append(cur)
        elif cur and line.startswith('  ') and line.strip() and not line.lstrip().startswith('- '):
            cur['text'] += ' ' + line.strip()
        elif not line.strip():
            cur = None
        elif line.lstrip().startswith('- '):
            cur = None
    return items


def git_evidence(repo, ref):
    """id-token -> list of (sha, subject) for non-register commits on ref."""
    out = subprocess.run(['git', '-C', repo, 'log', '--format=%h%x09%s', ref],
                         capture_output=True, text=True, check=True).stdout
    toks = collections.defaultdict(list)
    for row in out.splitlines():
        sha, _, subj = row.partition('\t')
        if REGISTER_COMMIT.search(subj):
            continue
        for t in re.findall(ID_ONE, subj):
            toks[t].append((sha, subj))
    return toks


def classify(item, ident, dup_count, gitev, later):
    t, h = item['text'], item['heading']
    head = t[:200]
    # severity: entry first, then section heading; only stated words
    sev, sev_src = 'UNKNOWN', ''
    for k, rx in SEV_RE.items():
        if rx.search(head):
            sev, sev_src = k, 'entry'
            break
    if sev == 'UNKNOWN':
        for k, rx in SEV_RE.items():
            if rx.search(h):
                sev, sev_src = k, 'section'
                break
    layer, derived = 'UNKNOWN', False
    for k, rx in LAYER_STATED.items():
        if rx.search(head) or rx.search(h):
            layer = k
            break
    if layer == 'UNKNOWN':
        derived = True
        if re.search(r'\((?:Bug|bug)\b', head):
            layer = 'source-defect'
        elif re.search(r'mutant|\btests?\b|assert|fixture|census|guard test', head, re.I):
            layer = 'test-instrumentation'
        elif re.search(r'\bdocs?\b|comment|wording|README|\.md\b|reword', head, re.I):
            layer = 'process-doc'
        else:
            derived = False
    # status
    status, ev = 'open', 'no closing word in entry, no landing commit on ref'
    d = DUP_RE.search(t)
    if d and d.group(1) != ident:
        status, ev = 'duplicate-of ' + d.group(1), 'entry text'
    elif DONE_LEAD.match(t) or DONE_RE.search(head):
        status, ev = 'landed', 'entry text states closed/done (PROVISIONAL unless the entry says verified)'
    elif FULL_DONE.search(t):
        status, ev = 'landed', 'entry text (later in the entry) states closed/fixed (PROVISIONAL)'
    elif dup_count == 1 and later.get(ident):
        ln, sn = later[ident]
        status, ev = 'landed', 'doc line %d states closed: "%s" (PROVISIONAL)' % (ln, sn)
    elif dup_count == 1 and '-' in ident and gitev.get(ident):
        sha, subj = gitev[ident][0]
        status, ev = 'landed', 'git %s %s (PROVISIONAL)' % (sha, subj[:80])
    sec = bool(SEC_RE.search(t[:300]))
    # blocker
    if status != 'open':
        blk = ('no', 'not open')
    elif sev == 'critical':
        blk = ('yes', 'severity critical stated')
    elif BLOCK_RE.search(head):
        blk = ('yes', 'entry states release-blocking')
    elif sev == 'important' and (layer == 'source-defect' or sec):
        blk = ('yes', 'important + source-defect or security-adjacent')
    elif sev == 'important':
        blk = ('no', 'important but %s layer, not security-adjacent' % layer)
    elif sev == 'UNKNOWN' and layer == 'source-defect' and sec:
        blk = ('yes', 'source-defect, security-adjacent, severity UNKNOWN: conservative')
    elif sev == 'UNKNOWN':
        blk = ('no', 'severity UNKNOWN (not stated); not security-adjacent' if not sec else 'severity UNKNOWN; security-adjacent word, review')
    else:
        blk = ('no', 'severity %s' % sev)
    fam = re.sub(r'-?[A-Z]?\d+[a-z]?$', '', ident) or ident
    cluster = '%s / %s' % (fam, re.sub(r'\s*\(.*$', '', h)[:48])
    return dict(id=ident, line=item['line'], layer=layer + ('*' if derived else ''),
                sev=sev + ('(sec)' if sev_src == 'section' else ''), sevraw=sev,
                blocker=blk[0], reason=blk[1], cluster=cluster, status=status, evidence=ev, security=sec)


def later_mentions(doc_text, items):
    """id -> (line, snippet) for a non-lead line where the id is followed within 50 chars by a closing word."""
    lead_lines = {it['line'] for it in items}
    res = {}
    for n, line in enumerate(doc_text.split('\n'), 1):
        if n in lead_lines:
            continue
        mc = re.search(r'Closed here: ((?:' + ID_ONE + r'(?: \(|, |\)|$)\s*)+)', line)
        if mc:
            for cid in re.findall(ID_ONE, mc.group(1)):
                res[cid] = (n, line[mc.start():mc.start() + 90].replace('|', '/'))
        for m in re.finditer(r'(?<![\w/.-])(' + ID_ONE + r')(?![\w-])', line):
            w = line[m.end():m.end() + 50]
            if LATER_DONE.search(w) and not re.search(r'\b(open|OPEN|Queued|not)\b', w):
                res[m.group(1)] = (n, line[max(0, m.start() - 10):m.end() + 50].strip()[:90].replace('|', '/'))
    return res


def build(doc_text, repo, ref):
    items = parse(doc_text)
    later = later_mentions(doc_text, items)
    cnt = collections.Counter(i for it in items for i in it['ids'])
    gitev = git_evidence(repo, ref)
    rows = []
    for it in items:
        for ident in it['ids']:
            rows.append(classify(it, ident, cnt[ident], gitev, later))
    return rows


def needles(rows, doc_text):
    ids = {r['id'] for r in rows}
    errs = []
    if 'CAT-PREEXIST-1' not in ids:
        errs.append('positive needle CAT-PREEXIST-1 not found')
    if 'ZZ-NOSUCH-999' in ids or 'ZZ-NOSUCH-999' in doc_text:
        errs.append('negative needle ZZ-NOSUCH-999 found')
    if not any(r['id'].startswith('PX-F') for r in rows):
        errs.append('positive needle family PX-F* not found')
    return errs


def render(rows, ref, sha, doc):
    C = collections.Counter
    op = [r for r in rows if r['status'] == 'open']
    lines = []
    w = lines.append
    w('# Follow-ups triage (machine-derived)\n')
    w('| Field | Value |\n|---|---|')
    w('| Revision | 1 |\n| Created | 2026-11-12 |\n| Status | active (PROVISIONAL: derived by script, not human-verified row by row) |')
    w('| Roadmap-Item | CAT-WP10-TRIAGE |')
    w('| Source | `%s` at `%s` of `%s` |' % (doc, sha, ref))
    w('| Generator | `scripts/docs/gen_follow_ups_triage.py` (test: `scripts/docs/test_gen_follow_ups_triage.py`) |')
    w('| Companion docs | `docs/FOLLOW_UPS_NEXT_PHASE.md`, `docs/WORKABLE_ITEMS_REGISTER.md` |\n')
    w('Do not edit the table by hand; regenerate: `python3 scripts/docs/gen_follow_ups_triage.py --out docs/FOLLOW_UPS_TRIAGE_2026-11-12.md`.\n')
    w('## Table of contents\n\n- [Method and limits](#method-and-limits)\n- [Counts](#counts)\n- [Top blockers](#top-blockers)\n- [Proposed follow-up policy](#proposed-follow-up-policy-not-applied)\n- [Full table](#full-table)\n')
    w('## Method and limits\n')
    w('* Rows are bullet entries of the register that begin with an id (one row per id and line). Ids mentioned only inside prose are not rows. Ids that appear as the lead of several bullets (`F1`, `R1`, ...) are listed once per bullet and get no git evidence.')
    w('* Reused generic ids (`F1`, `F3`, `R1`, ...) have several rows; a closing in a later section ("closed on branch ...") cannot be tied to the right row by id, so such rows may still read `open` although a later section closed them. Review those rows by line before acting.')
    w('* Severity is only what the entry (first 200 characters) or its section heading states (`(sec)` = taken from the section heading). Not stated: `UNKNOWN`. Nothing is inferred.')
    w('* Layer: stated (`source-defect` / `test-instrumentation` / `process-doc`) or, with a trailing `*`, derived from the Type word or keywords (a heuristic).')
    w('* Status: `landed` when the entry says closed/done, or a non-register commit on `%s` names the id (PROVISIONAL: the commit shows the work reached main, not that the register was updated or the fix verified). `duplicate-of` only when the entry says so.' % ref)
    w('* Blocker: `yes` only on a stated critical, a stated release-blocking word, or important + source-defect/security-adjacent, or an UNKNOWN-severity security-adjacent source defect. Everything else `no` with the reason. This is a rule, not a verdict; the operator decides.')
    w('* Cluster: id family plus section title. It groups for batching; it is NOT a proven common root cause.')
    w('* Control needles (11.4.273) passed before this file was written: CAT-PREEXIST-1 present, PX-F* family present, fabricated ZZ-NOSUCH-999 absent.\n')
    w('## Counts\n')
    w('Rows: %d. By status: %s.\n' % (len(rows), ', '.join('%s %d' % (k.split(' ')[0], v) for k, v in sorted(C(r['status'].split(' ')[0] for r in rows).items()))))
    w('Open rows by layer (derived `*` merged): %s.\n' % ', '.join('%s %d' % (k, v) for k, v in sorted(C(r['layer'].rstrip('*') for r in op).items())))
    w('Open rows by severity: %s.\n' % ', '.join('%s %d' % (k, v) for k, v in sorted(C(r['sevraw'] for r in op).items())))
    w('Open layer x severity:\n')
    w('| layer | critical | important | minor | nit | UNKNOWN |\n|---|---|---|---|---|---|')
    for lay in ('source-defect', 'test-instrumentation', 'process-doc', 'UNKNOWN'):
        sub = [r for r in op if r['layer'].rstrip('*') == lay]
        w('| %s | %s |' % (lay, ' | '.join(str(sum(1 for r in sub if r['sevraw'] == s)) for s in ('critical', 'important', 'minor', 'nit', 'UNKNOWN'))))
    blk = [r for r in op if r['blocker'] == 'yes']
    w('\nOpen blockers by rule: %d.  Open security-adjacent words: %d.\n' % (len(blk), sum(1 for r in op if r['security'])))
    w('Open rows per cluster family (top 15):\n')
    fam = C(r['cluster'].split(' / ')[0] for r in op)
    w('| family | open |\n|---|---|')
    for k, v in fam.most_common(15):
        w('| %s | %d |' % (k, v))
    w('\n## Top blockers\n')
    if blk:
        w('| id | line | layer | severity | reason | cluster |\n|---|---|---|---|---|---|')
        for r in blk[:25]:
            w('| %s | %d | %s | %s | %s | %s |' % (r['id'], r['line'], r['layer'], r['sev'], r['reason'], r['cluster'].replace('|', '/')))
    else:
        w('None by the rule.')
    w('\nKnown product blockers recorded elsewhere (docs/WORKABLE_ITEMS_REGISTER.md Part A; not derivable from this register by the rule, listed so the table is not read as clean): `CAT-D9-SUSPEND` (admin suspend is a no-op, important, security, pre-existing) and `UI-FAV-1` (favourites navigate to the wrong entity, important, pre-existing). The table does not replace that list.')
    w('\n## Proposed follow-up policy (not applied)\n')
    w('PROPOSAL for operator decision (11.4.66); no rule file was changed. Text for `docs/FOLLOW_UPS_NEXT_PHASE.md` header or the review-budget policy:\n')
    w('> **Follow-up triage policy (proposed 2026-11-12).** (1) A reviewer finding of severity minor joins an existing open class item of the same root-cause family (same id family and section) as an added line, unless it is security-adjacent (authz, credential, injection, path confinement, data loss, information leak); a security-adjacent minor keeps its own id and is triaged for the next release. (2) A nit is not registered as its own id: it is applied in the same change when trivial, or listed once under the class item; a nit still open after the triage pass for the next release is closed with the reason `nit-after-triage`. (3) Each landed batch registers at most one new item per distinct class; the count of open ids must not grow between consecutive releases except for new increments (11.4.230(E.4) direction rule). (4) Every id carries a stated severity; UNKNOWN is a triage defect to fix at registration.\n')
    w('## Full table\n')
    w('| id | line | layer | severity | blocker | reason | cluster | status | evidence |\n|---|---|---|---|---|---|---|---|---|')
    order = {'yes': 0, 'no': 1}
    for r in sorted(rows, key=lambda r: (r['status'] != 'open', order[r['blocker']], r['line'])):
        w('| %s | %s | %s | %s | %s | %s | %s | %s | %s |' % tuple(
            str(r[k]).replace('|', '/') for k in ('id', 'line', 'layer', 'sev', 'blocker', 'reason', 'cluster', 'status', 'evidence')))
    return '\n'.join(lines) + '\n'


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument('--doc', default='docs/FOLLOW_UPS_NEXT_PHASE.md')
    ap.add_argument('--ref', default='main')
    ap.add_argument('--repo', default='.')
    ap.add_argument('--out')
    ap.add_argument('--selftest', action='store_true')
    a = ap.parse_args(argv)
    try:
        text = open(a.repo + '/' + a.doc, encoding='utf-8').read()
        rows = build(text, a.repo, a.ref)
        sha = subprocess.run(['git', '-C', a.repo, 'rev-parse', '--short', a.ref], capture_output=True, text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError) as e:
        print('ERROR: %s' % e, file=sys.stderr)
        return 2
    errs = needles(rows, text)
    if errs:
        print('NEEDLE FAILURE: %s' % '; '.join(errs), file=sys.stderr)
        return 2
    if a.selftest:
        print('needles ok; rows=%d' % len(rows))
        return 0
    md = render(rows, a.ref, sha, a.doc)
    if a.out:
        open(a.repo + '/' + a.out, 'w', encoding='utf-8').write(md)
    else:
        sys.stdout.write(md)
    return 0


if __name__ == '__main__':
    sys.exit(main())
