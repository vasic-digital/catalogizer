#!/usr/bin/env python3
"""gen_follow_ups_triage.py - machine-derived triage table of docs/FOLLOW_UPS_NEXT_PHASE.md.

Purpose  Reads the follow-up register (bullet items led by an id), plus `git log` of a ref,
         and prints one table row per id: id, layer, severity, release blocker (yes/no + reason),
         root-cause cluster (id-family + section, NOT a proven root cause), status
         (open / landed / unconfirmed / duplicate-of) with the evidence used. `unconfirmed` is
         NOT closed: it is weak evidence only (a later prose mention, a closing word that is not at
         the start of the entry, a partial entry with an open remainder).
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
ID_LEAD = r'(?:' + ID_ONE + r'|[A-Z][A-Z0-9]*-[A-Z0-9]*\d[A-Z0-9]*(?:-[A-Z][A-Z0-9]*)+)'   # also CAT-D9-SUSPEND (digit inside)
LEAD = re.compile(r'^(\s*)- (?:\*\*|`)?(' + ID_LEAD + r'(?:(?: / |, | and )' + ID_LEAD + r')*)(?=[\s:(,.*`)]|$)')
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
CLOSE_TOK = r'(?:PROVISIONAL-DONE|FIXED|LANDED|CLOSED|DONE|RESOLVED|\[LANDED\])'
# strong: the entry itself OPENS with a closing token (never a noun such as "fixed members")
# a closing token is a STATUS only when it is followed by punctuation / end / a preposition / "(" - never when it is the
# subject of a sentence ("PROVISIONAL-DONE entries ...", "FIXED members ...")  [WP10R4-1]
CLOSE_AFTER = r'(?=\s*[:;,.)\]]|\s*$|\s*\(|\s+(?:in|by|on|at|here|via|as|with|for)\b)'
START_CLOSE = re.compile(r'^(?:\[LANDED\]|(?:' + CLOSE_TOK + r'|(?:fixed|closed|landed|done|resolved))' + CLOSE_AFTER + r')')
OPEN_ANY_CS = re.compile(r'\b(?:OPEN|OWED|Queued|PARTIAL|PARTLY)\b|\bOpen:')
# remainder detector, case-insensitive: a landed claim whose own text qualifies it (round-3 class: "landed claim negated or qualified by its own text")
REMAIN_RE = re.compile(
    r'\bstill\s+(?:open|unguarded|unfixed|missing|last-write-wins|queued|owed|pending|unprotected|unchecked|unconfirmed|vulnerable|broken|failing)\b'
    r'|\bnot\s+(?:re-?checked|covered|yet|fixed|handled|guarded|addressed|done|run|waived|measured|verified|tested|wired|implemented|exercised)\b'
    r'|\bstill\s+(?:not|no)\b|\b(?:no|none)\b[^.;:]{0,30}\byet\b'
    r'|\b(?:follow-ups?|remaining|residual|caveats?|limitations?|known gaps?)\s*:'
    r'|\([a-d1-3]\)\s+\S|(?:^|\s)[1-3]\.\s+\S'
    r'|\boperator decision\b[^.]{0,60}\b(?:open|pending|owed|needed|required)\b'
    r'|\bfor the \w+ side\b|\bpartial(?:ly)?\b|\bresidual\b|\bunfixed\b|\bunguarded\b'
    r'|\b(?:fixes|covers|handles) only\b|\bonly the \w+ (?:side|half|part|leg)\b', re.I)
class _Open:
    @staticmethod
    def search(t):
        return OPEN_ANY_CS.search(t) or REMAIN_RE.search(t)
OPEN_ANY = _Open()
NEG_BEFORE, NEG_AFTER = 40, 40      # characters scanned around a closing word  [WP10R4-8]
NEG_RE = re.compile(r'\bNO-GO\b|\bnot\b|\bnever\b|\bpending\b|\bblocked\b|\bwithheld\b|\bun(?:landed|fixed|resolved)\b', re.I)
DONE_HEAD = re.compile(r'\b(DONE|FIXED|LANDED|CLOSED|RESOLVED|SUPERSEDED|PROVISIONAL-DONE)\b|\[LANDED\]')
OPEN_HEAD = re.compile(r'STILL OPEN|\bOPEN\b|\bOWED\b|\bQueued\b|\[[^\]]*\b(?:open|queued|owed)\b[^\]]*\]')
FULL_DONE = re.compile(r'\*\*(?:FIXED|CLOSED|DONE|LANDED)\b|\b(?:CLOSED|FIXED|LANDED|landed|closed) (?:in|by|on|at)\b|\(CLOSED|\(FIXED|\(DONE')
LATER_DONE = re.compile(r'\b(CLOSED|DONE|FIXED|landed|LANDED)\b')
DUP_RE = re.compile(r'(?:duplicate of|same as|merged into|dup of)\s+`?(' + ID_ONE + ')')
SEC_RE = re.compile(r'security|SSRF|authz|CSRF|injection|bypass|privilege|credential|secret|data loss|leak|IDOR', re.I)
BLOCK_RE = re.compile(r'release[- ]block|blocks the release|\bBLOCKER\b|\bblocking\b', re.I)
REGISTER_COMMIT = re.compile(r'register|follow-ups? (?:registered|added)|record(?:ed)? follow', re.I)
LABEL_LANDED = re.compile(r'^(Fixed|Closed|Landed|Done|Resolved)\b[^:]{0,80}:\s*$')
LABEL_OPEN = re.compile(r'^(Still open|Open|Remaining|Not fixed|Open follow-ups)\b[^:]{0,80}:\s*$')
RANGE_RE = re.compile(r'(?P<pre>[A-Z][A-Z0-9-]*?)(?P<a>\d+)\.\.(?:[A-Z][A-Z0-9-]*?)?(?P<b>\d+)')
HEAD_DONE = re.compile(r'\b(landed|closed|resolved|DONE)\b', re.I)


def expand_ranges(text):
    """All ids named by ranges such as PX-F1..F7 or D7-R2-F1..F4 in text (cap 60 per range)."""
    res = []
    for m in RANGE_RE.finditer(text):
        a, b = int(m.group('a')), int(m.group('b'))
        if 0 <= b - a < 60:
            res += [m.group('pre') + str(k) for k in range(a, b + 1)]
    return res


SLASH_RE = re.compile(r'(?<![\w/.-])(' + ID_ONE + r')((?:/[A-Z]*\d+[a-z]?)+)(?![\w.-])')


def expand_slash(text):
    """Ids named by slash lists such as CAT-PREEXIST-7/8/9 or PW-F5/F6 (the sibling replaces the base id's trailing number,
    with its letters when it carries some). The base id is returned first; a slash with no id before it yields nothing."""
    res = []
    for m in SLASH_RE.finditer(text):
        base = m.group(1)
        res.append(base)
        for sib in m.group(2).split('/')[1:]:
            pre = re.sub(r'[A-Z]*\d+[a-z]?$' if re.match(r'[A-Z]', sib) else r'\d+[a-z]?$', '', base)
            res.append(pre + sib)
    return res


def starts_closed(t):
    """True when the entry text (after the id) opens with a closing token: directly, after a leading (...) group,
    or as an element of that group, e.g. `(DONE): ...`, `(Task, FIXED)`, `: FIXED in ...`. Nouns ("fixed members") do not count."""
    t = t.lstrip(' *`:-')
    pm = re.match(r'\(([^)]*)\)', t)
    if pm:
        for el in re.split(r'[,;]', pm.group(1)):
            if re.match(r'^(?:' + CLOSE_TOK + r')' + CLOSE_AFTER, el.strip()):
                return True
        t = t[pm.end():]
    t = t.lstrip(' *`:,.-')
    return bool(START_CLOSE.match(t))


def letters(ident):
    """Letters of the family prefix (before the last number); generic ids (R2-1, FU-2, D4-3) have < 3."""
    return len(re.sub(r'[^A-Z]', '', re.sub(r'-?\d+[a-z]?$', '', ident)))


def negated(text):
    """True when a closing word in text is negated or qualified by words around it (NO-GO, NOT landed, pending, blocked, unlanded...).
    Windowed check per closing word; NO-GO anywhere negates the whole line."""
    if re.search(r'\bNO-GO\b', text):
        return True
    ms = list(HEAD_DONE.finditer(text))
    if not ms:
        return bool(NEG_RE.search(text))
    return all(NEG_RE.search(text[max(0, m.start() - NEG_BEFORE):m.start()]) or NEG_RE.search(text[m.end():m.end() + NEG_AFTER]) for m in ms)


def heading_landed(doc_text):
    """id -> (line, heading) for ids a heading states landed/closed: single id or range PX-F1..F7."""
    res = {}
    for n, line in enumerate(doc_text.split('\n'), 1):
        if not line.startswith('#') or not HEAD_DONE.search(line) or negated(line):
            continue
        h = line.lstrip('#').strip()
        done_at = HEAD_DONE.search(h).start()
        pre_h = h[:done_at]
        for m in RANGE_RE.finditer(pre_h):
            for k in range(int(m.group('a')), int(m.group('b')) + 1):
                res[m.group('pre') + str(k)] = (n, h[:70])
        for m in re.finditer(r'(?<![\w/.-])(' + ID_ONE + r')(?![\w.-])', pre_h):
            if letters(m.group(1)) >= 3:
                res.setdefault(m.group(1), (n, h[:70]))
        for i in expand_slash(pre_h):          # CAT-PREEXIST-7/8/9, PW-F5/F6  [WP10R4-4]
            if letters(i) >= 3:
                res.setdefault(i, (n, h[:70]))
    return res


def parse(doc_text):
    """Return list of item dicts for bullet lines led by an id. text starts AFTER the id.
    `parent` is the text of the enclosing bullet (smaller indent) for a nested id bullet, else ''."""
    items, heading, cur, mode, bul = [], '', None, None, {}
    for n, line in enumerate(doc_text.split('\n'), 1):
        if line.startswith('#'):
            heading, cur, mode, bul = line.lstrip('#').strip(), None, None, {}
            continue
        m = LEAD.match(line)
        bm = re.match(r'^(\s*)- (.*)$', line)
        if bm:
            k = len(bm.group(1))
            bul = {i: v for i, v in bul.items() if i < k}
        if m:
            ids = re.split(r' / |, | and ', m.group(2))
            lower = [i for i in bul if i < len(m.group(1))]
            parent = bul[max(lower)] if lower else ''
            cur = {'line': n, 'ids': ids, 'text': line[m.end():].lstrip('*`').strip(), 'heading': heading, 'mode': mode, 'parent': parent}
            items.append(cur)
        elif cur and line.startswith('  ') and line.strip() and not line.lstrip().startswith('- '):
            cur['text'] += ' ' + line.strip()
        elif not line.strip():
            cur = None
        elif line.lstrip().startswith('- '):
            cur = None
            bul[len(bm.group(1))] = bm.group(2)
        else:
            cur = None
            st = line.strip()
            if LABEL_LANDED.match(st) and not negated(st):
                mode = 'landed'
            elif LABEL_OPEN.match(st):
                mode = 'open'
    return items


def git_evidence(repo, ref):
    """id-token -> list of (sha, subject) for CODE commits on ref: not a register commit, subject not `docs:`/`docs(`,
    and the diff touches at least one file that is neither *.md nor under docs/. Ranges in the subject (D7-R2-F1..F4) are expanded."""
    out = subprocess.run(['git', '-C', repo, 'log', '--name-only', '--format=%x01%h%x09%s', ref],
                         capture_output=True, text=True, check=True).stdout
    toks = collections.defaultdict(list)
    for blk in out.split('\x01')[1:]:
        head, _, files = blk.partition('\n')
        sha, _, subj = head.partition('\t')
        if REGISTER_COMMIT.search(subj) or re.match(r'docs[(:]', subj):
            continue
        if not any(f.strip() and not f.endswith('.md') and not f.startswith('docs/') for f in files.split('\n')):
            continue
        for t in set(re.findall(ID_ONE, subj)) | set(expand_ranges(subj)):
            toks[t].append((sha, subj))
    return toks


def own_head(t):
    """Text that belongs to this row's own id: cut at the next id token after the leading (...) group."""
    t = t[:240]
    pm = re.match(r'\s*\([^)]*\)', t)
    start = pm.end() if pm else 0
    nx = re.search(r'(?<![\w/.=-])(' + ID_ONE + r')(?![\w.-])', t[start:])
    if nx and letters(nx.group(1)) >= 2 and '-' in nx.group(1):
        t = t[:start + nx.start()]
    return t


