#!/usr/bin/env python3
"""Static scanner (never executes anything): finds git hook bodies WRITTEN BY test scripts (.sh/.bash/.py) that run a
history/worktree/ref-changing git command (commit/merge/rebase/checkout/switch/pull/reset/cherry-pick/am/revert/push/...)
without a recursion guard that is a REAL guard for that hook.  Such a hook re-fires itself through the git command it runs
(e.g. post-commit + `git commit --amend`, pre-push + `git push`) => unbounded fork recursion (host hang 2026-10-08).

A hook body is considered guarded only if it (a) TESTS a marker (a file test `[ -e M ]`/`test -f M`, or a string test of
`$VAR`) AND (b) leaves the hook (`exit`/`return`) when it holds AND (c) SETS that same marker (creates the file M / assigns
VAR) BEFORE the risky git command - a test of something nothing ever sets is not a guard.

Recognised hook installation forms: redirects/printf/echo/heredocs to any path ending in /<hook> (custom core.hooksPath
dirs included), hook_case-style `<hook> '<body>'` helpers, python string/open() writers, a body held in a variable assigned
earlier in the file, and cp/ln/install/mv/cat> of a literal source file (the SOURCE is scanned).  An installation whose
source the scanner cannot read (a non-literal path, `sed -i` on an existing hook) is itself reported (unreadable body) unless
the line (or the one above) carries `# hook-scan: reviewed <reason>` - an audited escape, reason mandatory.

Usage: scan_recursive_hook_bodies.py <root> [--exclude <path>]...   Exit 0 clean, 1 findings (file:line: [kind] text), 2 usage."""
import os
import re
import shlex
import sys

HOOKS = ['post-commit', 'post-merge', 'post-checkout', 'post-rewrite', 'prepare-commit-msg', 'commit-msg', 'pre-commit',
         'pre-merge-commit', 'reference-transaction', 'pre-push', 'pre-rebase', 'post-applypatch', 'pre-applypatch',
         'applypatch-msg', 'push-to-checkout', 'pre-auto-gc', 'post-update', 'pre-receive', 'post-receive', 'fsmonitor-watchman']
HOOKALT = '|'.join(re.escape(h) for h in HOOKS)
VERBS = r'commit|merge|rebase|checkout|switch|pull|reset|cherry-pick|am|revert|push|update-ref|stash|worktree|fetch|clone'
# a path ending in /<hook> (hooks/<hook>, "$HP/<hook>", .githooks/<hook> ...), a hook_case-style `<hook> '<body>'`, a "<hook>" string
WRITE = re.compile(r'/(?:%s)\b|(?<![\w/.-])(?:%s)(?![\w-])[\'"]?\s+[\'"$]|[\'"](?:%s)[\'"]' % (HOOKALT, HOOKALT, HOOKALT))
HOOKPATH = re.compile(r'\S*/(?:%s)\b\S*|\S*hooks?/\S*' % HOOKALT)    # hook file paths are not part of the hook BODY
GITCMD = r'(?:(?<![\w$./-])git|(?<![\w])(?:/[\w./-]*/)git|"?\$\{?\w*GIT\w*\}?"?)'
RISKY = re.compile(GITCMD + r'\s[^\n;|&]*?\b(?:%s)\b' % VERBS)
HOOKTOK = re.compile(r'["\']?\S*/(?:%s)\b\S*' % HOOKALT)
HEREDOC = re.compile(r'<<-?\s*[\'"]?(\w+)[\'"]?')
ALLOW = re.compile(r'#\s*hook-scan:\s*reviewed\s+\S')
SKIPDIRS = ('.git', 'node_modules', '__pycache__', '.venv', 'vendor')
EXT = ('.sh', '.bash', '.py')


def norm(body):
    """expand the literal escapes printf/echo -e/python turn into newlines, so `\\ngit commit` is seen as a command start"""
    body = body.replace('\\\n', ' ')
    return re.sub(r'\\[ntr]', '\n', body)


