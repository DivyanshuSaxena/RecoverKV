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

	_, err = database.Exec(`
		PRAGMA synchronous = NORMAL;
		PRAGMA journal_mode = WAL;`)
	if err != nil {
		log.Println("=== PARGMA UPDATE FAILED:")
		return nil, false
	}

	statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS store (key TEXT PRIMARY KEY, value TEXT, uid TEXT)")
	defer statement.Close()

	_, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== TABLE CREATION FAILED:", err.Error())
		return nil, false
	}

	updateStatement, err = database.Prepare("REPLACE INTO store (key, value, uid) VALUES (?, ?, ?)")
	if checkErr(err) {
		log.Println("=== UPDATE QUERY PREPATION FAILED:", err.Error())
		return nil, false
	}

	log.Println("=== DB ", dbPath, " SUCCESSFULLY INITIALIZED ===")
	return database, true
}

// UpdateKey updates the `value` for `key` in DB.
// It also stores the uid of last query that modified it.
func UpdateKey(key string, value string, database *sql.DB, uid int64) bool {

	_, err := updateStatement.Exec(key, value, uid)

	if checkErr(err) {
		log.Println("=== KEY UPDATE FAILED:", err.Error())
	}
	//write to own log file
	return true
}

// GetValue returns value for `key` in the DB
func GetValue(key string, database *sql.DB) (string, bool) {
	rows, err := database.Query("SELECT value FROM store WHERE key=?", key)
	if checkErr(err) {
		log.Println("=== KEY FETCH FAILED:", err.Error())
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

// Return the last UID
func FetchLocalUID(path string)


func CurrLogPath() string

// Logs query to a log file.
func logQuery(path string, query string) bool

// Apply a given query if it's latest for the key.
// return false only if failed applying.
func ApplyQuery(path string, ) bool

// Searches the entire log file for uid and returns that line.
func SearchQueryLog(path string, uid int64) string{

}



// TODO: For later
func compactLogs(path string) bool