#include <stdio.h>
#include <unistd.h>

#include <iostream>

#include "../client.h"

using namespace std;

#define TOTAL_TESTS 5
#define KEY_SIZE 50
#define VALUE_SIZE 100
#define MAX_PARTITION_TRIES 1000

#define SLEEP_INTERVAL 15

// SINGLE WRITE CYCLE FOLLOWED BY READS
int test_correctness_single_write_and_read(KV739Client *client,
        int total_requests)
{
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d-%d", processID, i % total_requests);
        sprintf(newValues[i], "%x-%x-%d", processID, i / total_requests, 1);
    }

    // send write requests to server
    for (int i = 0; i < total_requests; i++)
    {		
    		int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << " in TEST 1" << endl;
            return 0;
        }
    }

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << " in TEST 1" << endl;
            return 0;
        }
    }

    // measure correctness in server response
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i]) != 0)
        {
            cout << "TEST 1 FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST 1 PASSED - SINGLE UPDATES" << endl;
    return 1;
}

// MULTIPLE WRITE CYCLES FOLLOWED BY READS
int test_correctness_write_intensive(KV739Client *client, int total_requests,
                                     int total_cycles)
{
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests * total_cycles][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests * total_cycles; i++)
    {
        sprintf(keys[i % total_requests], "%d-%d", processID, i % total_requests);
        sprintf(newValues[i], "%x-%x-%d", processID, i / total_requests, 2);
    }

    // send write requests to server
    for (int i = 0; i < total_requests * total_cycles; i++)
    {
        int res = client->kv739_put(keys[i % total_requests], newValues[i], oldValues[i % total_requests]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << " in TEST 2" << endl;
            return 0;
        }
    }

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << " in TEST 2" << endl;
            return 0;
        }
    }

    // measure correctness in server response
    int delta = (total_requests * total_cycles) - total_requests;
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i + delta], oldValues[i]) != 0)
        {
            cout << "TEST 2 FAILED: Case " << i << " Expected "
                 << newValues[i + delta] << ", Got " << oldValues[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST 2 PASSED - MULTPLE WRITES" << endl;
    return 1;
}

// SINGLE WRITE CYCLE FOLLOWED BY MULTIPLE READS
int test_correctness_read_intensive(KV739Client *client, int total_requests,
                                    int total_cycles)
{
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests * total_cycles][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d-%d", processID, i);
        sprintf(newValues[i], "%x-%x-%d", processID, i / total_requests, 3);
    }

    // send write requests to server
    for (int i = 0; i < total_requests; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);

        if (res	== -1 || res == -2)
        {
            cout << "Failed to send write request " << i << " in TEST 3" << endl;
            return 0;
        }
    }

    // send read request to server
    for (int i = 0; i < total_requests * total_cycles; i++)
    {
        if (client->kv739_get(keys[i % total_requests], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << " in TEST 3" << endl;
            return 0;
        }
    }

    // measure correctness in server response
    int delta = (total_requests * total_cycles) - total_requests;
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i + delta]) != 0)
        {
            cout << "TEST 3 FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i + delta] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST 3 PASSED - MULTPLE READS" << endl;
    return 1;
}

// DURABILITY TEST
int test_correctness_durability(KV739Client *client, char *server_name, int total_requests)
{
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d-%d", processID, i % total_requests);
        sprintf(newValues[i], "%x-%x-%d", processID, i / total_requests, 4);
    }

    // send write requests to server
    for (int i = 0; i < total_requests; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);
        if (res	== -1 || res == -2)
        {
            cout << "Failed to send write request " << i << " in TEST 4" << endl;
            return 0;
        }
    }

    cout << "TEST 4 [Killed server " << server_name << " please restart it.] " << endl;
    if (client->kv739_die(server_name, 1) == -1)
    {
        cout << "Failed to send kill request to " << server_name << " in TEST 4" << endl;
        return 0;
    }
    // Ideally, we should not have to wait for any command
    sleep(10);

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << " in TEST 4" << endl;
            return 0;
        }
    }

    // measure correctness in server response
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i]) != 0)
        {
            cout << "TEST 4 FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i] << "for key " << keys[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST 4 PASSED - DURABILITY" << endl;
    return 1;
}

