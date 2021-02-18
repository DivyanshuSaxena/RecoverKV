rm server.log
rm /tmp/test.db
taskset -c 20-29,30-39 ./recoverKV
