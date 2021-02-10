#include <iostream>
#include "client.h"

using namespace std;

int main(int argc, char * argv[]) {

	KV739Client clientKV;

	char serverName[] = "localhost:8081";

	cout << "Response: " << clientKV.kv739_init(serverName) << endl; 

	return 0;
}