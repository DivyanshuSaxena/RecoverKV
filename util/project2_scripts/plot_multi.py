# Script to plot Read/Write parameters versus Number of keys
# Arguments:
# 1: Relative path to where the log files are stored
import sys
import numpy as np
import matplotlib.pyplot as plt

# Update number of clients here
num_clients = [10, 15, 20, 25, 30, 40]
multi = ['multi_reads_', 'multi_writes_']

path = sys.argv[1]

latency_data = {'reads': [], 'writes': []}
thrput_data = {'reads': [], 'writes': []}

# Read the multi reads and writes
for fil in multi:
    for it in range(len(num_clients)):
        key = num_clients[it]
        with open(path + '/' + fil + str(key) + '.log') as f:
            content = f.readlines()
        timings = content[:-2]
        timings = [float(x.strip()) for x in timings]
        summary = content[-2:]

        latency = float(summary[0].split(':')[1].strip()[:-2])
        throughput = float(summary[1].split(':')[1].strip()[:-2])
        latency_data[fil.split('_')[1]].append(latency)
        thrput_data[fil.split('_')[1]].append(throughput)

x = np.arange(len(num_clients))

fig, ax = plt.subplots()
ax.plot(latency_data['reads'], 'b-*', label='Put')
ax.plot(latency_data['writes'], 'g-^', label='Get')

ax.set_ylabel('Latency (in ms)')
ax.set_xlabel('Number of concurrent clients')
ax.set_title('Impact of concurrent connections on Latency')
ax.set_xticks(x)
ax.set_xticklabels(num_clients)
ax.legend()

plt.savefig('multi_latency.png')


fig, ax = plt.subplots()
ax.plot(thrput_data['reads'], 'b-*', label='Put')
ax.plot(thrput_data['writes'], 'g-^', label='Get')

ax.set_ylabel('Throughput (requests per second)')
ax.set_xlabel('Number of concurrent clients')
ax.set_title('Impact of concurrent connections on Total Throughput')
ax.set_xticks(x)
ax.set_xticklabels(num_clients)
ax.legend()

plt.savefig('multi_thrput.png')
