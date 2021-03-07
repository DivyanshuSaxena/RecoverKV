#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>

#include <iostream>

#include "../client.h"

using namespace std;

#define SLEEP_INTERVAL 5

#define KEY_SIZE 50
#define VALUE_SIZE 100
#define MAX_PARTITION_TRIES 1000

int test_num = 0;
int total_servers = 0;

// SINGLE WRITE CYCLE FOLLOWED BY READS
int test_correctness_single_write_and_read(KV739Client *client,
        int total_requests)
{

    test_num++;
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d%d", processID, i % total_requests);
        sprintf(newValues[i], "%x%x%d", processID, i / total_requests, 1);
    }

    // send write requests to server
    for (int i = 0; i < total_requests; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // measure correctness in server response
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i]) != 0)
        {
            cout << "TEST " << test_num << " FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - SINGLE UPDATES" << endl;
    return 1;
}

// MULTIPLE WRITE CYCLES FOLLOWED BY READS
int test_correctness_write_intensive(KV739Client *client, int total_requests,
                                     int total_cycles)
{
    test_num++;
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests * total_cycles][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests * total_cycles; i++)
    {
        sprintf(keys[i % total_requests], "%d%d", processID, i % total_requests);
        sprintf(newValues[i], "%x%x%d", processID, i / total_requests, 2);
    }

    // send write requests to server
    for (int i = 0; i < total_requests * total_cycles; i++)
    {
        int res = client->kv739_put(keys[i % total_requests], newValues[i], oldValues[i % total_requests]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // measure correctness in server response
    int delta = (total_requests * total_cycles) - total_requests;
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i + delta], oldValues[i]) != 0)
        {
            cout << "TEST " << test_num << " FAILED: Case " << i << " Expected "
                 << newValues[i + delta] << ", Got " << oldValues[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - MULTPLE WRITES" << endl;
    return 1;
}

// SINGLE WRITE CYCLE FOLLOWED BY MULTIPLE READS
int test_correctness_read_intensive(KV739Client *client, int total_requests,
                                    int total_cycles)
{
    test_num++;
    // get process id
    pid_t processID = getpid();

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests * total_cycles][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d%d", processID, i);
        sprintf(newValues[i], "%x%x%d", processID, i / total_requests, 3);
    }

    // send write requests to server
    for (int i = 0; i < total_requests; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);

        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // send read request to server
    for (int i = 0; i < total_requests * total_cycles; i++)
    {
        if (client->kv739_get(keys[i % total_requests], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // measure correctness in server response
    int delta = (total_requests * total_cycles) - total_requests;
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i + delta]) != 0)
        {
            cout << "TEST " << test_num << " FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i + delta] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - MULTPLE READS" << endl;
    return 1;
}

// DURABILITY TEST
int test_correctness_durability(KV739Client *client, char **serverNames, int total_requests)
{
    test_num++;

    // get process id
    pid_t processID = getpid();

    // server to kill
    char *server_name = serverNames[rand() % total_servers];

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d%d", processID, i % total_requests);
        sprintf(newValues[i], "%x%x%d", processID, i / total_requests, 4);
    }

    // send write requests to server
    for (int i = 0; i < total_requests; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    cout << "TEST " << test_num << " Killed server " << server_name  << endl;
    if (client->kv739_die(server_name, 1) == -1)
    {
        cout << "Failed to send kill request to " << server_name << " in TEST " << test_num << endl;
        return 0;
    }

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // measure correctness in server response
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i]) != 0)
        {
            cout << "TEST " << test_num << " FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i] << " for key " << keys[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - DURABILITY" << endl;
    return 1;
}

