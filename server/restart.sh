rm server.log
rm /tmp/test.db
taskset -c 20-29,10-19 ./recoverKV
