package persist

import (
    "database/sql"
    "fmt"
	"log"
	"os"

    _ "github.com/mattn/go-sqlite3"
)

type MyMap map[string]string

func init(){
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
}

//"./recover.db"
func CreateTable(dbPath string) (*sql.DB, bool){
	database, err := sql.Open("sqlite3", dbPath)
	if checkErr(err){
		log.Println("=== FAILED TO OPEN DB:", err.Error())
		return nil, false
	}
	statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS store (key TEXT PRIMARY KEY, value TEXT)")
    _, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== TABLE CREATION FAILED:", err.Error())
		return nil, false
	}
	return database, true
}

func InsertKey(key string, value string, database *sql.DB) bool{
	statement, _ := database.Prepare("INSERT INTO store (key, value) VALUES (?, ?)")
    _, err := statement.Exec(key, value)
	if checkErr(err) {
		log.Println("=== KEY INSERTION FAILED:", err.Error())
	}
	return true
}

func GetValue(key string, database *sql.DB) (string, bool){
	rows, err := database.Query("SELECT value FROM store WHERE key=?", key)
	if checkErr(err) {
		log.Println("=== KEY DELETION FAILED:", err.Error())
		return "", false
	}
	var value string
	for rows.Next() {
		rows.Scan(&value)
	}
	return value, true
}

func DeleteKey(key string, database *sql.DB) bool {
	statement, _ := database.Prepare("DELETE FROM store WHERE key=?")
	_, err := statement.Exec(key)
	if checkErr(err) {
		log.Println("=== KEY DELETION FAILED:", err.Error())
		return false
	}
	return true
}

func (table *MyMap) loadKV(dbPath string, database *sql.DB) bool{
	rows, err := 
		database.Query("SELECT key, value FROM store")
	if checkErr(err) {
		log.Println("=== DB LOAD FAILED:", err.Error())
		return false
	}
    var key string
    var value string
    for rows.Next() {
        rows.Scan(&key, &value)
        (*table)[key] = value
		//fmt.Println(key + " : " + value)
    }
	return true
}

// If additional error handling is required.
func checkErr(err error) bool{
	if err != nil {
		return true
		//panic(err)
	}
	return false
}

func main() {
	db, stat := CreateTable("test.db") 
	
	if stat {
		if InsertKey("hello", "world", db) {
			if val, bo := GetValue("hello", db); bo {
				fmt.Println(val)
			}
		}
	}

}