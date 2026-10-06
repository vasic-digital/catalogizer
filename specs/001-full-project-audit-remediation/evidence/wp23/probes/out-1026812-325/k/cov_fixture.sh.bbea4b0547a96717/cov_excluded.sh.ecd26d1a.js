var data = {lines:[
{"lineNum":"    1","line":"#!/usr/bin/env bash"},
{"lineNum":"    2","line":"# a vendored script the fence excludes: its lines must not be counted."},
{"lineNum":"    3","line":"echo vendored","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    4","line":"if false; then","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    5","line":"  echo never","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    6","line":"fi"},
]};
var percent_low = 25;var percent_high = 75;
var header = { "command" : "cov_fixture.sh", "date" : "2026-10-06 21:11:07", "instrumented" : 3, "covered" : 0,};
var merged_data = [];
