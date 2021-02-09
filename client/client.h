#include <string>
#include <iostream>

#include <grpcpp/grpcpp.h>
#include "recoverKV.grpc.pb.h"

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;

using recoverKV::RecoverKV;
using recoverKV::Request;
using recoverKV::Response;

using namespace std;

// Client callable methods
int kv739_init(char *server_name);
int kv739_shutdown(void);
int kv739_get(char *key, char *value);
int kv739_put(char *key, char *value, char *old_value);

// Stub class for overriding proto
class RecoverKVClient {
    public:
        RecoverKVClient(std::shared_ptr<Channel> channel) : stub_(RecoverKV::NewStub(channel)) {}
        int sendRequest(char *key, char *value);

    private:
        std::unique_ptr<RecoverKV::Stub> stub_;
};