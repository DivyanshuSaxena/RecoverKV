#!/bin/bash

# Arguments
# 
# 1: server ID
# 2: serve port
# 3: Whether to clean logs and data or not

if [[ $3 -gt 0 ]]; then
  rm -f -- server$2.log
  rm -f -- /tmp/$2
  rm -f -- /tmp/$2-wal
  rm -f -- /tmp/$2-shm
fi
./recoverKV $1 $2