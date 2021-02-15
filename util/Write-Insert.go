package main

import (
    "database/sql"
    "fmt"
    "time"
    _ "github.com/mattn/go-sqlite3"
    "os"
)

func main() {
	ops:=10000
	database, _ :=
		sql.Open("sqlite3", "./bogo.db")
	statement, _ :=
		database.Prepare("CREATE TABLE IF NOT EXISTS store (key TEXT PRIMARY KEY, value TEXT)")
    statement.Exec()
    start_sql:= time.Now()
    for i:=0; i<ops; i++ {
	statement, _ =
		database.Prepare("INSERT INTO store (key, value) VALUES (?, ?)")
    statement.Exec("RANDOMVALUE", "ASDASKGHVASDKV")
    }
    duration_sql := time.Since(start_sql)
    fmt.Println("IOPS: ops/sec")
    fmt.Println("SQL insert:", float64(ops)/float64(duration_sql.Seconds()))

   filename := "file.txt"
   f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
   defer f.Close()

   start_file := time.Now()
   for i:=0; i<ops; i++ {
   text:= "INSERT INTO store (key, value) VALUES (RANDOMVALUE, ASDASKGHVASDKV)\n"
   if _, err = f.WriteString(text); err != nil {
   	 panic(err)
   }
   f.Sync()
   }
   duration_file := time.Since(start_file)
   //fmt.Println("File append:",duration_file)
   fmt.Println("File appemdt:", float64(ops)/float64(duration_file.Seconds()))
}

/*
==== Prepare outside for loop and no fsync for file
IOPS: ops/sec
SQL insert: 84416.0415229677
File append: 703207.7807409391

=== Prepare inside for loop and no fsync for file
IOPS: ops/sec
SQL insert: 36916.05464893253
File appemdt: 693832.2338636478

== Prepare outside for loop and with fsync after each write
IOPS: ops/sec
SQL insert: 82598.394160016
File append: 9116.557306587609

== Prepare outside for loop and with fsync after all writes
IOPS: ops/sec
SQL insert: 73898.50764180772
File append: 386102.2942507206
*/
