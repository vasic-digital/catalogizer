# container_hygiene_shopts.awk - shell-option reader for scripts/tests/test_container_hygiene.sh (rule R3)
#
# Purpose:  input = normalised "RUN <body>" lines (see run_lines in the guard). An errexit or pipefail setting counts
#           only when it is a real command at a command position (line start after RUN --flags, or after ; & | ( )
#           and only for the text AFTER it: `set -e`, `set -eu`, `set -o errexit`, `sh -e`/`bash -ec`/`/bin/sh -e ...`;
#           `set -o pipefail`, `set -euo pipefail`, `bash -o pipefail -c`. A word that merely contains "errexit" or
#           "pipefail" (`echo nopipefail`) does not count, and a `set -e` after the build does not cover it.
#           After errexit, `;` is rewritten as ` && `; after pipefail, a single `|` is rewritten as ` && `.
# Usage:    awk -f scripts/tests/container_hygiene_shopts.awk  (filter, stdin to stdout)
# Side-fx:  none.
# Kept in its own file (not inline in the shell script) so it is read as awk, not as shell text.
# Honest limit: an option set inside a nested `sh -c "..."` is treated as applying to the rest of the whole line.
#               `set +e` / `set +o pipefail` are not modelled (the option stays on once set).
BEGIN {
  SEP = "(^RUN([ \t]+--[^ \t]+)*[ \t]+|[;&|(][ \t]*)"
  CMD = "([^ \t;&|()\"\047]*/)?"
  ERR = SEP CMD "(set([ \t]+-[A-Za-z]+)*[ \t]+(-[A-Za-z]*e[A-Za-z]*|-o[ \t]+errexit)|(ba|da)?sh([ \t]+-[A-Za-z]+)*[ \t]+-[A-Za-z]*e[A-Za-z]*)([ \t;&)\"\047]|$)"
  PF  = SEP CMD "(set|(ba|da)?sh)([ \t]+-[A-Za-z]+)*[ \t]+-[A-Za-z]*o[ \t]+pipefail([ \t;&)\"\047]|$)"
}
# cut(s, re): index just after the separator of the first match of re in s, 0 if none
function cut(s, re,   p, q) {
  if (!match(s, re)) return 0
  p = RSTART; q = substr(s, p); match(q, SEP); return p + RLENGTH
}
{
  s = $0
  c = cut(s, ERR)
  if (c) { pre = substr(s, 1, c - 1); post = substr(s, c); gsub(/;/, " \\&\\& ", post); s = pre post }
  c = cut(s, PF)
  if (c) { pre = substr(s, 1, c - 1); post = substr(s, c)
    while (match(post, /[^|]\|[^|]/)) post = substr(post, 1, RSTART) " && " substr(post, RSTART + 2)
    s = pre post }
  print s
}
