#include <iostream>
#include <unistd.h>
#include <stdio.h>

#include "../client.h"

using namespace std;

#define TOTAL_TESTS 3
#define KEY_SIZE 50
#define VALUE_SIZE 100

#define SLEEP_INTERVAL 30

// WRITES FOLLOWED BY READS TEST
int test_correctness_1(KV739Client *client, int total_requests) {

	// get process id
	pid_t processID = getpid();

	// generate key/value pairs
	char keys[total_requests][KEY_SIZE] = {0};
	char newValues[total_requests][VALUE_SIZE] = {0};
	char oldValues[total_requests][VALUE_SIZE] = {0};
	for (int i = 0; i < total_requests; i++) {
		sprintf(keys[i], "%d-%d", processID, i % total_requests);
		sprintf(newValues[i], "%x-%x", processID, i / total_requests);
	}

	// send write requests to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_put(keys[i], newValues[i], oldValues[i]) == -1) {
			cout << "Failed to send write request " << i << " in TEST 1" << endl;
			return 0;
		}
	}

	// send read request to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_get(keys[i], oldValues[i]) == -1) {
			cout << "Failed to send read request " << i << " in TEST 1" << endl;
			return 0;
		}
	}

	// measure correctness in server response
	for (int i = 0; i < total_requests; i++) {
		if (strcmp(newValues[i], oldValues[i]) != 0) {
			cout << "TEST 1 FAILED: Case " << i << " Expected " <<  newValues[i] << ", Got " << oldValues[i] << endl; 
			return 0;
		}
	}

	// test passed
	cout << "TEST 1 PASSED - SINGLE UPDATES" << endl;
	return 1;
}

// MULTIPLE WRITE CYCLES FOLLOWED BY READS
int test_correctness_2(KV739Client *client, int total_requests, int total_cycles) {
	
	// get process id
	pid_t processID = getpid();

	// generate key/value pairs
	char keys[total_requests][KEY_SIZE] = {0};
	char newValues[total_requests * total_cycles][VALUE_SIZE] = {0};
	char oldValues[total_requests][VALUE_SIZE] = {0};
	for (int i = 0; i < total_requests * total_cycles; i++) {
		sprintf(keys[i % total_requests], "%d-%d", processID, i % total_requests);
		sprintf(newValues[i], "%x-%x", processID, i / total_requests);
	}

	// send write requests to server
	for (int i = 0; i < total_requests * total_cycles; i++) {
		if (client->kv739_put(keys[i % total_requests], newValues[i], oldValues[i % total_requests]) == -1) {
			cout << "Failed to send write request " << i << " in TEST 2" << endl;
			return 0;
		}
	}

	// send read request to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_get(keys[i], oldValues[i]) == -1) {
			cout << "Failed to send read request " << i << " in TEST 2" << endl;
			return 0;
		}
	}

	// measure correctness in server response
	int delta = (total_requests * total_cycles) - total_requests;
	for (int i = 0; i < total_requests; i++) {
		if (strcmp(newValues[i + delta], oldValues[i]) != 0) {
			cout << "TEST 1 FAILED: Case " << i << " Expected " <<  newValues[i + delta] << ", Got " << oldValues[i] << endl; 
			return 0;
		}
	}

	// test passed
	cout << "TEST 2 PASSED - MULTPLE UPDATES" << endl;
	return 1;
}

// DURABILITY TEST
int test_correctness_3(KV739Client *client, int total_requests) {
	
		// get process id
	pid_t processID = getpid();

	// generate key/value pairs
	char keys[total_requests][KEY_SIZE] = {0};
	char newValues[total_requests][VALUE_SIZE] = {0};
	char oldValues[total_requests][VALUE_SIZE] = {0};
	for (int i = 0; i < total_requests; i++) {
		sprintf(keys[i], "%d-%d", processID, i % total_requests);
		sprintf(newValues[i], "%x-%x", processID, i / total_requests);
	}

	// send write requests to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_put(keys[i], newValues[i], oldValues[i]) == -1) {
			cout << "Failed to send write request " << i << " in TEST 3" << endl;
			return 0;
		}
	}

	cout << "TEST 3: Please restart the server now, waiting for " << SLEEP_INTERVAL << " seconds..." << endl;
	sleep(SLEEP_INTERVAL);
	cout << "TEST 3: Beginning read requests now" << endl;

	// send read request to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_get(keys[i], oldValues[i]) == -1) {
			cout << "Failed to send read request " << i << " in TEST 3" << endl;
			return 0;
		}
	}

	// measure correctness in server response
	for (int i = 0; i < total_requests; i++) {
		if (strcmp(newValues[i], oldValues[i]) != 0) {
			cout << "TEST 3 FAILED: Case " << i << " Expected " <<  newValues[i] << ", Got " << oldValues[i] << endl; 
			return 0;
		}
	}

	// test passed
	cout << "TEST 3 PASSED - DURABILITY" << endl;
	return 1;	
}

int main(int argc, char *argv[]) {

	int tests_passed = 0;

	// get server name from command line
	char *serverName = argv[1];

	// construct client
	KV739Client *client = new KV739Client();

	// intialize connection to server
	if (client->kv739_init(serverName) == -1) {
		return 0;
	}

	// sanity check for writes and reads
	tests_passed += test_correctness_1(client, 1000);
	cout << "\n";

	// multiple overallaping writes followed by reads
	tests_passed += test_correctness_2(client, 1000, 1);
	cout << "\n";

	// check server failure and data durability
	tests_passed += test_correctness_3(client, 1000);
	cout << "\n";

	// free state and disconnect from server
	client->kv739_shutdown();
	delete client;

	cout << "TESTS PASSED: " << tests_passed << "/" << TOTAL_TESTS << endl;

	return 0;
}