// RECOVERY TEST
int test_correctness_recovery(KV739Client *client, char **serverNames, int total_requests, int clean)
{
    test_num++;

    // get process id
    pid_t processID = getpid();

    // server to kill
    char *server_name = serverNames[rand() % total_servers];

    // generate key/value pairs
    char keys[total_requests][KEY_SIZE] = {0};
    char newValues[total_requests][VALUE_SIZE] = {0};
    char oldValues[total_requests][VALUE_SIZE] = {0};
    for (int i = 0; i < total_requests; i++)
    {
        sprintf(keys[i], "%d%d", processID, i % total_requests);
        sprintf(newValues[i], "%d", i);
    }

    // send write requests to server
    for (int i = 0; i < total_requests / 4; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    cout << "TEST " << test_num << " Killed server " << server_name << endl;
    if (client->kv739_die(server_name, clean) == -1)
    {
        cout << "Failed to send kill request to " << server_name << " in TEST " << test_num << endl;
        return 0;
    }

    // send write requests to server -- again
    for (int i = total_requests / 4; i < total_requests; i++)
    {
        int res = client->kv739_put(keys[i], newValues[i], oldValues[i]);
        if (res == -1 || res == -2)
        {
            cout << "Failed to send write request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // Ideally, we should not have to wait for any command
    cout << "Waiting for " << SLEEP_INTERVAL << " secs server to recover" << endl;
    sleep(SLEEP_INTERVAL);

    // send read request to server
    for (int i = 0; i < total_requests; i++)
    {
        if (client->kv739_get(keys[i], oldValues[i]) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }
    }

    // measure correctness in server response
    for (int i = 0; i < total_requests; i++)
    {
        if (strcmp(newValues[i], oldValues[i]) != 0)
        {
            cout << "TEST " << test_num << " FAILED: Case " << i << " Expected " << newValues[i]
                 << ", Got " << oldValues[i] << " for key " << keys[i] << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - RECOVERY WITH CLEAN: " << clean << endl;
    return 1;
}

// CHECK FOR INVALID KEY EXISTENCE
int test_correctness_key_existance(KV739Client *client)
{
    test_num++;
    char key[KEY_SIZE] = "][";
    char oldValue[VALUE_SIZE] = {0};

    if (client->kv739_get(key, oldValue) != 1)
    {
        cout << "TEST " << test_num << " FAILED: " << key << " exists on server" << endl;
        return 0;
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - KEY EXISTENCE" << endl;
    return 1;
}

int test_correctness_partition(KV739Client *client, char **serverNames)
{
    test_num++;
    pid_t processID = getpid();

    // Parition all servers
    char *reachable[] = {NULL};

    int idx = 0;
    while(serverNames[idx])
    {
        if (client->kv739_partition(serverNames[idx], reachable) == -1)
        {
            cout << "Cannot partition server " << serverNames[idx] << " in TEST " << test_num << endl;
            return 0;
        }
        idx += 1;
    }

    char key[KEY_SIZE] = {0};
    char newValue[VALUE_SIZE] = {0};
    char oldValue[VALUE_SIZE] = {0};

    // generate key/value pair
    sprintf(key, "%d%s%d", processID, "partition", 6);
    sprintf(newValue, "%x%s%d", processID, "partition", 6);

    // send write request to all servers
    int res = client->kv739_put(key, newValue, oldValue);
    if (res == -1 || res == -2)
    {
        cout << "Failed to send write request " << " in TEST " << test_num << endl;
        return 0;
    }

    // Kill a server (cleanly)
    cout << "TEST " << test_num << " Killed server " << serverNames[0] << endl;
    if (client->kv739_die(serverNames[0], 1) == -1)
    {
        cout << "Failed to send kill request to " << serverNames[0] << " in TEST " << test_num << endl;
        return 0;
    }

    // send write request with same key but different value so that killed server misses update
    sprintf(newValue, "%x%s%d", processID, "partitionMiss", 6);
    res = client->kv739_put(key, newValue, oldValue);
    if (res == -1 || res == -2)
    {
        cout << "Failed to send updated write request " << " in TEST " << test_num << endl;
        return 0;
    }

    // Wait for the server to recover.
    cout << "Waiting for " << SLEEP_INTERVAL << " secs server to recover" << endl;
    sleep(SLEEP_INTERVAL);

    // measure correctness for that particular missed value
    for (int i = 0; i < MAX_PARTITION_TRIES; i++)
    {

        // send read request to a random server
        if (client->kv739_get(key, oldValue) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }

        if (strcmp(newValue, oldValue) != 0)
        {
            cout << "TEST " << test_num << " PASSED - PARTITION" << endl;
            return 1;
        }
    }

    // test failed
    cout << "TEST " << test_num << " FAILED: All servers are updated for key: " << key << endl;
    return 0;
}

int test_correctness_partition_healing(KV739Client *client, char **serverNames)
{
    test_num++;
    pid_t processID = getpid();

    int idx = 0;
    while(serverNames[idx])
    {
        // Parition all servers
        char *reachable[] = {serverNames[(idx + 1) % total_servers], serverNames[(idx + 2) % total_servers], NULL};
        if (client->kv739_partition(serverNames[idx], reachable) == -1)
        {
            cout << "Cannot heal partition for server " << serverNames[idx] << " in TEST " << test_num << endl;
            return 0;
        }
        idx += 1;
    }

    // Wait for the servers to recover.
    cout << "Waiting for " << SLEEP_INTERVAL << " secs server to recover" << endl;
    sleep(SLEEP_INTERVAL);

    char key[KEY_SIZE] = {0};
    char newValue[VALUE_SIZE] = {0};
    char oldValue[VALUE_SIZE] = {0};

    // generate key/value pair. Use the key which one server does not have
    sprintf(key, "%d%s%d", processID, "partition", 6);
    sprintf(newValue, "%x%s%d", processID, "partitionMiss", 6);

    // measure correctness for that particular missed value
    for (int i = 0; i < MAX_PARTITION_TRIES; i++)
    {

        // send read request to a random server
        if (client->kv739_get(key, oldValue) == -1)
        {
            cout << "Failed to send read request " << i << "in TEST " << test_num << endl;
            return 0;
        }

        // all reads should match
        if (strcmp(newValue, oldValue) != 0)
        {
            cout << "TEST " << test_num << " FAILED: Case " << i << " Expected " << newValue
                 << ", Got " << oldValue << " for key " << key << endl;
            return 0;
        }
    }

    // test passed
    cout << "TEST " << test_num << " PASSED - PARTITION HEALING" << endl;
    return 1;
}

int test_correctness_client_permissions(KV739Client *client, int clean)
{
    test_num++;

    // Lets try to kill a random IP:port
    char server_name[] = "localhost:8989";

    if (client->kv739_die(server_name, clean) == -1)
    {
        cout << "TEST " << test_num << " PASSED - CLIENT PERMISSIONS" << endl;
        return 1;
    }

    // test failed
    cout << "TEST " << test_num << " FAILED: Interacted with a server we did not init for" << endl;
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
    for(int i = 1; i < argc; i++)
    {
        serverNames[i - 1] = new char[strlen(argv[i])];
        total_servers++;
        strncpy(serverNames[i - 1], argv[i], strlen(argv[i]));
    }
    serverNames[argc - 1] = NULL;

    // construct client
    KV739Client *client = new KV739Client();

    // intialize connection to server
    if (client->kv739_init(serverNames) == -1)
    {
        cout << "Failed to init client" << endl;
        return 0;
    }

    cout << "Total Servers to init: " << total_servers << "\nStarting tests now..." << endl;

    int tests_passed = 0;

    // sanity check for writes and reads
    tests_passed += test_correctness_single_write_and_read(client, 1000);
    cout << "-----------------------------------------" << endl;

    // multiple overallaping writes followed by reads
    tests_passed += test_correctness_write_intensive(client, 1000, 3);
    cout << "-----------------------------------------" << endl;

    // single writes followed by multiple reads
    tests_passed += test_correctness_read_intensive(client, 1000, 3);
    cout << "-----------------------------------------" << endl;

    // check server failure and data durability
    tests_passed += test_correctness_durability(client, serverNames, 1000);
    cout << "-----------------------------------------" << endl;

    // check server failure and data durability -- clean stop
    tests_passed += test_correctness_recovery(client, serverNames, 1000, 1);
    cout << "-----------------------------------------" << endl;

    // check server failure and data durability -- unclean stop
    tests_passed += test_correctness_recovery(client, serverNames, 1000, 0);
    cout << "-----------------------------------------" << endl;

    // check for non-existent keys
    tests_passed += test_correctness_key_existance(client);
    cout << "-----------------------------------------" << endl;

    // check if no recovery during partition
    tests_passed += test_correctness_partition(client, serverNames);
    cout << "-----------------------------------------" << endl;

    // assumes that there is complete partition (only run after test_correctness_partition)
    tests_passed += test_correctness_partition_healing(client, serverNames);
    cout << "-----------------------------------------" << endl;

    // check for client permissions (cannot reach server to which we did not init)
    tests_passed += test_correctness_client_permissions(client, 1);
    cout << "-----------------------------------------" << endl;

    // free state and disconnect from server
    if (client->kv739_shutdown() == -1)
    {
        cout << "Server shutdown falure" << endl;
        return 0;
    }

    cout << "TESTS PASSED: " << tests_passed << "/" << test_num << endl;

    // cleanup
    for (int i = 0; i < argc; i++)
        delete serverNames[i];
    delete serverNames;
    delete client;

    return 0;
}
