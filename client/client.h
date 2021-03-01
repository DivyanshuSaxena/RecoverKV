#include <string>
#include <iostream>
#include <exception>
#include <uuid/uuid.h>

#include <grpcpp/grpcpp.h>
#include "recoverKV.grpc.pb.h"

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;

using recoverKV::RecoverKV;
using recoverKV::Request;
using recoverKV::Response;
using recoverKV::StateRequest;
using recoverKV::KillRequest;
using recoverKV::PartitionRequest;

using namespace std;

// Stub class for overriding proto
class RecoverKVClient
{
public:

    RecoverKVClient(std::shared_ptr<Channel> channel) : stub_(RecoverKV::NewStub(channel)) {}

    int getValue(char *client_id, char *key, char *value);
    int setValue(char *client_id, char *key, char *value, char *old_value);

    int initLBState(char *client_id, string servers_list);
    int freeLBState(char *client_id);

    int stopServer(char *client_id, char *server_name, int clean);
    int partitionServer(char *client_id, char *server_name, string reachable_list);

private:
    std::unique_ptr<RecoverKV::Stub> stub_;
};

// Client callable methods
class KV739Client
{
public:

    KV739Client();

    ~KV739Client();

    int kv739_init(char **server_names);
    int kv739_shutdown(void);
    int kv739_get(char *key, char *value);
    int kv739_put(char *key, char *value, char *old_value);

    int kv739_die(char *server_name, int clean);
    int kv739_partition(char *server_name, char **reachable);

private:

    char *client_id;
    string lb_addr;

    uuid_t id;
    RecoverKVClient *client;
};