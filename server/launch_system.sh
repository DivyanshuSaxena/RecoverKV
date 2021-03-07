#!/bin/bash

# Trap Ctrl^C
trap ctrl_c INT
function ctrl_c() {
    pgrep recoverLB | xargs kill
    pgrep recoverKV | xargs kill
    echo "All servers killed"
}

# Hardcoded three node architecture
./run_server.sh localhost 50051 50054 localhost 50050 1 &
./run_server.sh localhost 50052 50055 localhost 50050 1 &
./run_server.sh localhost 50053 50056 localhost 50050 1 &

./recoverLB localhost:50050 3 localhost:50051 localhost:50052 localhost:50053 50054 50055 50056 &
wait