def classify(item, ident, dup_count, gitev, later, hland):
    t, h = item['text'], item['heading']
    head = own_head(t)
    sev, sev_src, sev_note = 'UNKNOWN', '', ''
    # entry: the stated token nearest after the id wins (not vocabulary order)
    hits = [(rx.search(head).start(), k) for k, rx in SEV_RE.items() if rx.search(head)]
    if hits:
        sev, sev_src = min(hits)[1], 'entry'
    if sev == 'UNKNOWN':
        for k, rx in SEV_RE.items():
            if rx.search(h):
                sev, sev_src = k, 'section'
                break
    if sev == 'UNKNOWN' and re.search(r'\bHIGH\b', head):
        sev_note = ' (stated HIGH, not in vocabulary)'
    layer, derived = 'UNKNOWN', False
    for src in (head, h):          # the entry's own stated layer beats the section heading's  [WP10R4-8]
        for k, rx in LAYER_STATED.items():
            if rx.search(src):
                layer = k
                break
        if layer != 'UNKNOWN':
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
    h80 = t[:80]
    open_hit = bool(OPEN_HEAD.search(h80))
    openw = bool(OPEN_ANY.search(t))
    closed_start = starts_closed(t)
    parent_closed = bool(item.get('parent')) and starts_closed(item['parent'])
    status, ev = 'open', 'no closing evidence in the entry, no code commit on ref'
    d = DUP_RE.search(t)
    if d and d.group(1) != ident:
        status, ev = 'duplicate-of ' + d.group(1), 'entry text'
    elif (closed_start or parent_closed) and not openw:
        status = 'landed'
        ev = 'entry opens with a closing token and states nothing open (PROVISIONAL)' if closed_start else 'nested under a closed parent bullet (PROVISIONAL)'
    elif (closed_start or parent_closed) and openw:
        status, ev = 'unconfirmed', 'partial: opens with a closing token but also states an open/owed/partial remainder (landed in part; NOT closed)'
    elif open_hit:
        status, ev = 'open', 'entry states OPEN/OWED/Queued near its start'
    elif item.get('mode') == 'landed':
        status, ev = ('unconfirmed', 'under a "Fixed/Closed:" label line but the entry also states an open remainder') if openw else ('landed', 'under a "Fixed/Closed:" label line in its section (PROVISIONAL)')
    elif item.get('mode') == 'open':
        status, ev = 'open', 'under an "Open/Still open:" label line'
    elif hland.get(ident) and (dup_count == 1 or (letters(ident) >= 3 and hland[ident][0] > item['line'])):
        ln, hd = hland[ident]
        status, ev = ('unconfirmed', 'heading line %d names it landed but the entry states an open remainder' % ln) if openw else ('landed', 'heading line %d: "%s" (PROVISIONAL)' % (ln, hd))
    elif dup_count == 1 and letters(ident) >= 3 and gitev.get(ident) and not openw:
        c = gitev[ident]
        sha, subj = c[0]
        status, ev = 'landed', 'code commit %s %s%s (PROVISIONAL)' % (sha, subj[:80], ' (+%d more)' % (len(c) - 1) if len(c) > 1 else '')
    elif dup_count == 1 and letters(ident) >= 3 and gitev.get(ident) and openw:
        status, ev = 'unconfirmed', 'a code commit names it (%s) but the entry states an open/owed/partial remainder' % gitev[ident][0][0]
    elif DONE_HEAD.search(h80) or FULL_DONE.search(t):
        status, ev = 'unconfirmed', 'a closing word appears in the entry but not at its start (weak)'
    elif dup_count == 1 and later.get(ident):
        ln, sn = later[ident]
        status, ev = 'unconfirmed', 'only a later prose line %d mentions it closed: "%s" (weak)' % (ln, sn)
    sec = bool(SEC_RE.search(t[:300]))
    unclass = layer == 'UNKNOWN'
    nonlayer = layer in ('test-instrumentation', 'process-doc')
    if status not in ('open', 'unconfirmed'):
        blk = ('no', 'not open')
    elif sev == 'critical':
        blk = ('yes', 'severity critical stated')
    elif BLOCK_RE.search(head):
        blk = ('yes', 'entry states release-blocking')
    elif sev == 'important' and (layer == 'source-defect' or unclass):
        blk = ('yes', 'important + %s' % ('source-defect' if layer == 'source-defect' else 'layer unclassifiable = source-defect per 11.4.235(D)'))
    elif sev == 'important':
        blk = ('no', 'important but %s layer%s' % (layer, ' (security-adjacent word present, not a source defect)' if sec else ''))
    elif sev == 'UNKNOWN' and not nonlayer:
        blk = ('unclassified', 'severity UNKNOWN + %s layer: cannot be called blocker or not (11.4.276(G.2)/11.4.235(D)); set severity at registration%s' % (layer, '; security-adjacent word present, triage first' if sec else ''))
    elif sev == 'UNKNOWN':
        blk = ('no', 'severity UNKNOWN but %s layer' % layer)
    else:
        blk = ('no', 'severity %s' % sev)
    if status == 'unconfirmed' and blk[0] != 'no':
        blk = (blk[0], 'status unconfirmed (not closed); ' + blk[1])
    fam = re.sub(r'-?[A-Z]?\d+[a-z]?$', '', ident) or ident
    cluster = '%s / %s' % (fam, re.sub(r'\s*\(.*$', '', h)[:48])
    return dict(id=ident, line=item['line'], layer=layer + ('*' if derived else ''),
                sev=sev + ('(sec)' if sev_src == 'section' else '') + sev_note, sevraw=sev,
                blocker=blk[0], reason=blk[1], cluster=cluster, status=status, evidence=ev, security=sec)


