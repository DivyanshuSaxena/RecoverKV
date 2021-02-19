import numpy as np
import matplotlib.pyplot as plt

with open('results1.csv') as f:
	content = f.readlines()

content = content[1:]
content = [x.strip() for x in content]

simple_data = []
for _ in range(4):
	simple_data.append([])

for i in range(len(content)):
	content_list = content[i].split(',')
	if i % 5 == 1:
		simple_data[0].append(float(content_list[3]))
		simple_data[1].append(float(content_list[4]))
	if i % 5 == 2:
		simple_data[2].append(float(content_list[3]))
		simple_data[3].append(float(content_list[4]))

print(simple_data)

num_keys = ["10k", "50k", "100k", "200k"]
x = np.arange(len(num_keys))

fig, ax = plt.subplots()
ax.bar(x-0.15, simple_data[0], color='b', width=0.3, label='Uniform')
ax.bar(x+0.15, simple_data[2], color='g', width=0.3, label='Exponential')

ax.set_ylabel('Latency (in ms)')
ax.set_xlabel('Number of keys')
ax.set_title('Comparison of workloads')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend(loc='lower right')

plt.show()


fig, ax = plt.subplots()
ax.bar(x-0.15, simple_data[1], color='b', width=0.3, label='Uniform')
ax.bar(x+0.15, simple_data[3], color='g', width=0.3, label='Exponential')

ax.set_ylabel('Throughput (requests per second)')
ax.set_xlabel('Number of keys')
ax.set_title('Comparison of workloads')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend(loc='lower right')

plt.show()