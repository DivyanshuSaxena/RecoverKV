#include <iostream>
#include <unistd.h>
#include <stdio.h>

#include "../client.h"

using namespace std;

#define TOTAL_TESTS 5
#define KEY_SIZE 50
#define VALUE_SIZE 100

#define SLEEP_INTERVAL 30

// SINGLE WRITE CYCLE FOLLOWED BY READS
int test_correctness_single_write_and_read(KV739Client *client, int total_requests) {

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
int test_correctness_write_intensive(KV739Client *client, int total_requests, int total_cycles) {
	
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
			cout << "TEST 2 FAILED: Case " << i << " Expected " <<  newValues[i + delta] << ", Got " << oldValues[i] << endl; 
			return 0;
		}
	}

	// test passed
	cout << "TEST 2 PASSED - MULTPLE WRITES" << endl;
	return 1;
}

// SINGLE WRITE CYCLE FOLLOWED BY MULTIPLE READS
int test_correctness_read_intensive(KV739Client *client, int total_requests, int total_cycles) {
	
	// get process id
	pid_t processID = getpid();

	// generate key/value pairs
	char keys[total_requests][KEY_SIZE] = {0};
	char newValues[total_requests][VALUE_SIZE] = {0};
	char oldValues[total_requests * total_cycles][VALUE_SIZE] = {0};
	for (int i = 0; i < total_requests; i++) {
		sprintf(keys[i], "%d-%d", processID, i);
		sprintf(newValues[i], "%x-%x", processID, i / total_requests);
	}

	// send write requests to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_put(keys[i], newValues[i], oldValues[i]) == -1) {
			cout << "Failed to send write request " << i << " in TEST 3" << endl;
			return 0;
		}
	}

	// send read request to server
	for (int i = 0; i < total_requests * total_cycles; i++) {
		if (client->kv739_get(keys[i % total_requests], oldValues[i]) == -1) {
			cout << "Failed to send read request " << i << " in TEST 3" << endl;
			return 0;
		}
	}

	// measure correctness in server response
	int delta = (total_requests * total_cycles) - total_requests;
	for (int i = 0; i < total_requests; i++) {
		if (strcmp(newValues[i], oldValues[i + delta]) != 0) {
			cout << "TEST 3 FAILED: Case " << i << " Expected " <<  newValues[i] << ", Got " << oldValues[i + delta] << endl; 
			return 0;
		}
	}

	// test passed
	cout << "TEST 3 PASSED - MULTPLE READS" << endl;
	return 1;
}

// DURABILITY TEST
int test_correctness_durability(KV739Client *client, int total_requests) {
	
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
			cout << "Failed to send write request " << i << " in TEST 4" << endl;
			return 0;
		}
	}

	cout << "TEST 4 [Please restart the server now, waiting for " << SLEEP_INTERVAL << " seconds...]" << endl;
	sleep(SLEEP_INTERVAL);
	cout << "TEST 4 [Beginning read requests now]" << endl;

	// send read request to server
	for (int i = 0; i < total_requests; i++) {
		if (client->kv739_get(keys[i], oldValues[i]) == -1) {
			cout << "Failed to send read request " << i << " in TEST 4" << endl;
			return 0;
		}
	}

	// measure correctness in server response
	for (int i = 0; i < total_requests; i++) {
		if (strcmp(newValues[i], oldValues[i]) != 0) {
			cout << "TEST 4 FAILED: Case " << i << " Expected " <<  newValues[i] << ", Got " << oldValues[i] << endl; 
			return 0;
		}
	}

	// test passed
	cout << "TEST 4 PASSED - DURABILITY" << endl;
	return 1;
}

// CHECK FOR INVALID KEY EXISTENCE
int test_correctness_key_existance(KV739Client *client) {

	char key[KEY_SIZE] = "][";
	char oldValue[VALUE_SIZE] = {0};

	if (client->kv739_get(key, oldValue) != 1) {
		cout << "TEST 5 FAILED: " << key << " exists on server" << endl; 
		return 0;
	}

	// test passed
	cout << "TEST 5 PASSED - KEY EXISTENCE" << endl;
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
	tests_passed += test_correctness_single_write_and_read(client, 1000);
	cout << "-----------------------------------------" << endl;

	// multiple overallaping writes followed by reads
	tests_passed += test_correctness_write_intensive(client, 1000, 1);
	cout << "-----------------------------------------" << endl;

	// single writes followed by multiple reads
	tests_passed += test_correctness_read_intensive(client, 1000, 1);
	cout << "-----------------------------------------" << endl;

	// check server failure and data durability
	tests_passed += test_correctness_durability(client, 1000);
	cout << "-----------------------------------------" << endl;

	// check for non-existent keys
	tests_passed += test_correctness_key_existance(client);
	cout << "-----------------------------------------" << endl;

	// free state and disconnect from server
	client->kv739_shutdown();
	delete client;

	cout << "TESTS PASSED: " << tests_passed << "/" << TOTAL_TESTS << endl;

	return 0;
}