// CHECK FOR INVALID KEY EXISTENCE
int test_correctness_key_existance(KV739Client *client)
{
    char key[KEY_SIZE] = "][";
    char oldValue[VALUE_SIZE] = {0};

    if (client->kv739_get(key, oldValue) != 1)
    {
        cout << "TEST 5 FAILED: " << key << " exists on server" << endl;
        return 0;
    }

    // test passed
    cout << "TEST 5 PASSED - KEY EXISTENCE" << endl;
    return 1;
}

int test_correctness_partition(KV739Client *client, char **serverNames)
{

    pid_t processID = getpid();

    // Parition all servers
    char *reachable[] = {NULL};

    int idx = 0;
    while(serverNames[idx])
    {
        if (client->kv739_partition(serverNames[idx], reachable) == -1)
        {
            cout << "Cannot partition server " << serverNames[idx] << " in TEST 6" << endl;
            return 0;
        }
        idx += 1;
    }

    char key[KEY_SIZE] = {0};
    char newValue[VALUE_SIZE] = {0};
    char oldValue[VALUE_SIZE] = {0};

    // generate key/value pair
    sprintf(key, "%d-%s-%d", processID, "partition", 6);
    sprintf(newValue, "%x-%s-%d", processID, "partition", 6);

    // send write request to all servers
    int res = client->kv739_put(key, newValue, oldValue);
    if (res	== -1 || res == -2)
    {
        cout << "Failed to send write request " << " in TEST 6" << endl;
        return 0;
    }

    // Kill a server (cleanly)
    if (client->kv739_die(serverNames[0], 1) == -1)
    {
        cout << "Failed to send kill request to " << serverNames[idx] << " in TEST 6" << endl;
        return 0;
    }

    // just to make sure it is fully dead
    sleep(2);

    // send write request with same key but different value so that killed server misses update
    sprintf(newValue, "%x-%s-%d", processID, "partitionMiss", 6);
    res = client->kv739_put(key, newValue, oldValue);
    if (res	== -1 || res == -2)
    {
        cout << "Failed to send updated write request " << " in TEST 6" << endl;
        return 0;
    }

    cout << "TEST 6 [Killed server " << serverNames[0] << " please restart it.] " << endl;
    system("pause");

    // measure correctness for that particular missed value
    for (int i = 0; i < MAX_PARTITION_TRIES; i++)
    {

        // send read request to a random server
        if (client->kv739_get(key, oldValue) == -1)
        {
            cout << "Failed to send read request " << i << " in TEST 6" << endl;
            return 0;
        }

        if (strcmp(newValue, oldValue) != 0)
        {
            cout << "TEST 6 PASSED - PARTITION" << endl;
            return 1;
        }
    }

    // test failed
    cout << "TEST 6 FAILED: All servers are updated for key: " << key << endl;
    return 0;
}

int main(int argc, char *argv[])
{
    if (argc == 1)
    {
        cout << "No server names entered" << endl;
        return 0;
    }

    // get server names from command line
    char **serverNames = new char *[argc];
    for(int i = 1; i < argc; i++) {
    	serverNames[i-1] = new char[strlen(argv[i])];
      strncpy(serverNames[i-1], argv[i], strlen(argv[i]));
    }
    serverNames[argc-1] = NULL;

    // construct client
    KV739Client *client = new KV739Client();

    // intialize connection to server
    if (client->kv739_init(serverNames) == -1)
    {
    		cout << "Failed to init client" << endl;
        return 0;
    }

    cout << "Server Init complete. Starting tests now" << endl;

    int tests_passed = 0;

    // sanity check for writes and reads
    // tests_passed += test_correctness_single_write_and_read(client, 1000);
    cout << "-----------------------------------------" << endl;

    // multiple overallaping writes followed by reads
    // tests_passed += test_correctness_write_intensive(client, 1000, 3);
    cout << "-----------------------------------------" << endl;

    // single writes followed by multiple reads
    // tests_passed += test_correctness_read_intensive(client, 1000, 3);
    cout << "-----------------------------------------" << endl;

    // check server failure and data durability
    tests_passed += test_correctness_durability(client, serverNames[0], 10);
    cout << "-----------------------------------------" << endl;

    // check for non-existent keys
    // tests_passed += test_correctness_key_existance(client);
    cout << "-----------------------------------------" << endl;

    // free state and disconnect from server
    if (client->kv739_shutdown() == -1) {
    	cout << "Server shutdown falure" << endl;
    	return 0;
    }

    cout << "TESTS PASSED: " << tests_passed << "/" << TOTAL_TESTS << endl;

    // cleanup
    for (int i = 0; i < argc; i++)
    	delete serverNames[i];
    delete serverNames;

    return 0;
}
