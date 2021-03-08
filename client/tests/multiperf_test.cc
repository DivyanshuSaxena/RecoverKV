#include <stdio.h>
#include <unistd.h>

#include <chrono>
#include <fstream>
#include <iostream>
#include <random>
#include <thread>
#include <vector>

#include "../client.h"

using std::cout;
using std::thread;
using std::vector;
using std::chrono::duration;
using std::chrono::duration_cast;
using std::chrono::high_resolution_clock;
using exp_dist = std::exponential_distribution<double>;

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
    sprintf(key, "%d-%d", processID, i % total_keys);
    sprintf(value, "%x-%0.2f-%d", processID, (double)i / total_keys, 1);

    keys.push_back(key);
    values.push_back(value);
  }
}

// PARSE TIME ARRAYS AND DISPLAY STATISTICS
void latency_results(vector<double> &time, std::string name, double run_time) {
  cout << "Results for test: " << name << endl;

  ofstream out_file(name + ".log");
  for (auto t : time) {
    out_file << t << endl;
  }

  // Calculate average latency and total throughput
  double total_time = accumulate(time.begin(), time.end(), 0.0);
  double average = total_time / time.size();
  double throughput = time.size() * 1000 / run_time;

  // Display statistics
  cout << "Average Latency: " << average << "ms" << endl;
  out_file << "Average Latency: " << average << "ms" << endl;
  cout << "Throughput: " << throughput << "/s" << endl;
  out_file << "Throughput: " << throughput << "/s" << endl;
  cout << "-----------------------------------------" << endl;

  out_file.close();
}

// TESTS AND RUNS ANALYSIS FOR MULTI-THEREADED WRITES
void test_performance_async_writes(KV739Client *client, vector<double> &times,
                                   int t_id, int p_size) {
  // Temporary variable to read oldvalue
  char *old_value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Send write requests to server
  for (int i = t_id; i < keys.size(); i += p_size) {
    start = high_resolution_clock::now();
    if (client->kv739_put(&keys.at(i)[0], &values.at(i)[0], old_value) == -1) {
      cout << "Failed to send write request " << i << " in TEST 1" << endl;
      return;
    }
    end = high_resolution_clock::now();
    duration_sec = duration_cast<duration<double, std::milli>>(end - start);
    times[i] = duration_sec.count();
  }
}

// TESTS AND RUNS ANALYSIS FOR MULTI-THREADED READS
void test_performance_async_reads(KV739Client *client, vector<double> &times,
                                  int t_id, int p_size) {
  // Temporary variable to read value
  char *value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Send read requests to server
  for (int i = t_id; i < keys.size(); i += p_size) {
    start = high_resolution_clock::now();
    if (client->kv739_get(&keys.at(i)[0], value) == -1) {
      cout << "Failed to send read request in READ TEST" << endl;
      return;
    }
    end = high_resolution_clock::now();
    duration_sec = duration_cast<duration<double, std::milli>>(end - start);
    times[i] = duration_sec.count();
  }
}

int main(int argc, char *argv[]) {
  int tests_passed = 0;
  int total_servers = 0;

  if (argc < 4) {
    cout << "Format: ./multiperf_test <num-keys> <pool-size> <ip-addr:port of "
            "atleast two servers"
         << endl;
    return 0;
  }

  // get the number of keys from the command line
  int num_keys = atoi(argv[1]);
  cout << "Number of keys: " << num_keys << endl;

  // get thread pool size for multi-threaded performance test
  int pool_size = atoi(argv[2]);

  // get server names from command line
  char **serverNames = new char *[argc - 1];
  for (int i = 3; i < argc; i++) {
    serverNames[i - 3] = new char[strlen(argv[i])];
    total_servers++;
    strncpy(serverNames[i - 3], argv[i], strlen(argv[i]));
  }
  serverNames[argc - 2] = NULL;
  cout << "Created server name list" << endl;

  // construct clients for each thread
  vector<shared_ptr<KV739Client>> clients;
  for (int i = 0; i < pool_size; i++) {
    shared_ptr<KV739Client> client_ptr(new KV739Client());
    // intialize connection to server
    if (client_ptr->kv739_init(serverNames) == -1) {
      return 0;
    }
    clients.push_back(client_ptr);
  }

  // generate key-value pairs
  generate_keys(num_keys);

  vector<double> write_times(keys.size());
  vector<double> read_times(keys.size());
  vector<thread> t_vec;

  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_writes, duration_reads;

  start = high_resolution_clock::now();
  for (int t = 0; t < pool_size; t++) {
    thread t_obj(test_performance_async_writes, clients.at(t).get(),
                 std::ref(write_times), t, pool_size);
    t_vec.emplace_back(move(t_obj));
  }

  for (int i = 0; i < t_vec.size(); i++) {
    t_vec.at(i).join();
  }
  end = high_resolution_clock::now();
  duration_writes = duration_cast<duration<double, std::milli>>(end - start);

  t_vec.clear();

  start = high_resolution_clock::now();
  for (int t = 0; t < pool_size; t++) {
    thread t_obj(test_performance_async_reads, clients.at(t).get(),
                 std::ref(read_times), t, pool_size);
    t_vec.emplace_back(move(t_obj));
  }

  for (int i = 0; i < t_vec.size(); i++) {
    t_vec.at(i).join();
  }
  end = high_resolution_clock::now();
  duration_reads = duration_cast<duration<double, std::milli>>(end - start);

  latency_results(write_times, "multi_writes_" + to_string(pool_size),
                  duration_writes.count());
  latency_results(read_times, "multi_reads_" + to_string(pool_size),
                  duration_reads.count());

  // free state and disconnect all clients from server
  for (auto client_ptr : clients) {
    client_ptr->kv739_shutdown();
  }

  cout << "TEST COMPLETED" << endl;

  return 0;
}