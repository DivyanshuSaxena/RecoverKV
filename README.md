# RecoverKV
A strongly consistent distributed KV store.

## Design

RecoverKV is designed and implemented to be strongly-consistent in both failure and non-failure cases, provided that there is no partition.  

RecoverKV uses a quorum protocol where `R=1` and `W=N` hence, `R+W > N`. Writes go to all the servers, while reads can be done on any one server.
Each server maintains an in-memory map which is used for fast look-ups and also logs updates to the key-value pairs in a SQLite3 table, which is used for recovery.

Writes to the servers are managed by a load balancer which ensures the writes are always in order by assigning every write query a unique id (uid).
These immutable objects are maintained in an unordered log at each server. These logs serve as the backbone for our consistent recovery procedure.

For more details, please check the reports for this project. These reports document the incremental changes to the KV Store:

- Durability [Report](docs/739_RecoverKV_report1.pdf)
- Strong consistency and High Availability [Report](docs/739_RecoverKV_report2.pdf)
- System design to counter Load Balancer failures [Report](docs/739_RecoverKV_report3.pdf)

## Instructions for running the KV store

### Starting load balancer and servers

#### Using fixed architecture
For running the three-node fixed architecture please use the `launch_system.sh` file:
```bash
cd server

# Start three backend server and lb
bash launch_system.sh
```

The script will start the servers at: `localhost:50051 localhost:50052 localhost:50053`

#### Using dynamic architecture
For running flexible set of servers:
```bash
cd server

# start a server
./run_server.sh <server ip> <server port> <recovery port> <LB ip> <LB port> 1

# Example Usage: For starting three servers on localhost
./run_server.sh localhost 50051 50054 localhost 50050 1
./run_server.sh localhost 50052 50055 localhost 50050 1
./run_server.sh localhost 50053 50056 localhost 50050 1

# Then start the load balancer
./recoverLB <ip_addr:port> <number of servers> <server 1 ip:port> <server 2 ip:port> ... <server 1s recovery port> <server 2s recovery port> ...

# Example Usage: (with respect to above started servers)
./recoverLB localhost:50050 3 localhost:50051 localhost:50052 localhost:50053 50054 50055 50056

```

Note: Before starting the tests, give load balancer a few seconds to register all the servers on startup. Currently, we only support one load balancer.

### Client Interface Testing
The client interface implements the methods to access the key/value server instance, based on GRPC protocol. Each client is an instance of the `KV739Client` class, the
exact methods implemented can be found in `client/client.h`.

Therefore, you will have to modify your testing script, to create an instance of `KV739Client` class and then call API methods.

\##Example Usage: (with respect to above started servers)\
You will be passing `localhost:50051 localhost:50052 localhost:50053` server addresses to your testing scripts.

Note: In the tests, after killing a server please give it >5 seconds to recover back to consistent state.

## Instructions for Developers
```bash
cd /server/
#go mod tidy
make gen
make build
# Now user your choice of screen multiplexer to initiate bellow commands in each shell
# To start the load balancer
./recoverLB <ip_addr:port> <number of servers> <server 1 ip:port> <server 2 ip:port> ... <server 1s recovery port> <server 2s recovery port> ...
# Start as many servers as specified in load balancer above
./run_server.sh <server ip> <server port> <recovery port> <LB ip> <LB port> <Delete previous data>
```
Here is an example run, with load balancer managing 3 servers.
```bash
./recoverLB localhost:50050 3 localhost:50051 localhost:50052 localhost:50053 50054 50055 50056
./run_server.sh localhost 50051 50054 localhost 50050 1
./run_server.sh localhost 50052 50055 localhost 50050 1
./run_server.sh localhost 50053 50056 localhost 50050 1
```

## Client

The client interface implements the methods to access the key/value server instance, based on GRPC protocol. Each client is an instance of the `KV739Client` class, the
exact methods implemented can be found in `client/client.h`.

### Prerequisite

For compilation, the scripts assume that GRPC C++ Plugin libraries are installed in `/usr/local`, or are added on to the `$PATH` environment variable.

### Build

Run `make` inside the `client/` directory to generate a new `lib739kv.so`. This shared object file can then be used in any C/C++ application.

## Server

The server interface is written in Go and implements the key value service. Install go and golang grpc packages.

### Instructions to build and execute

For the first time, run:

```bash
cd server/
make gen
```

If any go dependency issues arise, run `make clean` and then `make` inside the `server/` directory. For subsequent runs, the following suffices:

```bash
make build
```

The above command generates an executable `recoverKV`. By default, this starts the service at port 50051 on the localhost.

### Logging and Storage

The key-value service uses a SQLite DB file located at `/tmp/test.db`. For clearing the database, stop the server and then remove the `test.db` file to start the service from a clean slate.  
Additionally, the server also maintains some logs in the `server/server.log` directory.

## Tests

Run `make test` inside the `client/` directory to generate testing executables. We provide one correctness and two performance tests.

### Performance Tests

After starting server instance, conduct the performance tests:

```bash
./performance_test <server-ip>:<port> <num-keys> <batch-size>
```

The second argument above is the total number of keys that will be used for the tests and the third argument is the batch size that will be used for sub test 3.  

e.g `./performance_test 127.0.0.1:50051 10000 100`

The other performance test is to measure the service throughput in situation of multiple client connection. Use the following command:

```bash
./multiperf_test <server-ip>:<port> <num-keys> <num-parallel-clients>
```

e.g `./multiperf_test 127.0.0.1:50051 10000 4`

### Correctness Test

After starting server instance, conduct the correctness tests:

```bash
./correctness_test <server-ip>:<port>
```

e.g `./correctness 127.0.0.1:50051`

In either of performance or correctness tests, based on the instructions output by the testing script, you might have to shutdown/restart the server at appropriate times.
