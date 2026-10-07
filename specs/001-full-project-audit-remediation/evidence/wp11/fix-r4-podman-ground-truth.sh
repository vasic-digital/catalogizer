#!/usr/bin/env bash
# fix-r4 ground truth for the engine option grammar: which word does the REAL podman take as the image operand?
# `podman run|create --pull=never <flags> localhost/doesnotexist-wf16:N true` never pulls and never creates a container: every call ends with
# "image not known" naming the word podman took as the image. NEEDLE: the control line (no flags) must name its own image; if it does not,
# the instrument is blind and no row below means anything.
set -u
IMG() { echo "localhost/doesnotexist-wf16:$1"; }
n=0
probe() { # <verb> <flags...>  (the image operand is appended)
  n=$((n+1)); local verb="$1"; shift
  out="$(podman "$verb" --pull=never "$@" "$(IMG $n)" true 2>&1 | head -3 | tr '\n' ' ')"
  case "$out" in *"doesnotexist-wf16:$n"*) r="operand=IMAGE($n)";; *) r="NOT-THE-IMAGE: $out";; esac
  printf '%-70s -> %s\n' "podman $verb --pull=never $* <image>" "$r"
}
probe run
probe run --rm
probe run -d
probe run -dp 127.0.0.1:3999:3999
probe run -dv /tmp:/x
probe run -itw /src
probe run -dm 512m
probe run -dti -p 80:80
probe run -dp127.0.0.1:3998:3998
probe run -dv/tmp:/y
probe run --rm -d --init
probe run --name=x --rm
probe run -e A=b -d
probe run -dp=127.0.0.1:3997:3997
probe create -itw /src
probe create -dp 127.0.0.1:3999:3999
probe create -dm 512m
# `--` ends the options: the next word is the image
n=$((n+1)); out="$(podman run --pull=never --rm -- "$(IMG $n)" true 2>&1 | head -3 | tr '\n' ' ')"; case "$out" in *"doesnotexist-wf16:$n"*) r="operand=IMAGE($n)";; *) r="NOT-THE-IMAGE: $out";; esac; printf '%-70s -> %s\n' "podman run --pull=never --rm -- <image>" "$r"
# an array expansion before the image (bash): the shell turns it into the option words
ARGS=(--rm -d); n=$((n+1)); out="$(podman run --pull=never "${ARGS[@]}" "$(IMG $n)" true 2>&1 | head -3 | tr '\n' ' ')"; case "$out" in *"doesnotexist-wf16:$n"*) r="operand=IMAGE($n)";; *) r="NOT-THE-IMAGE: $out";; esac; printf '%-70s -> %s\n' 'podman run --pull=never "${ARGS[@]}" <image>' "$r"
