#include <iostream>
#include "../client.h"

using namespace std;

#define KEY_SIZE 50
#define VALUE_SIZE 100

int main(int argc, char * argv[]) {

	// get server name from command line
	char *serverName = argv[1];

	// construct client
	KV739Client *client = new KV739Client();

	// intialize connection to server
	if (client->kv739_init(serverName) == -1) {
		return 0;
	}

	cout << "Latency, Throughput,.." << endl;


	// free state and disconnect from server
	client->kv739_shutdown();
	delete client;


	return 0;
}