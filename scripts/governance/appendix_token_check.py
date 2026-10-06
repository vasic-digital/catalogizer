#!/usr/bin/env python3
"""appendix_token_check.py - per-anchor token comparison of the Spec Kit appendix digests against the canon (WF8 findings F1, F3).

Contract: every `CM-*` gate token and every `--flag` (no-escape-hatch flags) that appears in the canon block of an anchor MUST appear in the
matching `#### §<id>` digest of the appendix. Restating is the digest's whole job (appendix Part 1 says it states every gate and flag);
this tool is the deterministic instrument for that claim, which no other tool checked (regen check guards only generated regions).

Usage: appendix_token_check.py --canon <Constitution.md> --appendix <constitution-appendix.md> [--allow <tsv>] [--json <out.json>]

Canon block-starts (forms that occur in Constitution.md): `### §<id> ...`, `#### §<id> ...`, and a line beginning `**§<id> — ` (bold
block form; a bare `§<id> ` at the start of a prose line, or `**§<id> precedence`, is a citation, never a block-start). A block ends at the next block-start or at any `## ` / `# ` heading. Appendix digests start at `#### §<id>` and end at the next
`#### `, `### `, `## `, `# ` heading, except that a lettered sub-clause digest heading `#### §<id>(X)` continues its parent digest (the canon
keeps such a clause inside the parent block). Ids are full dotted ids (11.4.10.A never prefix-matches 11.4.10).

Allow file (TSV: `<id>\t<token or *>\t<reason>`): an exemption for a token that is a carrier (placeholder / repealed-gate mention) or, with
`*`, an anchor that has no digest by design. A reason is mandatory (exit 2 otherwise). A STALE exemption (token already present in the
digest, or not in the canon block, or the anchor is not missing its digest) is a failure: an exemption list must not rot.

Exit: 0 no gaps; 1 gaps / no-digest / stale exemption; 2 usage, unreadable input, BLIND (zero blocks or zero digests extracted), duplicate
canon block-start, malformed allow file.
"""
import argparse, hashlib, json, re, sys

ID = r'(11\.4\.\d+(?:\.[A-Za-z0-9]+)*|12\.\d+(?:\.[A-Za-z0-9]+)*)'
CANON_START = re.compile(r'^ {0,3}(?:#{3,4} +§' + ID + r'(?=\s|—)|\*\*§' + ID + r'\s+—)')
APP_START = re.compile(r'^#### +§' + ID + r'(?=\s|—)')
HEAD = re.compile(r'^#{1,2} ')
APP_HEAD = re.compile(r'^#{1,4} ')
APP_SUB = re.compile(r'^#### +§' + ID + r'\(')  # lettered sub-clause digest: continues its parent digest
TOK = re.compile(r'CM-[A-Z0-9][A-Z0-9-]*[A-Z0-9]|(?<![\w-])--[a-z][a-z0-9-]*[a-z0-9]')


WRAP = re.compile(r'((?:CM-[A-Z0-9-]*|(?<![\w-])--[a-z][a-z0-9-]*)-)[ \t]*\n[ \t]*(?=[A-Za-z0-9])')


def joined(text):
    """A token wrapped at a hyphen across a line break is one token (the canon wraps long CM-* names and flags)."""
    return WRAP.sub(r'\1', text)


def usage(msg):
    sys.stderr.write("appendix_token_check: " + msg + "\n")
    sys.exit(2)


def segment(text, start_re, end_re, cont_re=None):
    blocks, order, cur, buf, dups = {}, [], None, [], []
    for line in text.splitlines():
        m = start_re.match(line)
        if m:
            if cur is not None:
                blocks[cur] = "\n".join(buf)
            cur = m.group(1) or m.group(2)
            if cur in blocks or cur in order:
                dups.append(cur)
            order.append(cur)
            buf = [line]
            continue
        if cur is not None and end_re.match(line) and not (cont_re and cont_re.match(line)):
            blocks[cur] = "\n".join(buf)
            cur, buf = None, []
            continue
        if cur is not None:
            buf.append(line)
    if cur is not None:
        blocks[cur] = "\n".join(buf)
    return blocks, dups


