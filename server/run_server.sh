#!/bin/bash

# Arguments
# 
# 1: IP Address of the server instance
# 2: Server Port
# 3: Recovery Port of the server instance
# 4: IP Address of the load balancer
# 5: Load Balancer Port
# 6: Should restart (1 if complete restart else 0)

if [[ $6 -gt 0 ]]; then
  rm -f -- server$2.log
  rm -f -- /tmp/$2
  rm -f -- /tmp/$2-wal
  rm -f -- /tmp/$2-shm
fi
./recoverKV $1 $2 $3 $4 $5