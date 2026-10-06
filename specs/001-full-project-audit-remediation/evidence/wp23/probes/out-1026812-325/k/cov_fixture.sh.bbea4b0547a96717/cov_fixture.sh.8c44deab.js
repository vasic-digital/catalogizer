var data = {lines:[
{"lineNum":"    1","line":"#!/usr/bin/env bash"},
{"lineNum":"    2","line":"# fixture of the T199 bash line-trace harness: one covered function, one never-called function, one branch never taken."},
{"lineNum":"    3","line":"set -u","class":"lineCov","hits":"1","order":"1","possible_hits":"0",},
{"lineNum":"    4","line":"covered_fn() {"},
{"lineNum":"    5","line":"  local a=1","class":"lineCov","hits":"1","order":"3","possible_hits":"0",},
{"lineNum":"    6","line":"  local b=2","class":"lineCov","hits":"1","order":"4","possible_hits":"0",},
{"lineNum":"    7","line":"  echo $((a+b))","class":"lineCov","hits":"1","order":"5","possible_hits":"0",},
{"lineNum":"    8","line":"}"},
{"lineNum":"    9","line":"uncovered_fn() {"},
{"lineNum":"   10","line":"  local c=3","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"   11","line":"  echo \"never $c\"","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"   12","line":"}"},
{"lineNum":"   13","line":"covered_fn","class":"lineCov","hits":"1","order":"2","possible_hits":"0",},
{"lineNum":"   14","line":"if [ \"${1:-}\" = never ]; then","class":"lineCov","hits":"1","order":"6","possible_hits":"0",},
{"lineNum":"   15","line":"  echo \"branch not taken\"","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"   16","line":"fi"},
{"lineNum":"   17","line":"echo done","class":"lineCov","hits":"1","order":"7","possible_hits":"0",},
]};
var percent_low = 25;var percent_high = 75;
var header = { "command" : "cov_fixture.sh", "date" : "2026-10-06 21:11:07", "instrumented" : 10, "covered" : 7,};
var merged_data = [];
