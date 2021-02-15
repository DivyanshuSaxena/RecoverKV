package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

//type MyMap map[string]string

func init() {
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
}

// InitDB initializes DB at the dbPath"./recover.db"
func InitDB(dbPath string) (*sql.DB, bool) {
	database, err := sql.Open("sqlite3", dbPath)
	if checkErr(err) {
		log.Println("=== FAILED TO OPEN DB:", err.Error())
		return nil, false
	}
	statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS store (key TEXT PRIMARY KEY, value TEXT)")
	defer statement.Close()

	_, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== TABLE CREATION FAILED:", err.Error())
		return nil, false
	}

	log.Println("=== DB ", dbPath, " SUCCESSFULLY INITIALIZED ===")
	return database, true
}

// UpdateKey updates the `value` for `key` in DB
func UpdateKey(key string, value string, database *sql.DB) bool {
	statement, _ := database.Prepare("REPLACE INTO store (key, value) VALUES (?, ?)")
	defer statement.Close()

	_, err := statement.Exec(key, value)
	if checkErr(err) {
		log.Println("=== KEY INSERTION FAILED:", err.Error())
	}

	return true
}

// GetValue returns value for `key` in the DB
func GetValue(key string, database *sql.DB) (string, bool) {
	rows, err := database.Query("SELECT value FROM store WHERE key=?", key)
	if checkErr(err) {
		log.Println("=== KEY DELETION FAILED:", err.Error())
		return "", false
	}
	defer rows.Close()

	var value string
	for rows.Next() {
		rows.Scan(&value)
	}

	return value, true
}

// LoadKV reads back db from the file when service restarts
func (table *BigMAP) LoadKV(dbPath string, database *sql.DB) bool {
	rows, err :=
		database.Query("SELECT key, value FROM store")
	if checkErr(err) {
		log.Println("=== DB LOAD FAILED:", err.Error())
		return false
	}
	defer rows.Close()

	var key string
	var value string
	for rows.Next() {
		rows.Scan(&key, &value)
		(*table)[key] = value
	}

	return true
}

// If additional error handling is required.
func checkErr(err error) bool {
	if err != nil {
		return true
		//panic(err)
	}
	return false
}
