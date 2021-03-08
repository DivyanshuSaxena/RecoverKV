# Script to plot Read/Write parameters versus Number of keys
# Arguments:
# 1: Relative path to where the log files are stored
import sys
import numpy as np
import matplotlib.pyplot as plt

# Update the number of keys here
num_keys = [10000, 20000, 50000, 100000]
simple = ['simple_reads_', 'simple_writes_']
error = ['recover_reads_', 'recover_writes_']

path = sys.argv[1]

data = {'reads': [], 'writes': []}
recover_data = {'reads': [], 'writes': []}

# Read the simple reads and writes
for fil in simple:
    for it in range(len(num_keys)):
        key = num_keys[it]
        with open(path + '/' + fil + str(key) + '.log') as f:
            content = f.readlines()
        timings = content[:-2]
        timings = [float(x.strip()) for x in timings]
        summary = content[-2:]

        throughput = float(summary[1].split(':')[1].strip()[:-2])
        data[fil.split('_')[1]].append(throughput)

        vt, bt = np.histogram(timings, bins=key)
        cum_vt = np.cumsum(vt).astype(float)
        cum_vt /= cum_vt[-1]

        plt.plot(bt[:-1], cum_vt, label=str(key))
    
    plt.title('Latency CDF (' + fil.split("_")[1] + ')')
    plt.xlabel('Latency (in ms)')
    plt.legend()
    plt.savefig('latency_cdf_' + fil.split("_")[1] + '.png')
    plt.close()

# Get the performance of reads and writes when error occurred
for fil in error:
    for it in range(len(num_keys)):
        key = num_keys[it]
        with open(path + '/' + fil + str(key) + '.log') as f:
            content = f.readlines()
        timings = content[:-2]
        timings = [float(x.strip()) for x in timings]
        summary = content[-2:]

        throughput = float(summary[1].split(':')[1].strip()[:-2])
        recover_data[fil.split('_')[1]].append(throughput)

print(data, recover_data)

# Plot the plain vs recover graph
x = np.arange(len(num_keys))

fig, ax = plt.subplots()
ax.bar(x-0.1, data['writes'], color='b', width=0.2, label='Plain Puts')
ax.bar(x+0.1, recover_data['writes'], color='g', width=0.2, label='Recover Puts')

ax.set_ylabel('Throughput')
ax.set_xlabel('Number of keys')
ax.set_title('Throughput of Put operations under normal and failure conditions')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.savefig('put_throughput.png')

fig, ax = plt.subplots()
ax.bar(x-0.15, data['reads'], color='b', width=0.3, label='Plain Gets')
ax.bar(x+0.15, recover_data['reads'], color='g', width=0.3, label='Recover Gets')

ax.set_ylabel('Throughput')
ax.set_xlabel('Number of keys')
ax.set_title('Throughput of Get operations under normal and failure conditions')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.savefig('get_throughput.png')