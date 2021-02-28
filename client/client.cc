#include "client.h"

// Stub method to get value for a key from server
int RecoverKVClient::getValue(size_t object_id, char *key, char *value) {
  Request request;
  Response response;
  ClientContext context;

  request.set_key(key);
  request.set_value("");

  Status status = stub_->getValue(&context, request, &response);

  if (status.ok()) {
    int successCode = response.successcode();

    // if successful attempt then update value for returning
    if (successCode == 0) {
      copy(response.value().begin(), response.value().end(), value);
      value[response.value().size()] = '\0';
    }

    return successCode;
  } else {
    cout << "[ERROR]" << status.error_code() << ": " << status.error_message()
         << endl;
    return -1;
  }
}

// Stub method to update value for a key at the server
int RecoverKVClient::setValue(size_t object_id, char *key, char *value, char *old_value) {
  Request request;
  Response response;
  ClientContext context;

  request.set_key(key);
  request.set_value(value);

  Status status = stub_->setValue(&context, request, &response);

  if (status.ok()) {
    int successCode = response.successcode();

    // if old value existed for this key
    if (successCode == 0) {
      copy(response.value().begin(), response.value().end(), old_value);
      old_value[response.value().size()] = '\0';
    }

    return successCode;
  } else {
    cout << "[ERROR]" << status.error_code() << ": " << status.error_message()
         << endl;
    return -2;
  }
}

// Stub method to establish client state at load balancer
int RecoverKVClient::initLBState(size_t object_id, string servers_list) {
	StateRequest request;
	Response response;
	ClientContext context;

	request.set_objectid(object_id);
  	request.set_serverslist(servers_list);

	Status status = stub_->initLBState(&context, request, &response);

  	if (!status.ok()) {
		cout << "[ERROR]" << status.error_code() << ": " << status.error_message()
     		 << endl;
		return -1;   
	}
	
	return response.successcode();
}

// Stub method to free client state at load balancer
int RecoverKVClient::freeLBState(size_t object_id) {
	StateRequest request;
	Response response;
	ClientContext context;

	request.set_objectid(object_id);
  	request.set_serverslist("");

  	Status status = stub_->freeLBState(&context, request, &response);

  	if (!status.ok()) {
		cout << "[ERROR]" << status.error_code() << ": " << status.error_message()
     		 << endl;
		return -1;   
	}
	
	return response.successcode();
}

// Stub method for stopping/killing server
int RecoverKVClient::stopServer(size_t object_id, char* server_name, int clean) {

	KillRequest request;
	Response response;
	ClientContext context;

	request.set_objectid(object_id);
	request.set_servername(server_name);
  	request.set_cleantype(clean);

  	Status status = stub_->stopServer(&context, request, &response);

  	if (!status.ok()) {
		cout << "[ERROR]" << status.error_code() << ": " << status.error_message()
     		 << endl;
		return -1;   
	}
	
	return response.successcode();
}

// Stub method partitioning server network
int RecoverKVClient::partitionServer(size_t object_id, char* server_name, string reachable_list) {
	
	PartitionRequest request;
	Response response;
	ClientContext context;

	request.set_objectid(object_id);
	request.set_servername(server_name);
  	request.set_reachable(reachable_list);

  	Status status = stub_->partitionServer(&context, request, &response);

  	if (!status.ok()) {
		cout << "[ERROR]" << status.error_code() << ": " << status.error_message()
     		 << endl;
		return -1;   
	}
	
	return response.successcode();
}

// Default constructor for the client interface
KV739Client::KV739Client() { 

	// object iddentifier for this client
	object_id = 0;

	// Hardcode load balancer address
	lb_addr = "localhost:50050";

	try {

		client = new RecoverKVClient(
		grpc::CreateChannel(lb_addr, grpc::InsecureChannelCredentials()));

	} catch (exception &e) {

		client = NULL;
		cout << "[ERROR]" << e.what() << '\n';
	}
}

// Destructor for clean up
KV739Client::~KV739Client() {

	this->kv739_shutdown(); 

	delete client;
  	client = NULL;
}