def series_words(heading):
    return {w for w in re.findall(r'[a-z]{7,}', heading.lower())} - {'follow', 'review', 'landing', 'landed', 'round'}


def later_mentions(doc_text, items):
    """id -> (line, snippet) for a line (lead lines included, except the lead's own ids) where the id is followed
    within 50 chars by a closing word."""
    lead_ids = {it['line']: set(it['ids']) for it in items}
    res = {}
    for n, line in enumerate(doc_text.split('\n'), 1):
        mc = re.search(r'Closed here: ((?:' + ID_ONE + r'(?: \(|, |\)|$)\s*)+)', line)
        if mc and n not in lead_ids:
            for cid in re.findall(ID_ONE, mc.group(1)):
                res[cid] = (n, line[mc.start():mc.start() + 90].replace('|', '/'))
        for m in re.finditer(r'(?<![\w/.-])(' + ID_ONE + r')(?![\w-])', line):
            if m.group(1) in lead_ids.get(n, ()):
                continue
            w = line[m.end():m.end() + 50]
            if LATER_DONE.search(w) and not (NEG_RE.search(w) or re.search(r'\b(?:open|queued|owed)\b', w, re.I)):
                res[m.group(1)] = (n, line[max(0, m.start() - 10):m.end() + 50].strip()[:90].replace('|', '/'))
    return res