def unquote(s):
    return s.replace("'", '').replace('"', '')


def guarded(body):
    """True iff the body holds a guard that is tested, exits when it holds, and is set before the first risky command."""
    b = unquote(norm(body))
    m = RISKY.search(b)
    if not m:
        return True
    risky_at = m.start()
    # file marker: test M ... exit/return ... create M ... risky
    for t in re.finditer(r'(?:\[\[?|\btest)\s+!?\s*-[efsdLx]\s+(\S+?)\s*(?:\]\]?|;|&&|\|\||$)', b):
        marker = t.group(1)
        if marker.startswith('-'):
            continue
        create = re.search(r'(?:>>?|\btouch\b[^;&|\n]*?|\bmkdir\b[^;&|\n]*?)\s*' + re.escape(marker) + r'(?![\w/.-])', b[t.end():])
        if not create:
            continue
        c_at = t.end() + create.start()
        if c_at < risky_at and re.search(r'\b(exit|return)\b', b[t.start():c_at]):
            return True
    # env marker: test $V ... exit/return ... V=... risky
    for t in re.finditer(r'(?:\[\[?|\btest)\s+(?:-[nz]\s+)?!?\s*"?\$\{?(\w+)(?::?-[^}]*)?\}?"?', b):
        var = t.group(1)
        setm = re.search(r'(?:^|[\s;&({])(?:export\s+)?%s=' % re.escape(var), b[t.end():], re.M)
        if not setm:
            continue
        s_at = t.end() + setm.start()
        if s_at < risky_at and re.search(r'\b(exit|return)\b', b[t.start():s_at]):
            return True
    return False


def gather(lines, i, is_py):
    """the text of the hook body written at line i (+ heredoc / triple-quoted / continuation / variable-held parts)"""
    body = ln = lines[i]
    j = i
    h = HEREDOC.search(ln)
    if h:
        j = i + 1
        while j < len(lines) and lines[j].strip() != h.group(1):
            body += '\n' + lines[j]
            j += 1
    tq = re.search(r'("""|\'\'\')', ln)
    if tq and ln.count(tq.group(1)) % 2 == 1:
        j = i + 1
        while j < len(lines) and tq.group(1) not in lines[j]:
            body += '\n' + lines[j]
            j += 1
        if j < len(lines):
            body += '\n' + lines[j]
    while body.rstrip().endswith('\\') and j + 1 < len(lines):
        j += 1
        body += '\n' + lines[j]
    if is_py and body.rstrip().endswith(('(', ',')):      # python call continued on the next lines
        for k in range(1, 6):
            if j + k < len(lines):
                body += '\n' + lines[j + k]
        j += 5
    # a body held in a variable: include its earlier assignment(s)
    names = set(re.findall(r'\$\{?([A-Za-z_]\w*)\}?', ln))
    if is_py:
        names |= set(re.findall(r'\b([A-Za-z_]\w*)\b', ln))
    for name in names:
        if name in ('T', 'D', 'M', 'HS', 'cat', 'printf', 'echo'):
            continue
        pat = re.compile(r'^\s*(?:local\s+|export\s+|declare\s+(?:-\w+\s+)?|readonly\s+)?%s\s*=' % re.escape(name))
        for k in range(max(0, i - 30), i):
            if pat.match(lines[k]):
                chunk = lines[k]
                q = 1
                while chunk.count("'") % 2 == 1 or chunk.count('"') % 2 == 1:
                    if k + q >= i or q > 10:
                        break
                    chunk += '\n' + lines[k + q]
                    q += 1
                body += '\n' + chunk
    return body, j


def read_source(base_dir, root, tok):
    tok = tok.strip('\'"')
    if not tok or '$' in tok or '*' in tok or '`' in tok:
        return None
    for d in (base_dir, root, os.getcwd()):
        p = tok if os.path.isabs(tok) else os.path.join(d, tok)
        if os.path.isfile(p):
            try:
                return open(p, errors='replace').read()
            except OSError:
                return None
    return None


