#!/usr/bin/env bash
cat <<END-DATA >/dev/null
data line
END-DATA
echo a # trailing <<WORD is a comment
cat <<ONE <<TWO >/dev/null
one body
ONE
two body
TWO
echo z
