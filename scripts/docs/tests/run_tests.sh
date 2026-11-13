#!/bin/sh
# run_tests.sh - run the docs tooling tests without pytest: run_tests.sh [-k SUBSTRING] [FILE ...]  (see run_tests.py). Needs python3, pandoc, weasyprint, poppler (pdftotext).
exec nice -n 10 python3 "$(dirname "$0")/run_tests.py" "$@"
