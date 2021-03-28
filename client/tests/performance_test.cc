#include <stdio.h>
#include <unistd.h>

#include <chrono>
#include <fstream>
#include <iostream>
#include <random>
#include <vector>

#include "../client.h"

using std::cout;
using std::vector;
using std::chrono::duration;
using std::chrono::duration_cast;
using std::chrono::high_resolution_clock;
using exp_dist = std::exponential_distribution<double>;

#define TOTAL_TESTS 4

#define KEY_SIZE 128
#define VALUE_SIZE 2048

#define SLEEP_INTERVAL 30

// Global variables
vector<std::string> keys;
vector<std::string> values;

// GENERATE KEY-VALUE PAIRS AND STORE IN GLOBAL VECTOR
void generate_keys(int total_keys) {
  srand(time(0));

  // get process id
  pid_t processID = getpid();

  for (int i = 0; i < total_keys; i++) {
    char *key = new char[KEY_SIZE];
    char *value = new char[VALUE_SIZE];
    sprintf(key, "%d-%d-%d", processID, i % total_keys, rand());
    sprintf(value, "%x-%0.2f-%d", processID, (double)i / total_keys, 1);

    keys.push_back(key);
    values.push_back(value);
  }
}

// PARSE TIME ARRAYS AND DISPLAY STATISTICS
void latency_results(vector<double> &time, std::string name) {
  cout << "Results for test: " << name << endl;

  ofstream out_file(name + ".log");
  for (auto t : time) {
    out_file << t << endl;
  }

  // Calculate average latency and total throughput
  double total_time = accumulate(time.begin(), time.end(), 0.0);
  double average = total_time / time.size();
  double throughput = 1000 / average;

  // Display statistics
  cout << "Average Latency: " << average << "ms" << endl;
  out_file << "Average Latency: " << average << "ms" << endl;
  cout << "Throughput: " << throughput << "/s" << endl;
  out_file << "Throughput: " << throughput << "/s" << endl;
  cout << "-----------------------------------------" << endl;

  out_file.close();
}

// TESTS AND RUNS ANALYSIS OVER A SINGLE ROUND OF WRITES
int test_performance_simple_writes(KV739Client *client, bool compile_results,
                                   string name, int kill_key = -1,
                                   char *server_name = NULL) {
  // Temporary variable to read oldvalue
  char *old_value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Store time information in vector
  vector<double> times;

  // Send write requests to server
  for (int i = 0; i < keys.size(); i++) {
    start = high_resolution_clock::now();
    if (client->kv739_put(&keys.at(i)[0], &values.at(i)[0], old_value) == -1) {
      cout << "Failed to send write request " << i << " in TEST 1" << endl;
      return 0;
    }
    end = high_resolution_clock::now();
    duration_sec = duration_cast<duration<double, std::milli>>(end - start);
    times.push_back(duration_sec.count());

    // Kill the server at the requisite iteration
    if (i == kill_key) {
      // Kill a server and then continue the performance tests
      if (client->kv739_die(server_name, 1) == -1) {
        cout << "Failed to send kill request to " << server_name << endl;
        return 0;
      }
    }
  }

  // Call analysis function
  if (compile_results) {
    latency_results(times, name);
  }

  return 1;
}

// TESTS AND RUNS ANALYSIS OVER A SINGLE ROUND OF READS
int test_performance_simple_reads(KV739Client *client, string name,
                                  int kill_key = -1, char *server_name = NULL) {
  // Temporary variable to read value
  char *value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Store time information in vector
  vector<double> times;

  // Send read requests to server
  for (int i = 0; i < keys.size(); i++) {
    auto key = keys.at(i);
    start = high_resolution_clock::now();
    if (client->kv739_get(&key[0], value) == -1) {
      cout << "Failed to send read request in READ TEST at iteration " << i
           << endl;
      return 0;
    }
    end = high_resolution_clock::now();
    duration_sec = duration_cast<duration<double, std::milli>>(end - start);
    times.push_back(duration_sec.count());

    // Kill the server at the requisite iteration
    if (i == kill_key) {
      // Kill a server and then continue the performance tests
      if (client->kv739_die(server_name, 1) == -1) {
        cout << "Failed to send kill request to " << server_name << endl;
        return 0;
      }
    }
  }

  // Call analysis function
  latency_results(times, name);

  return 1;
}

int main(int argc, char *argv[]) {
  int tests_passed = 0;
  int total_servers = 0;

  // if (argc < 4) {
  //   cout << "Format: ./performance_test <num-keys> <ip-addr:port of atleast "
  //           "two servers"
  //        << endl;
  //   return 0;
  // }

  // get the number of keys from the command line
  int num_keys = atoi(argv[1]);
  cout << "Number of keys: " << num_keys << endl;

  // get server names from command line
  char **serverNames = new char *[argc - 1];
  for (int i = 2; i < argc; i++) {
    serverNames[i - 2] = new char[strlen(argv[i])];
    total_servers++;
    strncpy(serverNames[i - 2], argv[i], strlen(argv[i]));
  }
  serverNames[argc - 2] = NULL;
  cout << "Created server name list" << endl;

  // construct client
  KV739Client *client = new KV739Client();

  // intialize connection to server
  if (client->kv739_init(serverNames) == -1) {
    return 0;
  }

  // generate key-value pairs
  generate_keys(num_keys);

  tests_passed += test_performance_simple_writes(
      client, true, "simple_writes_" + to_string(num_keys));
  tests_passed += test_performance_simple_reads(
      client, "simple_reads_" + to_string(num_keys));

  string proceed;
  cout << "Performance statistics for simple write and read workloads done. "
          "Restart the servers, and type Y to proceed with the next test."
       << endl;
  cout << "-----------------------------------------" << endl;
  cout << "Proceed [Y/N]?";
  cin >> proceed;

  if (proceed != "Y") {
    return 0;
  }

  // re-intialize connection to server
  client = new KV739Client();
  if (client->kv739_init(serverNames) == -1) {
    return 0;
  }

  tests_passed += test_performance_simple_writes(
      client, true, "recover_writes_" + to_string(num_keys), num_keys / 4,
      serverNames[0]);
  tests_passed += test_performance_simple_reads(
      client, "recover_reads_" + to_string(num_keys), num_keys / 4,
      serverNames[1]);

  // free state and disconnect from server
  client->kv739_shutdown();

  // cleanup
  for (int i = 0; i < argc - 1; i++) delete serverNames[i];
  delete serverNames;
  delete client;

  cout << "TESTS PASSED: " << tests_passed << "/" << TOTAL_TESTS << endl;

  return 0;
}