#!/usr/bin/env bash
# pipeline and and/or continuation fixture (review i1: the tracer reports each element's own line)
echo one two three | \
  tr ' ' '\n' | \
  sort -r | \
  head -n 2 >/dev/null
echo done
true && \
  echo and >/dev/null
echo \
  "an argument continuation: only the first line is a command line" >/dev/null
