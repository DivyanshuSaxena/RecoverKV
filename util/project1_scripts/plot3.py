import numpy as np
import matplotlib.pyplot as plt

with open('results3.csv') as f:
	content = f.readlines()

content = content[1:]
content = [x.strip() for x in content]

simple_data = []
for _ in range(4):
	simple_data.append([])

for i in range(len(content)):
	content_list = content[i].split(',')
	if i % 2 == 0:
		simple_data[0].append(float(content_list[2]))
		simple_data[1].append(float(content_list[3]))
	if i % 2 == 1:
		simple_data[2].append(float(content_list[2]))
		simple_data[3].append(float(content_list[3]))

print(simple_data)

num_keys = [5, 10, 15, 20, 25, 30]
x = np.arange(len(num_keys))

fig, ax = plt.subplots()
ax.plot(simple_data[0], 'b-*', label='Put')
ax.plot(simple_data[2], 'g-^', label='Get')

ax.set_ylabel('Latency (in ms)')
ax.set_xlabel('Number of concurrent clients')
ax.set_title('Impact of concurrent connections on Latency')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.show()


fig, ax = plt.subplots()
ax.plot(simple_data[1], 'b-*', label='Put')
ax.plot(simple_data[3], 'g-^', label='Get')

ax.set_ylabel('Throughput (requests per second)')
ax.set_xlabel('Number of concurrent clients')
ax.set_title('Impact of concurrent connections on Total Throughput')
ax.set_xticks(x)
ax.set_xticklabels(num_keys)
ax.legend()

plt.show()
