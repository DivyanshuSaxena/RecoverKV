# RecoverKV
A strongly consistent distributed KV store.
## Starting load balancer and srevers
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

### Developer

Run `make` inside the `client/` directory to generate a new `lib739kv.so`. This shared object file can then be used in any C/C++ application.

## Server

The server interface is written in Go and implements the key value service.

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

### Performance tests

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

### Correctness test

After starting server instance, conduct the correctness tests:

```bash
./correctness_test <server-ip>:<port>
```

e.g `./correctness 127.0.0.1:50051`

In either of performance or correctness tests, based on the instructions output by the testing script, you might have to shutdown/restart the server at appropriate times.
