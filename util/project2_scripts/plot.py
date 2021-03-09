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

total_times = {'reads': [0] * len(num_keys), 'writes': [0] * len(num_keys)}
server_times = {'reads': [0] * len(num_keys), 'writes': [0] * len(num_keys)}

path = sys.argv[1]

data = {'reads': [], 'writes': []}
recover_data = {'reads': [], 'writes': []}

# Read the simple reads and writes
for fil in simple:
    fig, ax = plt.subplots()

    for it in range(len(num_keys)):
        key = num_keys[it]
        with open(path + '/' + fil + str(key) + '.log') as f:
            content = f.readlines()
        timings = content[:-2]
        timings = [float(x.strip()) for x in timings]
        summary = content[-2:]
        total_times[fil.split('_')[1]][it] = sum(timings)

        throughput = float(summary[1].split(':')[1].strip()[:-2])
        data[fil.split('_')[1]].append(throughput)

        vt, bt = np.histogram(timings, bins=100)
        cum_vt = np.cumsum(vt).astype(float)
        cum_vt /= cum_vt[-1]

        ax.plot(bt[:-1], cum_vt, '-.', label=str(key))

    ax.set_title('Latency CDF (' + fil.split("_")[1] + ')')
    ax.set_xlabel('Latency (in ms)')
    ax.set_xscale('log', basex=2)
    ax.set_ylim(0, 1)
    ax.legend()
    plt.savefig('latency_cdf_' + fil.split("_")[1] + '.png')

# Get the time recorded by the server
for it in range(len(num_keys)):
    key = num_keys[it]
    with open(path + '/' + 'lb' + str(key)) as f:
        content = f.readlines()
    content = [x.strip() for x in content]
    total_time = 0
    for timing in content:
        if timing[-2:] == 'ms':
            total_time += float(timing[:-2])
        else:
            total_time += float(timing[:-2])/1000
    server_times['writes'][it] = total_time

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
diff_bars = [(j-i) / j * 100 for i,j in zip(server_times['writes'], total_times['writes'])]
server_bars = [i / j * 100 for i,j in zip(server_times['writes'], total_times['writes'])]
ax.bar(x, server_bars, align='center', width=0.3, label='Time within our system')
ax.bar(x, diff_bars, align='center', width=0.3, bottom=server_bars, label='Total Time registered by Client')

ax.set_ylabel('Fraction')
ax.set_xlabel('Number of keys')
ax.set_title(
    'Fraction of total time taken within server, and Overheads')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.savefig('compare_throughput.png')

fig, ax = plt.subplots()
ax.bar(x - 0.1, data['writes'], color='b', width=0.2, label='Plain Puts')
ax.bar(x + 0.1,
       recover_data['writes'],
       color='g',
       width=0.2,
       label='Recover Puts')

ax.set_ylabel('Throughput')
ax.set_xlabel('Number of keys')
ax.set_title(
    'Throughput of Put operations under normal and failure conditions')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.savefig('put_throughput.png')

fig, ax = plt.subplots()
ax.bar(x - 0.15, data['reads'], color='b', width=0.3, label='Plain Gets')
ax.bar(x + 0.15,
       recover_data['reads'],
       color='g',
       width=0.3,
       label='Recover Gets')

ax.set_ylabel('Throughput')
ax.set_xlabel('Number of keys')
ax.set_title(
    'Throughput of Get operations under normal and failure conditions')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.savefig('get_throughput.png')