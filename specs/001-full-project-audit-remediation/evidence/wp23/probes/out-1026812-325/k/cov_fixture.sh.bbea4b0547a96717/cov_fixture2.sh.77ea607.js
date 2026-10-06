var data = {lines:[
{"lineNum":"    1","line":"#!/usr/bin/env bash"},
{"lineNum":"    2","line":"# fixture 2: case arms, a here-document and a continued command."},
{"lineNum":"    3","line":"x=1","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    4","line":"case \"$x\" in","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    5","line":"  1) echo one ;;","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    6","line":"  2) echo two ;;","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    7","line":"esac"},
{"lineNum":"    8","line":"cat <<EOF2","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"    9","line":"body $x"},
{"lineNum":"   10","line":"EOF2"},
{"lineNum":"   11","line":"echo a \\","class":"lineNoCov","hits":"0","possible_hits":"0",},
{"lineNum":"   12","line":"  b"},
]};
var percent_low = 25;var percent_high = 75;
var header = { "command" : "cov_fixture.sh", "date" : "2026-10-06 21:11:07", "instrumented" : 6, "covered" : 0,};
var merged_data = [];
