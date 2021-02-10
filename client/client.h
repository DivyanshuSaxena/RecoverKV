#include <string>
#include <iostream>
#include <exception>

#include <grpcpp/grpcpp.h>
#include "recoverKV.grpc.pb.h"

using grpc::Channel;
using grpc::ClientContext;
using grpc::Status;

using recoverKV::RecoverKV;
using recoverKV::Request;
using recoverKV::Response;

using namespace std;

// Stub class for overriding proto
class RecoverKVClient {
	public:

		RecoverKVClient(std::shared_ptr<Channel> channel) : stub_(RecoverKV::NewStub(channel)) {}

		int getValue(char *key, char *value);
		int setValue(char *key, char *value, char *old_value);

	private:
		std::unique_ptr<RecoverKV::Stub> stub_;
};

// Client callable methods
class KV739Client {
	public:

		KV739Client();

		int kv739_init(char *server_name);
		int kv739_shutdown(void);
		int kv739_get(char *key, char *value);
		int kv739_put(char *key, char *value, char *old_value);

	private:

		RecoverKVClient *client;
};