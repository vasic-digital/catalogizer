#!/usr/bin/env python3
"""dotenv_get.py - the ONE reader of a dotenv file for scripts/test-infra (blocked_external.sh, nas_readonly_leg.sh), WF17 TI-D11.

Usage: python3 -I dotenv_get.py <file> <VARIABLE>     prints the value (no trailing newline added beyond one) and exits 0; exit 1 when the variable is unset; exit 2 on a usage error or an unreadable file.

Semantics: the documented subset of python-dotenv 1.x that the repository's `.env` files use (the test tests/infra/test_dotenv.sh compares this reader with python-dotenv itself as the oracle):
  - lines end at LF; a trailing CR is dropped (a CRLF file reads the same as an LF file); blank lines and lines whose first non-blank character is '#' are skipped;
  - `KEY=value`, `KEY = value` and `export KEY=value` are all the same assignment; a line without `=` is not an assignment;
  - a double-quoted value is read up to its closing unescaped quote, with the escapes \\\\ \\' \\" \\a \\b \\f \\n \\r \\t \\v decoded; a single-quoted value is read literally up to the closing quote;
  - an unquoted value ends at the first ` #` (whitespace then '#': an inline comment) and is stripped of trailing whitespace;
  - a duplicate assignment: the LAST one wins (python-dotenv's dict semantics);
  - nothing is ever evaluated or expanded.
The value is printed to stdout only; this tool never writes a value anywhere else.
"""
import re
import sys

ESC = {"\\": "\\", "'": "'", '"': '"', "a": "\a", "b": "\b", "f": "\f", "n": "\n", "r": "\r", "t": "\t", "v": "\v"}
ASSIGN = re.compile(r"^(?:export\s+)?([A-Za-z_][A-Za-z0-9_.-]*)\s*=\s*(.*)$")


def parse_value(rest: str) -> str:
    if rest.startswith('"'):
        out, i = [], 1
        while i < len(rest):
            c = rest[i]
            if c == "\\" and i + 1 < len(rest) and rest[i + 1] in ESC:
                out.append(ESC[rest[i + 1]]); i += 2; continue
            if c == '"':
                return "".join(out)
            out.append(c); i += 1
        return "".join(out)
    if rest.startswith("'"):
        j = rest.find("'", 1)
        return rest[1:] if j < 0 else rest[1:j]
    return re.sub(r"\s+#.*$", "", rest).rstrip()


def main(argv) -> int:
    if len(argv) != 3 or not re.match(r"^[A-Za-z_][A-Za-z0-9_.-]*$", argv[2]):
        print("usage: dotenv_get.py <file> <VARIABLE>", file=sys.stderr)
        return 2
    try:
        text = open(argv[1], "rb").read().decode("utf-8", "replace")
    except OSError:
        return 2
    value = None
    for raw in text.split("\n"):
        line = raw.rstrip("\r").strip()
        if not line or line.startswith("#"):
            continue
        m = ASSIGN.match(line)
        if m and m.group(1) == argv[2]:
            value = parse_value(m.group(2))
    if value is None:
        return 1
    sys.stdout.write(value + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