def build(doc_text, repo, ref):
    items = parse(doc_text)
    later = later_mentions(doc_text, items)
    hland = heading_landed(doc_text)
    cnt = collections.Counter(i for it in items for i in it['ids'])
    gitev = git_evidence(repo, ref)
    rows = []
    for it in items:
        for ident in it['ids']:
            rows.append(classify(it, ident, cnt[ident], gitev, later, hland))
    # the same project-specific id (>= 3 family letters) listed again later is one item: a later landed entry closes the earlier open ones
    byid = collections.defaultdict(list)
    for r in rows:
        byid[r['id']].append(r)
    heads = {}
    for it in items:
        heads[it['line']] = it['heading']
    for ident, rs in byid.items():
        rs.sort(key=lambda r: r['line'])
        # same item when the id is project-specific (>= 3 family letters, or 3+ hyphen parts), or the later entry's section heading names the id
        specific = bool(letters(ident) >= 3 or ident.count('-') >= 2 or (letters(ident) >= 2 and re.search(r'(?<![\w-])' + re.escape(ident) + r'(?![\w-])', heads.get(rs[-1]['line'], ''))))
        if len(rs) > 1 and not specific:
            # reused generic id (F1, F3, ...): pair an open row with the nearest later landed row of the same series
            # (<= 40 lines apart, section headings share a word of >= 7 letters)
            for i, r in enumerate(rs):
                if r['status'] != 'open':
                    continue
                for q in rs[i + 1:]:
                    if q['line'] - r['line'] > 40:
                        break
                    if q['status'] == 'landed' and series_words(heads.get(r['line'], '')) & series_words(heads.get(q['line'], '')):
                        r.update(status='landed', blocker='no', reason='not open',
                                 evidence='series pairing: landed entry of the same id at line %d (section headings share a word) (PROVISIONAL)' % q['line'])
                        break
        if len(rs) > 1 and specific and rs[-1]['status'] == 'landed':
            for r in rs[:-1]:
                if r['status'] == 'open':
                    r.update(status='landed', blocker='no', reason='not open',
                             evidence='later entry of the same id at line %d states it landed (PROVISIONAL)' % rs[-1]['line'])
        elif len(rs) > 1 and specific and rs[-1]['status'] == 'unconfirmed':
            for r in rs[:-1]:
                if r['status'] == 'open':
                    r.update(status='unconfirmed', evidence='later entry of the same id at line %d only weakly states it closed (NOT closed)' % rs[-1]['line'])
    return rows


