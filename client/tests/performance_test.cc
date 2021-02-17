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

#define TOTAL_TESTS 3
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
int test_performance_simple_writes(KV739Client *client, bool compile_results) {
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
  }

  // Call analysis function
  if (compile_results) {
    latency_results(times, "simple_write");
  }

  return 1;
}

// TESTS AND RUNS ANALYSIS OVER A SINGLE ROUND OF READS
int test_performance_simple_reads(KV739Client *client) {
  // Temporary variable to read value
  char *value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Store time information in vector
  vector<double> times;

  // Send read requests to server
  for (auto key : keys) {
    start = high_resolution_clock::now();
    if (client->kv739_get(&key[0], value) == -1) {
      cout << "Failed to send read request in READ TEST" << endl;
      return 0;
    }
    end = high_resolution_clock::now();
    duration_sec = duration_cast<duration<double, std::milli>>(end - start);
    times.push_back(duration_sec.count());
  }

  // Call analysis function
  latency_results(times, "simple_read");

  return 1;
}

// TESTS AND RUNS ANALYSIS OVER EXPONENTIAL DISTRIBUTION OF READ KEYS
int test_performance_exponential_reads(KV739Client *client, int total_reads) {
  // Temporary variable to read value
  char *value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Store time information in vector
  vector<double> times;
  int counts[keys.size()] = {0};

  std::default_random_engine generator;
  exp_dist distribution(3.5);

  // Send read requests to server
  for (int i = 0; i < total_reads; i++) {
    int index = 0;
    double number = distribution(generator);
    if (number < 1.0) {
      index = round(number * keys.size());
      index = index % keys.size();
      counts[index]++;
    }

    start = high_resolution_clock::now();
    if (client->kv739_get(&keys.at(index)[0], value) == -1) {
      cout << "Failed to send read request " << i << "in READ TEST" << endl;
      return 0;
    }
    end = high_resolution_clock::now();
    duration_sec = duration_cast<duration<double, std::milli>>(end - start);
    times.push_back(duration_sec.count());
  }

  // Call analysis function
  latency_results(times, "exponential_reads");

  return 1;
}

// TESTS AND RUNS ANALYSIS OVER ALTERNATE CYCLES OF WRITES AND READS
int test_performance_alternate_batches(KV739Client *client, int batch_size) {
  // Temporary variable to read value
  char *value = new char[VALUE_SIZE + 1];

  // Time registering variables
  high_resolution_clock::time_point start;
  high_resolution_clock::time_point end;
  duration<double, std::milli> duration_sec;

  // Store time information in vector
  vector<double> write_times;
  vector<double> read_times;

  int start_index = 0, end_index = batch_size;

  while (end_index < keys.size()) {
    end_index = min(end_index, (int)keys.size());

    // Send write requests to server
    for (int i = start_index; i < end_index; i++) {
      start = high_resolution_clock::now();
      if (client->kv739_put(&keys.at(i)[0], &values.at(i)[0], value) == -1) {
        cout << "Failed to send write request " << i << " in TEST 1" << endl;
        return 0;
      }
      end = high_resolution_clock::now();
      duration_sec = duration_cast<duration<double, std::milli>>(end - start);
      write_times.push_back(duration_sec.count());
    }

    // Send read requests to server
    for (int i = start_index; i < end_index; i++) {
      start = high_resolution_clock::now();
      if (client->kv739_get(&keys.at(i)[0], value) == -1) {
        cout << "Failed to send read request in READ TEST" << endl;
        return 0;
      }
      end = high_resolution_clock::now();
      duration_sec = duration_cast<duration<double, std::milli>>(end - start);
      read_times.push_back(duration_sec.count());
    }

    start_index += batch_size;
    end_index += batch_size;
  }

  // Call analysis function
  latency_results(write_times, "batch_writes_" + to_string(batch_size));
  latency_results(read_times, "batch_reads_" + to_string(batch_size));

  return 1;
}

int main(int argc, char *argv[]) {
  int tests_passed = 0;

  // get server name from command line
  char *server_name = argv[1];

  // get the number of keys from the command line
  int num_keys = atoi(argv[2]);

  // get batch size for write-read cycle of third test
  int batch_size = atoi(argv[3]);

  // construct client
  KV739Client *client = new KV739Client();

  // intialize connection to server
  if (client->kv739_init(server_name) == -1) {
    return 0;
  }

  // generate key-value pairs
  generate_keys(num_keys);

  tests_passed += test_performance_simple_writes(client, true);
  tests_passed += test_performance_simple_reads(client);

  string proceed;
  cout << "Performance statistics for simple write and read workloads done. "
          "Clear the database, restart the server and type Y to proceed with "
          "the next test."
       << endl;
  cout << "-----------------------------------------" << endl;
  cout << "Proceed [Y/N]?";
  cin >> proceed;

  if (proceed != "Y") {
    return 0;
  }

  int num_reads = 2 * num_keys;
  test_performance_simple_writes(client, false);
  tests_passed += test_performance_exponential_reads(client, num_reads);

  cout << "Performance statistics for exponential distribution of reads done. "
          "Clear the database, restart the server and type Y to proceed with "
          "the next test."
       << endl;
  cout << "-----------------------------------------" << endl;
  cout << "Proceed [Y/N]?";
  cin >> proceed;

  if (proceed != "Y") {
    return 0;
  }

  tests_passed += test_performance_alternate_batches(client, batch_size);

  // free state and disconnect from server
  client->kv739_shutdown();
  delete client;

  cout << "TESTS PASSED: " << tests_passed << "/" << TOTAL_TESTS << endl;

  return 0;
}