def installation(ln):
    """(source tokens, edits_in_place) if the line installs/edits an existing file AS a hook, else (None, False).
    Quote-aware: a `mv`/`cp` inside a quoted hook BODY is not an installation command."""
    try:
        lex = shlex.shlex(ln, posix=False, punctuation_chars=';&|')
        lex.whitespace_split = True
        toks = list(lex)
    except ValueError:
        return None, False
    segs, cur = [], []
    for t in toks:
        if t and set(t) <= set(';&|'):
            segs.append(cur)
            cur = []
        else:
            cur.append(t)
    segs.append(cur)
    for seg in segs:
        if not seg:
            continue
        if HOOKTOK.fullmatch(seg[-1]):
            for k, t in enumerate(seg):
                if t in ('cp', 'ln', 'install', 'mv', 'rsync'):
                    return [x for x in seg[k + 1:-1] if not x.startswith('-')], False
                if t == 'sed' and any(x.startswith('-i') or x == '--in-place' for x in seg[k + 1:]):
                    return [], True
        for k, t in enumerate(seg):
            if t == 'cat' and any(x in ('>', '>>') for x in seg[k + 1:]):
                gt = next(x for x, v in enumerate(seg) if k < x and v in ('>', '>>'))
                if gt + 1 < len(seg) and HOOKTOK.fullmatch(seg[gt + 1]):
                    return [x for x in seg[k + 1:gt] if not x.startswith('-')], False
    return None, False


def same_file_source(lines, tok):
    """a non-literal source such as "$T/hk_hook.sh": the body this same script writes to a file of that name"""
    base = os.path.basename(tok.strip('\'"'))
    if not base or '$' in base:
        return None
    for k, l in enumerate(lines):
        if re.search(r'>\s*["\']?\S*%s\b' % re.escape(base), l):
            body, _ = gather(lines, k, False)
            return body
    return None


def scan(root, excl):
    bad = []
    for dp, dn, fn in os.walk(root):
        dn[:] = [d for d in dn if d not in SKIPDIRS]
        for f in fn:
            if not f.endswith(EXT):
                continue
            p = os.path.join(dp, f)
            if os.path.abspath(p) in excl:
                continue
            try:
                lines = open(p, errors='replace').read().split('\n')
            except OSError:
                continue
            is_py = f.endswith('.py')
            i = 0
            while i < len(lines):
                ln = lines[i]
                if ln.lstrip().startswith('#') or not WRITE.search(ln):
                    i += 1
                    continue
                allowed = ALLOW.search(ln) or (i > 0 and ALLOW.search(lines[i - 1]))
                body, j = gather(lines, i, is_py)
                # installation of an existing file as a hook (cp/ln/install/mv/rsync SRC HOOK, cat SRC > HOOK, sed -i .. HOOK):
                # scan the SOURCE (literal path, or a file this very script writes); an unreadable source is itself a finding
                src_tokens, edits = installation(ln)
                if src_tokens is not None and not HEREDOC.search(ln):
                    text = None
                    for t in src_tokens:
                        text = read_source(dp, root, t) or same_file_source(lines, t)
                        if text is not None:
                            body += '\n' + text
                            break
                    if (text is None or edits) and not allowed:
                        bad.append('%s:%d: [unreadable-hook-source] %s' % (p, i + 1, ln.strip()[:200]))
                        i = j + 1
                        continue
                stripped = HOOKPATH.sub(' ', body)
                if not allowed and RISKY.search(norm(stripped)) and not guarded(stripped):
                    bad.append('%s:%d: [unguarded-recursive-hook] %s' % (p, i + 1, ln.strip()[:200]))
                i = j + 1
    return bad


def main(a):
    if not a:
        return 2
    root, excl, k = a[0], set(), 1
    while k < len(a):
        if a[k] == '--exclude' and k + 1 < len(a):
            excl.add(os.path.abspath(a[k + 1]))
            k += 2
        else:
            return 2
    bad = scan(root, excl)
    for b in bad:
        print(b)
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