def prose_only(doc_text, rows):
    have = {r['id'] for r in rows}
    seen, out = set(), []
    for n, line in enumerate(doc_text.split('\n'), 1):
        if not re.search(r'important|IMPORTANT|critical|CRITICAL|BLOCKER', line):
            continue
        for m in re.finditer(r'(?<![\w/.-])(' + ID_ONE + r')(?![\w.-])', line):
            i = m.group(1)
            if i not in have and i not in seen and '-' in i and letters(i) >= 3:
                seen.add(i)
                out.append((i, n))
    return out


def needles(rows, doc_text):
    ids = {r['id'] for r in rows}
    errs = []
    if 'CAT-PREEXIST-1' not in ids:
        errs.append('positive needle CAT-PREEXIST-1 not found')
    if 'ZZ-NOSUCH-999' in ids or 'ZZ-NOSUCH-999' in doc_text:
        errs.append('negative needle ZZ-NOSUCH-999 found')
    if not any(r['id'].startswith('PX-F') for r in rows):
        errs.append('positive needle family PX-F* not found')
    # status needles (11.4.201(7)(b)): a known-landed and a known-open id must classify correctly
    st = {r['id']: r['status'] for r in rows}
    if 'PXF23-R2' in st and st['PXF23-R2'] != 'landed':
        errs.append('status needle PXF23-R2 (own text says FIXED) not landed')
    if 'FU2-1' in st and st['FU2-1'] != 'open':
        errs.append('status needle FU2-1 (STILL OPEN) not open')
    if st.get('SMB-FU-17') == 'landed':
        errs.append('status needle SMB-FU-17 (docs-only commit, copy.go still uses plain SafeRemotePath) must not be landed')
    if 'R3-1' in st and st['R3-1'] != 'unconfirmed':
        errs.append('status needle R3-1 (only a later prose mention) must be unconfirmed')
    if 'PXF23-R2' not in st or 'FU2-1' not in st:
        errs.append('status needle id missing from rows')
    return errs


