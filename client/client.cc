#include "client.h"

int RecoverKVClient::sendRequest(char *key, char *value) {
    Request request;
    Response response;
    ClientContext context;

    request.set_key(key);
    request.set_value(value);

    Status status = stub_->sendRequest(&context, request, &response);

    if(status.ok()){

        return response.successcode();
    } else {

        cout << status.error_code() << ": " << status.error_message() << endl;
        return -1;
    }
}

int main(int argc, char* argv[]){
    
    cout << "okay" << endl;

    return 0;
}