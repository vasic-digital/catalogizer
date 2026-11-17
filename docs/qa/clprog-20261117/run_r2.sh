#!/bin/bash
E=docs/qa/clprog-20261117; C=$PWD
$E/run_tv.sh $C catalogizer-androidtv $E/green_tv_r2_full.txt :app:testDebugUnitTest
$E/run_tv.sh $C catalogizer-android $E/green_android_r2.txt :app:testDebugUnitTest --tests '*CatalogizerApiTest' --tests '*MediaRepositoryTest'
