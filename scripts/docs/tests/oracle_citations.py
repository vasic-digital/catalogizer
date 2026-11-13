#!/usr/bin/env python3
"""oracle_citations.py - independent oracle (shares no code with check_pdf_coverage.py / pdf_table_cols.py): no digit run and no file:line citation of a Markdown doc
may be split in its PDF twin.

Purpose  (1) every maximal digit run of 2 or more digits in the Markdown (outside link targets) must occur at least as often as a whole digit run in
         `pdftotext -raw` output of the PDF (a split run such as 10|0 loses the run); (2) every `ext:NN` / `ext:NN-MM` citation tail must occur as a
         whole pdftotext -bbox word. Reports each violation. Usage: oracle_citations.py FILE.md ... ; exit 0 clean, 1 violations, 2 usage/tool error.
"""
import collections, html, re, subprocess, sys


def md_text(p):
    t = open(p, encoding='utf-8', errors='replace').read()
    t = re.sub(r'<!--.*?-->', ' ', t, flags=re.S)
    t = re.sub(r'\]\([^)]*\)', ']', t)                       # link targets are not rendered
    t = re.sub(r'https?://\S+', ' ', t)                       # bare URLs may legitimately wrap
    t = re.sub(r'^\s*(```|~~~).*$', ' ', t, flags=re.M)
    return t


def main(argv):
    if not argv: print(__doc__, file=sys.stderr); return 2
    bad = 0
    for md in argv:
        pdf = md[:-3] + '.pdf'
        try:
            raw = subprocess.run(['pdftotext', '-raw', pdf, '-'], capture_output=True, text=True, check=True).stdout
            box = subprocess.run(['pdftotext', '-bbox', pdf, '-'], capture_output=True, text=True, check=True).stdout
        except (OSError, subprocess.CalledProcessError) as e:
            print('ERROR %s: %s' % (md, e)); return 2
        t = md_text(md)
        want = collections.Counter(re.findall(r'(?<!\d)\d{2,}(?!\d)', t))
        have = collections.Counter(re.findall(r'(?<!\d)\d{2,}(?!\d)', raw))
        words = {html.unescape(w) for w in re.findall(r'>([^<]+)</word>', box)}
        v = []
        for k, n in want.items():
            if have[k] < n: v.append('digit run %s: %d in md, %d whole in pdf' % (k, n, have[k]))
        for tail in sorted(set(re.findall(r'(?<=[A-Za-z]):\d+(?:-\d+)?', t))):
            cit = set(re.findall(r'[A-Za-z0-9]+' + re.escape(tail) + r'(?!\d)', t))
            for c in cit:
                if not any(w.endswith(c) or c in w for w in words): v.append('citation tail %s not a whole word' % c)
        print('%s %s: %d violation(s)' % ('FAIL' if v else 'OK  ', md, len(v)))
        for x in v[:10]: print('   ' + x)
        bad += len(v)
    return 1 if bad else 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
