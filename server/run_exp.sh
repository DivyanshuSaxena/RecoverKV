# Run the experiments

# Start the five LBs corresponding to the servers
./recoverLB 0 5 0 &
./recoverLB 1 5 0 &
./recoverLB 2 5 0 &
./recoverLB 3 5 0 &
./recoverLB 4 5 0 &

# Start the backend servers
./run_server.sh 0 50051 0 &
./run_server.sh 1 50054 0 &
./run_server.sh 2 50057 0 &
./run_server.sh 3 50060 0 &
./run_server.sh 4 50063 0 &

# Trap Ctrl^C
trap ctrl_c INT
function ctrl_c() {
    echo "Killing remaining servers"
    pgrep run_server | xargs kill -9
    pgrep recover | xargs kill -9
}