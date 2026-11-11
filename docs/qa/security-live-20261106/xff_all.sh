#!/bin/bash
E=$(dirname "$0")
for t in http://127.0.0.1:38082 http://192.168.1.115:38082 http://192.168.1.115:33080; do
  for i in 1 2 3; do $E/run_xff.sh $t 198.51.100.$i 203.0.113.$i; done
done
