var data = {lines:[
{"lineNum":"    1","line":"#!/usr/bin/env bash"},
{"lineNum":"    2","line":"# drives the fixtures the way a test suite drives scripts (through child bash processes)"},
{"lineNum":"    3","line":"d=\"$(cd \"$(dirname \"$0\")\" && pwd)\"","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    4","line":"bash \"$d/cov_fixture.sh\" >/dev/null","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    5","line":"bash \"$d/cov_fixture2.sh\" >/dev/null","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    6","line":"bash \"$d/cov_excluded.sh\" >/dev/null","class":"lineNoCov","hits":"0","possible_hits":"0",},
]};
var percent_low = 25;var percent_high = 75;
var header = { "command" : "cov_fixture.sh", "date" : "2026-10-06 21:11:07", "instrumented" : 4, "covered" : 0,};
var merged_data = [];
