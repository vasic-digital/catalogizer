#!/usr/bin/env bash
# fix-r4 ground truth for the shell grammar the scanners model, read from REAL shells/tools on this host.
# (1) wrappers: `<wrapper form> echo MARK` must run echo (MARK printed); a wrong shape must not.
# (2) compound commands: the output of the WHOLE compound command is the stdin of the next pipeline stage.
# (3) program readers: does the stage read its PROGRAM from stdin (prints PROG) or treat stdin as data (prints DATA / nothing)?
# NEEDLES: every block starts with a positive control that must print MARK/PROG; a block whose control is silent is reported BLIND.
set -u
RES=0
t() { # <label> <expected substring | none> <command...> : run via bash -c, show first line
  local lab="$1" want="$2"; shift 2
  out="$(timeout 20 bash -c "$*" 2>&1 </dev/null | head -2 | tr '\n' ' ')"
  if [ "$want" = none ]; then case "$out" in *MARK*|*PROG*) r="UNEXPECTED($out)"; RES=1;; *) r="none (ok)";; esac
  else case "$out" in *"$want"*) r="$want";; *) r="MISSING(want $want, got: $out)"; RES=1;; esac; fi
  printf '%-78s -> %s\n' "$lab" "$r"
}
echo "# (1) wrappers"
t "control: echo MARK" MARK 'echo MARK'
t "timeout 60 echo" MARK 'timeout 60 echo MARK'
t "timeout -s KILL 60 echo" MARK 'timeout -s KILL 60 echo MARK'
t "timeout --signal=KILL -k 5 60 echo" MARK 'timeout --signal=KILL -k 5 60 echo MARK'
t "timeout 1.5m echo" MARK 'timeout 1.5m echo MARK'
t "nice echo" MARK 'nice echo MARK'
t "nice -n 5 echo" MARK 'nice -n 5 echo MARK'
t "ionice -c 3 echo" MARK 'ionice -c 3 echo MARK'
t "ionice -c 2 -n 4 echo" MARK 'ionice -c 2 -n 4 echo MARK'
t "env echo" MARK 'env echo MARK'
t "env -u FOO echo" MARK 'env -u FOO echo MARK'
t "env FOO=1 echo" MARK 'env FOO=1 echo MARK'
t "env -i FOO=1 /usr/bin/echo" MARK 'env -i FOO=1 /usr/bin/echo MARK'
t "stdbuf -oL echo" MARK 'stdbuf -oL echo MARK'
t "stdbuf -i0 -o0 echo" MARK 'stdbuf -i0 -o0 echo MARK'
t "nohup echo" MARK 'nohup echo MARK'
t "setsid echo" MARK 'setsid echo MARK'
t "busybox echo" MARK 'busybox echo MARK'
t "command echo" MARK 'command echo MARK'
t "exec echo" MARK 'exec echo MARK'
t "time echo" MARK 'time echo MARK'
t "xargs echo" MARK 'echo MARK | xargs echo'
t "stacked: env FOO=1 timeout -s KILL 60 nice -n 5 echo" MARK 'env FOO=1 timeout -s KILL 60 nice -n 5 echo MARK'
t "sudo -h lists -u/--user as value options (help read, not run)" "user" 'sudo -h 2>&1 | grep -m1 -e "--user"'
t "negative control: a wrong shape does not run the command (nice -n echo MARK: echo is the adjustment)" none 'nice -n echo MARK'
echo "# (2) compound commands: the whole compound is one pipeline stage"
t "control: echo MARK | cat" MARK 'echo MARK | cat'
t "for ... done | cat" MARK 'for u in a; do echo MARK; done | cat'
t "while ... done < file | cat" MARK 'printf "a\n" >/dev/shm/r4gt.$$; while read u; do echo MARK; done </dev/shm/r4gt.$$ | cat; rm -f /dev/shm/r4gt.$$'
t "until ... done | cat" MARK 'until echo MARK; do :; done | cat'
t "if ... fi | cat" MARK 'if true; then echo MARK; fi | cat'
t "case ... esac | cat" MARK 'case x in x) echo MARK ;; esac | cat'
t "{ ...; } | cat" MARK '{ echo MARK; } | cat'
t "( ... ) | cat" MARK '( echo MARK ) | cat'
t "for ((;;)) ... done | cat" MARK 'for ((i=0;i<1;i++)); do echo MARK; done | cat'
t "nested { for ...; } | cat" MARK '{ for u in a; do echo MARK; done; } | cat'
t "negative control: the compound is NOT the stage before | when a ; ends it" none 'for u in a; do echo MARK; done >/dev/null; echo hi | cat | grep MARK'
echo "# (3) program readers (stdin = PROGRAM or DATA)"
t "control: echo 'echo PROG' | bash" PROG "echo 'echo PROG' | bash"
t "sh" PROG "echo 'echo PROG' | sh"
t "bash -s -- arg" PROG "echo 'echo PROG' | bash -s -- a"
t "bash -" PROG "echo 'echo PROG' | bash -"
t "bash -eu" PROG "echo 'echo PROG' | bash -eu"
t "bash -x" PROG "echo 'echo PROG' | bash -x 2>/dev/null"
t "bash -o pipefail" PROG "echo 'echo PROG' | bash -o pipefail"
t "bash -O extglob" PROG "echo 'echo PROG' | bash -O extglob"
t "bash --norc" PROG "echo 'echo PROG' | bash --norc"
t "bash -c 'cat' (inline program: stdin is DATA)" none "echo 'echo PROG' | bash -c 'tr a-z A-Z' | grep -c PROG"
t "bash -c 'cat' output is the data uppercased (proof stdin was data)" "ECHO PROG" "echo 'echo PROG' | bash -c 'tr a-z A-Z'"
echo 'echo SCRIPTRAN' > /dev/shm/r4gt-script.sh
t "bash ./script (script operand: stdin is DATA)" SCRIPTRAN "echo 'echo PROG' | bash /dev/shm/r4gt-script.sh"
t "bash ./script does not run the stdin program" none "echo 'echo PROG' | bash /dev/shm/r4gt-script.sh | grep -c PROG"
rm -f /dev/shm/r4gt-script.sh
t "source /dev/stdin" PROG "echo 'echo PROG' | { source /dev/stdin; }"
t ". /dev/stdin" PROG "echo 'echo PROG' | { . /dev/stdin; }"
t "python3 (no args)" PROG "echo 'print(\"PROG\")' | python3"
t "python3 -" PROG "echo 'print(\"PROG\")' | python3 -"
t "python3 -W ignore" PROG "echo 'print(\"PROG\")' | python3 -W ignore"
t "python3 -X dev -" PROG "echo 'print(\"PROG\")' | python3 -X dev -"
t "python3 -I" PROG "echo 'print(\"PROG\")' | python3 -I"
t "python3 -m json.tool (module: stdin is DATA)" none "echo '{}' | python3 -m json.tool | grep -c PROG"
t "python3 -c (inline: stdin is DATA)" none "echo 'print(\"PROG\")' | python3 -c 'import sys;sys.stdin.read()'"
t "perl" PROG "echo 'print qq(PROG)' | perl"
t "perl -I lib" PROG "echo 'print qq(PROG)' | perl -I /tmp"
t "perl -w" PROG "echo 'print qq(PROG)' | perl -w"
t "perl -ne (inline loop: stdin is DATA)" none "echo 'print qq(PROG)' | perl -ne 'print uc' | grep -c PROG"
t "ruby" PROG "echo 'puts :PROG' | ruby"
t "ruby -r json" PROG "echo 'puts :PROG' | ruby -r json"
t "ruby -I lib" PROG "echo 'puts :PROG' | ruby -I /tmp"
t "ruby -e (inline: stdin is DATA)" none "echo 'puts :PROG' | ruby -e 'puts 1' | grep -c PROG"
t "node" PROG "echo 'console.log(\"PROG\")' | node"
t "node -r module" PROG "echo 'console.log(\"PROG\")' | node -r fs"
t "node -e (inline: stdin is DATA)" none "echo 'console.log(\"PROG\")' | node -e '1' | grep -c PROG"
echo "# result: $([ $RES = 0 ] && echo ALL-AS-MODELLED || echo DIFFERENCES-FOUND)"
exit $RES
