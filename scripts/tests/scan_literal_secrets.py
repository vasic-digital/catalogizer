#!/usr/bin/env python3
"""Report literal (non-interpolated) secret values in compose/env-style files.

usage: scan_literal_secrets.py [--defaults-only] <allowlist> <file>...
--defaults-only: only inline ${NAME:-literal} defaults (for shell scripts, where KEY=value
assignments are mostly counters/locals).
Prints `file:line:KEY` (never the value) for every secret-named key whose value
is a non-empty literal. Names are matched case-insensitively and cover
PASSWORD/PASSWD/PASS/SECRET/TOKEN/API_KEY/ACCESS_KEY/PRIVATE_KEY/CREDENTIAL.
A value is NOT a literal when it is empty, a bare `${VAR}` / `${VAR:?...}` /
`${VAR:-}` / `${VAR-}` reference, or boolean-ish. Allowlist lines are
`<path>:<KEY>  # reason` (reason mandatory); an allowlisted entry that no longer
matches anything is itself reported (stale allowlist).
Exit 0 = clean, 1 = findings, 2 = usage/blind.
"""
import re, sys

NAME = r'[A-Za-z0-9_]*(?:PASSWORD|PASSWD|PASS|SECRET|TOKEN|API_KEY|ACCESS_KEY|PRIVATE_KEY|CREDENTIAL)[A-Za-z0-9_]*'
# Flow-map / inline forms: `{D_PASSWORD: x}` and shell `export K=v`, `-e K=v`, `--env K=v`.
FLOW = re.compile(r'(?<!\$)[{,]\s*["\']?(' + NAME + r')["\']?\s*:\s*([^,}]*)', re.I)
# Shell scripts: counters/words such as PASS, PASSES, TOKENS are everywhere, so the
# KEY=value forms are matched only for unambiguous secret names.
SHNAME = r'[A-Za-z0-9_]*(?:PASSWORD|PASSWD|SECRET|API_KEY|ACCESS_KEY|PRIVATE_KEY)[A-Za-z0-9_]*'
INLINE = re.compile(r'(?:^|[\s;&|(])(?:export\s+|local\s+|declare\s+(?:-\w+\s+)?|readonly\s+|-e\s+|--env[= ]\s*)["\']?(' + SHNAME + r')=([^\s;&|)]*)')
LINE = re.compile(r'^\s*(?:-\s*)?["\']?(' + NAME + r')["\']?\s*[:=]\s*(.*?)\s*$', re.I)
# Inline literal defaults: ${NAME:-value} / ${NAME-value} for secret-named variables
# (bare PASS is excluded here: shell pass/fail counters are everywhere in test scripts).
DEFAULT = re.compile(r'\$\{([A-Za-z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN|API_KEY|ACCESS_KEY|PRIVATE_KEY|CREDENTIAL)[A-Za-z0-9_]*):?-([^}]+)\}', re.I)
SAFE_KEY_SUFFIX = ('_FILE', '_PATH', '_DIR', '_PORT', '_PORTS', '_URL', '_ENABLED', '_PASSIVE_PORTS')
REF = re.compile(r'^["\']?\$\{[A-Za-z0-9_]+(?::?\?[^}]*|:?-)\}["\']?$')   # ${V:?x} ${V:-} ${V-}

def is_literal(val, shell=False):
    v = val.split(' #')[0].strip().strip('"\'')
    if v == '' or v.lower() in ('true', 'false', 'null', '~', 'none'):
        return False
    if REF.match(val.split(' #')[0].strip()):
        return False
    if re.fullmatch(r'\d{1,4}', v.split()[0]):      # shell counter (PASS=0), not a secret value
        return False
    if v.startswith(('$(', '$$(')):                # command substitution / arithmetic (`$$(` is the compose-escaped form)
        return False
    if v.startswith('$$'):
        return not shell     # shell: PID expansion; compose: `$$x` is the LITERAL `$x`
    if v.startswith('$') and ':-' not in v and '-' not in v.split('}')[0][2:]:
        return False        # plain $VAR / ${VAR} expansion (no literal default)
    return True

def load_allow(path):
    allow = {}
    for raw in open(path):
        s = raw.strip()
        if not s or s.startswith('#'):
            continue
        ent, _, why = s.partition('#')
        if not why.strip():
            print(f'BAD ALLOWLIST (reason required): {ent.strip()}', file=sys.stderr); sys.exit(2)
        allow[ent.strip()] = False
    return allow

def main():
    args = sys.argv[1:]
    defaults_only = bool(args) and args[0] == '--defaults-only'
    if defaults_only:
        args = args[1:]
    if len(args) < 2:
        print(__doc__, file=sys.stderr); return 2
    allow = load_allow(args[0]); found = 0
    for f in args[1:]:
        for n, line in enumerate(open(f, errors='replace'), 1):
            hits = []
            shell = f.endswith('.sh')
            m = None if defaults_only else LINE.match(line)
            cands = []
            if m and not (shell and not (re.fullmatch(SHNAME, m.group(1), re.I) and m.group(1).isupper())):
                cands.append((m.group(1), m.group(2)))
            if not defaults_only:
                if not shell:
                    cands += [(x.group(1), x.group(2)) for x in FLOW.finditer(line)]
                if shell:
                    cands += [(x.group(1), x.group(2)) for x in INLINE.finditer(line)]
            for key, val in cands:
                if not key.upper().endswith(SAFE_KEY_SUFFIX) and is_literal(val, shell) and key not in hits:
                    hits.append(key)
            for dm in DEFAULT.finditer(line):
                if dm.group(2).startswith('$'):      # default is itself an expansion / command substitution
                    continue
                if not dm.group(1).upper().endswith(SAFE_KEY_SUFFIX) and dm.group(1) not in hits:
                    hits.append(dm.group(1))
            for key in hits:
                ent = f'{f}:{key}'
                if ent in allow:
                    allow[ent] = True; continue
                print(f'{f}:{n}:{key}'); found += 1
    for ent, used in allow.items():
        if not used:
            print(f'STALE ALLOWLIST ENTRY: {ent}'); found += 1
    return 1 if found else 0

sys.exit(main())