def render(rows, ref, sha, doc, prose=(), doc_state='state not determined'):
    C = collections.Counter
    op = [r for r in rows if r['status'] == 'open']
    lines = []
    w = lines.append
    w('# Follow-ups triage (machine-derived)\n')
    w('| Field | Value |\n|---|---|')
    w('| Revision | 5 |\n| Created | 2026-11-12 |\n| Last modified | 2026-11-13 (Rev 5: WP10R4 nits - closing token as sentence subject is not a status, more remainder phrases, slash-listed heading ids, entry layer beats heading layer, wider negation windows, Source row names what was read; Rev 4: negated/qualified landed claims (NO-GO headings, case-insensitive inline remainders, bracketed open) are not landed; Rev 3: three-valued status, strong-evidence landed rule, unclassified blocker value) |\n| Status | active (PROVISIONAL: derived by script, not human-verified row by row) |')
    w('| Roadmap-Item | CAT-WP10-TRIAGE |')
    # the register is read from the file system, the git evidence from the ref: say both, never "doc at <sha> of <ref>"  [WP10R4-7]
    w('| Source | `%s` as read from the file system (%s); git evidence (code commits) from `%s` at `%s` |' % (doc, doc_state, ref, sha))
    w('| Generator | `scripts/docs/gen_follow_ups_triage.py` (test: `scripts/docs/test_gen_follow_ups_triage.py`) |')
    w('| Companion docs | `docs/FOLLOW_UPS_NEXT_PHASE.md`, `docs/WORKABLE_ITEMS_REGISTER.md` |\n')
    w('Do not edit the table by hand; regenerate: `python3 scripts/docs/gen_follow_ups_triage.py --out docs/FOLLOW_UPS_TRIAGE_2026-11-12.md`.\n')
    w('## Table of contents\n\n- [Method and limits](#method-and-limits)\n- [Counts](#counts)\n- [Top blockers](#top-blockers)\n- [Proposed follow-up policy](#proposed-follow-up-policy-not-applied)\n- [Full table](#full-table)\n')
    w('## Method and limits\n')
    w('* Rows are bullet entries of the register that begin with an id (one row per id and line). Ids mentioned only inside prose are not rows. Ids that appear as the lead of several bullets (`F1`, `R1`, ...) are listed once per bullet and get no git evidence.')
    w('* Reused generic ids (`F1`, `F3`, `R1`, ...) have several rows; a closing in a later section cannot be tied to the right row by id, so such rows may still read `open`. Review them by line. Status sources, in order: duplicate-of; `landed` (STRONG only): the entry itself opens with a closing token (FIXED / LANDED / [LANDED] / PROVISIONAL-DONE / DONE / CLOSED / RESOLVED, also inside its first parenthesis) and states nothing open anywhere in it, or it is nested under a bullet that opens that way, or a `Fixed:`/`Closed:` label line above it, or a heading that names the id or an id range landed (`PX-F1..F7 landed`; not when the heading is negated: NO-GO, NOT, pending, blocked, unlanded), or a CODE commit naming the id or a range of ids (id with at least 3 family letters, unique among the lead ids; the commit subject is not `docs:`/`docs(`, not a register commit, and its diff touches a file that is neither `*.md` nor under `docs/`); `open`: the entry says OPEN/OWED/Queued near its start; `unconfirmed`: everything weaker (a landed claim whose own text states a remainder, case-insensitive: still open/unguarded/missing, not re-checked/covered/yet, `Follow-ups:`/`Remaining:`/`Caveat:` lists, `for the X side`, partially, an open operator decision; a closing word that is not at the start of the entry, a later prose line, a partial entry that opens with a closing token but also states an open remainder, a code commit against an entry that states an open remainder). **`unconfirmed` is NOT closed**: it needs a human check and is counted next to open, never as done.')
    w('* Severity is only what the entry (first 200 characters) or its section heading states (`(sec)` = taken from the section heading). Not stated: `UNKNOWN`. Nothing is inferred.')
    w('* Layer: stated (`source-defect` / `test-instrumentation` / `process-doc`) or, with a trailing `*`, derived from the Type word or keywords (a heuristic).')
    w('* Status: `landed` only on the strong evidence above (PROVISIONAL: a code commit on `%s` shows work reached main, not that the register was updated or the fix verified); `duplicate-of` only when the entry says so. Known misses: reused generic ids (`FU-4`, `F1`, ...) whose closing sits in another section stay `open` or `unconfirmed`; nothing is guessed.' % ref)
    w('* Blocker (rows that are open or unconfirmed): `yes` on a stated critical, a stated release-blocking word, or important + (source-defect or unclassifiable layer, per 11.4.235(D) unclassifiable = source-defect). A security-adjacent word does not make an important test-instrumentation or process-doc row a blocker. `unclassified` = severity UNKNOWN with a source-defect or UNKNOWN layer: it can be called neither a blocker nor a non-blocker (11.4.276(G.2) / 11.4.235(D): an unrecorded severity is not minor-class, and it is not stated critical either); it is a triage defect, see proposed policy (4). Everything else `no` with the reason. This is a rule, not a verdict; the operator decides.')
    w('* Cluster: id family plus section title. It groups for batching; it is NOT a proven common root cause.')
    w('* Control needles (11.4.273) passed before this file was written: CAT-PREEXIST-1 present, PX-F* family present, fabricated ZZ-NOSUCH-999 absent; status needles PXF23-R2 landed, FU2-1 open, SMB-FU-17 not landed, R3-1 unconfirmed.\n')
    w('## Counts\n')
    w('Rows: %d. By status: %s. **`unconfirmed` rows are NOT closed**: they have only weak evidence and need a human check; they are not part of the landed count.\n' % (len(rows), ', '.join('%s %d' % (k.split(' ')[0], v) for k, v in sorted(C(r['status'].split(' ')[0] for r in rows).items()))))
    unc = [r for r in rows if r['status'] == 'unconfirmed']
    w('Unconfirmed rows by severity: %s; by layer: %s.\n' % (', '.join('%s %d' % (k, v) for k, v in sorted(C(r['sevraw'] for r in unc).items())), ', '.join('%s %d' % (k, v) for k, v in sorted(C(r['layer'].rstrip('*') for r in unc).items()))))
    w('Open rows by layer (derived `*` merged): %s.\n' % ', '.join('%s %d' % (k, v) for k, v in sorted(C(r['layer'].rstrip('*') for r in op).items())))
    w('Open rows by severity: %s.\n' % ', '.join('%s %d' % (k, v) for k, v in sorted(C(r['sevraw'] for r in op).items())))
    w('Open layer x severity:\n')
    w('| layer | critical | important | minor | nit | UNKNOWN |\n|---|---|---|---|---|---|')
    for lay in ('source-defect', 'test-instrumentation', 'process-doc', 'UNKNOWN'):
        sub = [r for r in op if r['layer'].rstrip('*') == lay]
        w('| %s | %s |' % (lay, ' | '.join(str(sum(1 for r in sub if r['sevraw'] == s)) for s in ('critical', 'important', 'minor', 'nit', 'UNKNOWN'))))
    nc = [r for r in rows if r['status'] in ('open', 'unconfirmed')]
    blk = [r for r in nc if r['blocker'] == 'yes']
    rev = [r for r in nc if r['blocker'] == 'unclassified']
    w('\nBlockers by rule (open or unconfirmed): %d (open %d, unconfirmed %d). `unclassified` (severity UNKNOWN + source-defect/UNKNOWN layer; neither blocker nor non-blocker): %d, of which %d carry a security-adjacent word and need triage first. See proposed policy (4): every id carries a stated severity.\n' % (len(blk), sum(1 for r in blk if r['status'] == 'open'), sum(1 for r in blk if r['status'] == 'unconfirmed'), len(rev), sum(1 for r in rev if r['security'])))
    w('Open rows per cluster family (top 15):\n')
    fam = C(r['cluster'].split(' / ')[0] for r in op)
    w('| family | open |\n|---|---|')
    for k, v in fam.most_common(15):
        w('| %s | %d |' % (k, v))
    w('\n## Top blockers\n')
    if blk or rev:
        w('Rule `yes` rows first, then the security-adjacent `unclassified` rows (first 25).\n')
        w('| blocker | id | line | status | layer | severity | reason | cluster |\n|---|---|---|---|---|---|---|---|')
        for r in (blk + [x for x in rev if x['security']][:25]):
            w('| %s | %s | %d | %s | %s | %s | %s | %s |' % (r['blocker'], r['id'], r['line'], r['status'], r['layer'], r['sev'], r['reason'].replace('|', '/'), r['cluster'].replace('|', '/')))
    else:
        w('None by the rule.')
    w('\nScope of the open set: only bullet entries that BEGIN with an id are rows. %d further ids are mentioned only inside prose or only in the register and are NOT rows, so a blocker that exists only there is invisible to the table. Prose/register-only ids on a line that says important/critical/blocker (first 20): %s. Known prose-only candidate: `OBF-1` ("Follow-ups (Queued, minor): OBF-1 ..." list near line 1921; minor, a rollback that can delete an unreadable config). `CAT-D9-SUSPEND` is a row (line 1279) and also in the register Part A. The table it does not replace the register.' % (len(prose), ', '.join('%s (line %d)' % x for x in prose[:20]) or 'none'))
    w('\n## Proposed follow-up policy (not applied)\n')
    w('PROPOSAL for operator decision (11.4.66); no rule file was changed. Text for `docs/FOLLOW_UPS_NEXT_PHASE.md` header or the review-budget policy:\n')
    w('> **Follow-up triage policy (proposed 2026-11-12).** (1) A reviewer finding of severity minor joins an existing open class item of the same root-cause family (same id family and section) as an added line, unless it is security-adjacent (authz, credential, injection, path confinement, data loss, information leak); a security-adjacent minor keeps its own id and is triaged for the next release. (2) A nit is not registered as its own id: it is applied in the same change when trivial, or listed once under the class item; a nit still open after the triage pass for the next release is closed with the reason `nit-after-triage`. (3) Each landed batch registers at most one new item per distinct class; the count of open ids must not grow between consecutive releases except for new increments (11.4.230(E.4) direction rule). (4) Every id carries a stated severity; UNKNOWN is a triage defect to fix at registration.\n')
    w('## Full table\n')
    w('| id | line | layer | severity | blocker | reason | cluster | status | evidence |\n|---|---|---|---|---|---|---|---|---|')
    order = {'yes': 0, 'unclassified': 1, 'no': 2}
    sord = {'open': 0, 'unconfirmed': 1}
    for r in sorted(rows, key=lambda r: (sord.get(r['status'], 2), order[r['blocker']], r['line'])):
        w('| %s | %s | %s | %s | %s | %s | %s | %s | %s |' % tuple(
            str(r[k]).replace('|', '/') for k in ('id', 'line', 'layer', 'sev', 'blocker', 'reason', 'cluster', 'status', 'evidence')))
    return '\n'.join(lines) + '\n'


def doc_state(repo, ref, doc, text):
    """How the register text that was read relates to the same file in ref (identical / differs / not in ref)."""
    p = subprocess.run(['git', '-C', repo, 'show', '%s:%s' % (ref, doc)], capture_output=True)
    if p.returncode != 0:
        return 'not present in `%s`' % ref
    if p.stdout.decode('utf-8', 'replace') == text:
        return 'identical to the copy in `%s`' % ref
    return 'DIFFERS from the copy in `%s` (working tree or branch tip is ahead)' % ref


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
    md = render(rows, a.ref, sha, a.doc, prose_only(text, rows), doc_state(a.repo, a.ref, a.doc, text))
    if a.out:
        open(a.repo + '/' + a.out, 'w', encoding='utf-8').write(md)
    else:
        sys.stdout.write(md)
    return 0


if __name__ == '__main__':
    sys.exit(main())