// Establish grpc connection with the load balancer
int KV739Client::kv739_init(char **server_names) {

  if (client == NULL) {
    cout << "[ERROR]"
         << "Could not connect to load balancer" << endl;
    return -1;
  }

  if (object_id != 0) {
    cout << "[ERROR]"
         << "Cannot init again" << endl;
    return -1;
  }

  if (server_names == NULL) {
  	cout << "[ERROR]"
        << "No server names provided to init" << endl;
    return -1;
  }

  // Parse list of input server instances
  int idx = 0;
  string server_str = "";
  string servers_list = "";
  while (server_names[idx]) {

  	if ((server_names[idx] != NULL) && (server_names[idx][0] == '\0')) {
    	cout << "[ERROR]"
         << "Invalid server name format in list" << endl;
    	return -1;
  	}

  	server_str += server_names[idx];
  	servers_list = servers_list + server_names[idx] + ",";
  	idx += 1;
  }  

  // Generate user id wrt servers initialized
  hash<string> hasher;
  sort(server_str.begin(), server_str.end());
  object_id = hasher(server_str);

  // Send server names to load balancer
  if (client->initLBState(object_id, servers_list) == -1) {
	cout << "[ERROR]"
     << "Could not establish connection with server instances" << endl;
	return -1;
  }

  // init successful
  return 0;
}

// Free state for the connection
int KV739Client::kv739_shutdown() {

  // Tell load balancer to free state
  if (client->freeLBState(object_id) == -1) {
  	cout << "[ERROR]"
     << "Unable to free connection state" << endl;
  	return -1;
  }
  object_id = 0;

  // shutdown successful
  return 0;
}

// Retrieve values from server through grpc tunnel
int KV739Client::kv739_get(char *key, char *value) {
  if (client == NULL || object_id == 0) {
    cout << "[ERROR]"
         << "Client not initialized" << endl;
    return -1;
  }

  if ((key != NULL) && (key[0] == '\0')) {
    cout << "[ERROR]"
         << "Key cannot be empty" << endl;
    return -1;
  }

  // send get request to server fir this object id
  return client->getValue(object_id, key, value);
}

// Update values at server through grpc tunnel
int KV739Client::kv739_put(char *key, char *value, char *old_value) {
  if (client == NULL || object_id == 0) {
    cout << "[ERROR]"
         << "Client not initialized" << endl;
    return -1;
  }

  if ((key != NULL) && (key[0] == '\0')) {
    cout << "[ERROR]"
         << "Key cannot be empty" << endl;
    return -1;
  }

  if ((value != NULL) && (value[0] == '\0')) {
    cout << "[ERROR]"
         << "New value cannot be empty" << endl;
    return -1;
  }

  // send update request to server for this object if
  return client->setValue(object_id, key, value, old_value);
}

int KV739Client::kv739_die(char *server_name, int clean) {
  if (client == NULL || object_id == 0) {
    cout << "[ERROR]"
         << "Client not initialized" << endl;
    return -1;
  }

  if ((server_name != NULL) && (server_name[0] == '\0')) {
    cout << "[ERROR]"
         << "Invalid server name" << endl;
    return -1;
  }

  return client->stopServer(object_id, server_name, clean);
}

int KV739Client::kv739_partition(char *server_name, char **reachable) {

  if (client == NULL || object_id == 0) {
    cout << "[ERROR]"
         << "Client not initialized" << endl;
    return -1;
  }

  if ((server_name != NULL) && (server_name[0] == '\0')) {
    cout << "[ERROR]"
         << "Invalid server name" << endl;
    return -1;
  }

  if (reachable == NULL) {
  	cout << "[ERROR]"
         << "Reachable array is not initialized" << endl;
    return -1;
  }

  // Parse reachable servers array
  int idx = 0;
  string reachable_list = "";
  while (reachable[idx]) {

  	// not reachable from any other servers
  	if ((reachable[idx] != NULL) && (reachable[idx][0] == '\0')) {
    	return client->partitionServer(object_id, server_name, "");
  	}

  	reachable_list = reachable_list + reachable[idx] + ",";
  	idx += 1;
  }

  return client->partitionServer(object_id, server_name, reachable_list);

}