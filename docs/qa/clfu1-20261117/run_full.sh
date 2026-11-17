#!/bin/bash
E=docs/qa/clfu1-20261117; C=$PWD
$E/run_tv.sh $C catalogizer-android $E/green_phone_full.txt :app:testDebugUnitTest
$E/run_tv.sh $C catalogizer-androidtv $E/green_tv_full.txt :app:testDebugUnitTest