def read(p):
    try:
        with open(p, "rb") as f:
            return f.read()
    except OSError as e:
        usage("cannot read %s: %s" % (p, e))


def load_allow(p):
    out = {}
    for n, line in enumerate(read(p).decode("utf-8").splitlines(), 1):
        if not line.strip() or line.startswith("#"):
            continue
        parts = line.split("\t")
        if len(parts) < 3 or not parts[2].strip():
            usage("allow file line %d: id, token and a non-empty reason are required" % n)
        out[(parts[0].strip(), parts[1].strip())] = parts[2].strip()
    return out


def main():
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--canon", required=True)
    ap.add_argument("--appendix", required=True)
    ap.add_argument("--allow")
    ap.add_argument("--json")
    try:
        a = ap.parse_args()
    except SystemExit as e:
        sys.exit(2 if e.code not in (0,) else 0)
    craw, araw = read(a.canon), read(a.appendix)
    canon, app = craw.decode("utf-8"), araw.decode("utf-8")
    allow = load_allow(a.allow) if a.allow else {}
    cb, cdups = segment(canon, CANON_START, HEAD)
    ab, adups = segment(app, APP_START, APP_HEAD, APP_SUB)
    if cdups:
        usage("duplicate canon block-start for %s" % ", ".join(sorted(set(cdups))))
    if adups:
        usage("duplicate appendix digest for %s" % ", ".join(sorted(set(adups))))
    if not cb:
        usage("BLIND: zero canon blocks extracted (a zero is not evidence)")
    if not ab:
        usage("BLIND: zero appendix digests extracted (a zero is not evidence)")
    gaps, nodigest, stale, used, checked, exempt = [], [], [], set(), 0, 0
    for k in sorted(cb):
        ct = set(TOK.findall(joined(cb[k])))
        if k not in ab:
            if (k, "*") in allow:
                used.add((k, "*"))
                exempt += 1
            else:
                nodigest.append(k)
            continue
        at = set(TOK.findall(joined(ab[k])))
        checked += len(ct)
        for t in sorted(ct - at):
            if (k, t) in allow:
                used.add((k, t))
                exempt += 1
            else:
                gaps.append((k, t))
    for (k, t), why in sorted(allow.items()):
        if (k, t) in used:
            continue
        stale.append((k, t, why))
    ainfo = sorted(set(ab) - set(cb))
    print("canon_blocks=%d appendix_digests=%d tokens_checked=%d gaps=%d no_digest=%d exempt=%d stale_exemptions=%d" % (
        len(cb), len(ab), checked, len(gaps), len(nodigest), exempt, len(stale)))
    for k, t in gaps:
        print("GAP §%s: digest lacks %s (present in canon block)" % (k, t))
    for k in nodigest:
        print("NO-DIGEST §%s: no digest in the appendix for this canon block" % k)
    for k, t, why in stale:
        print("STALE exemption §%s %s: not needed (%s)" % (k, t, why))
    print("INFO appendix digests without a canon block-start: %s" % (", ".join(ainfo) if ainfo else "none"))
    if a.json:
        with open(a.json, "w", encoding="utf-8") as f:
            json.dump({"canon_sha256": hashlib.sha256(craw).hexdigest(), "appendix_sha256": hashlib.sha256(araw).hexdigest(),
                       "canon_blocks": len(cb), "appendix_digests": len(ab), "tokens_checked": checked,
                       "gaps": [{"id": k, "token": t} for k, t in gaps], "no_digest": nodigest, "exempt": exempt,
                       "stale_exemptions": [{"id": k, "token": t} for k, t, _ in stale], "appendix_only": ainfo}, f, indent=1)
            f.write("\n")
    sys.exit(1 if (gaps or nodigest or stale) else 0)


if __name__ == "__main__":
    main()
