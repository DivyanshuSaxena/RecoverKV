#include "client.h"

// Stub method to get value for a key from server
int RecoverKVClient::getValue(char *key, char *value) {
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
int RecoverKVClient::setValue(char *key, char *value, char *old_value) {
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
    return -1;
  }
}

// Default constructor for the client interface
KV739Client::KV739Client() { client = NULL; }

// Destructor for clean up
KV739Client::~KV739Client() { this->kv739_shutdown(); }

// Establish grpc connection with the server
int KV739Client::kv739_init(char *server_name) {
  if ((server_name != NULL) && (server_name[0] == '\0')) {
    cout << "[ERROR]"
         << "Invalid server name" << endl;
    return -1;
  }

  try {
    client = new RecoverKVClient(
        grpc::CreateChannel(server_name, grpc::InsecureChannelCredentials()));
    return 0;

  } catch (exception &e) {
    cout << "[ERROR]" << e.what() << '\n';
  }

  return -1;
}

// Free state for the connection
int KV739Client::kv739_shutdown() {
  delete client;
  client = NULL;

  return 0;
}

// Retrieve values from server through grpc tunnel
int KV739Client::kv739_get(char *key, char *value) {
  if (client == NULL) {
    cout << "[ERROR]"
         << "Client not initialized" << endl;
    return -1;
  }

  if ((key != NULL) && (key[0] == '\0')) {
    cout << "[ERROR]"
         << "Key cannot be empty" << endl;
    return -1;
  }

  // send get request to server
  return client->getValue(key, value);
}

// Update values at server through grpc tunnel
int KV739Client::kv739_put(char *key, char *value, char *old_value) {
  if (client == NULL) {
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

  // send update request to server
  return client->setValue(key, value, old_value);
}