# RecoverKV

## Client
The client interface implements the methods to access the key/value server instance, based on GRPC protocol. Each client is an instance of the `KV739Client` class, the
exact methods implemented can be found in `client/client.h`.

### Prerequisite
For compilation, the scripts assume that GRPC C++ Plugin libraries are installed in `/usr/local`.

### Developer
Run `make` inside the `client/` directory to generate a new `lib739kv.so`.

## Tests
Run `make test` inside the `client/` directory to generate testing executables.

### Performance test
After starting server instance, conduct the performance tests:
```
./performance_test <server-ip>:<port>
```
e.g `./performance_test 127.0.0.1:8081`

### Correctness test
After starting server instance, conduct the correctness tests:
```
./correctness_test <server-ip>:<port>
```
e.g `./correctness 127.0.0.1:8081`

Based on the instructions output by the testing script, you might have to shutdown/restart the server at appropriate times.
