package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"strings"
    "strconv"
	"regexp"
	"bufio"
)

var prep_query string
var prep_query_log string
var prep_update_query_log string
//type MyMap map[string]string

var updateStatement *sql.Stmt
var updateLogStatement *sql.Smt
var updateLogTableStatement *sql.Smt

func init() {
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
}

// InitDB initializes DB at the dbPath
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

	statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS store (key TEXT PRIMARY KEY, value TEXT, uid INT)")
	defer statement.Close()

	_, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== STORE TABLE CREATION FAILED:", err.Error())
		return nil, false
	}
	prep_query = "REPLACE INTO store (key, value, uid) VALUES (?, ?, ?)"
	updateStatement, err = database.Prepare(prep_query)
	if checkErr(err) {
		log.Println("=== UPDATE QUERY PREPATION FAILED:", err.Error())
		return nil, false
	}
	// Create log table
	statement, _ = database.Prepare("CREATE TABLE IF NOT EXISTS log (uid INT PRIMARY KEY, query TEXT)")
	_, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== LOG TABLE CREATION FAILED:", err.Error())
		return nil, false
	}
	prep_query_log = "REPLACE INTO log (uid, query) VALUES (?, ?)"
	updateLogStatement, err = database.Prepare(prep_query_log)
	if checkErr(err) {
		log.Println("=== LOG UPDATE QUERY PREPATION FAILED:", err.Error())
		return nil, false
	}

	//Create update log table
	statement, _ = database.Prepare("CREATE TABLE IF NOT EXISTS update_log (key TEXT PRIMARY KEY, value TEXT, uid INT)")
	_, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== UPDATE LOG TABLE CREATION FAILED:", err.Error())
		return nil, false
	}
	// Clear update log table if failed during another recovery
	statement, _ = database.Prepare("DELETE FROM update_log")
	_, err = statement.Exec()
	if checkErr(err) {
		log.Println("=== UPDATE LOG TABLE TRUNCATE FAILED:", err.Error())
		return nil, false
	}

	prep_update_query_log = "REPLACE INTO update_log (key, value, uid) VALUES (?, ?, ?)"
	updateLogTableStatement, err = database.Prepare(prep_update_query_log)
	if checkErr(err) {
		log.Println("=== UPDATE LOG TABLE REPLACE QUERY PREPATION FAILED:", err.Error())
		return nil, false
	}
	log.Println("=== DB ", dbPath, " SUCCESSFULLY INITIALIZED ===")
	return database, true
}

// UpdateKey updates the `value` for `key` in DB.
// It also stores the uid of last query that modified it.
func UpdateKey(key string, value string, int64 uid, database *sql.DB) bool {

	// Prepare query for log file
	cur_query := strings.Replace(prep_query, "?", fmt.Sprintf("'%v'", val1), 1)
	cur_query = strings.Replace(cur_query, "?", fmt.Sprintf("'%v'", val2), 1)
	cur_query = strings.Replace(cur_query, "?", strconv.FormatInt(val3, 10), 1)

	if server_mode == "ALIVE" {
		_, err := updateStatement.Exec(key, value, uid)

		if checkErr(err) {
			log.Println("=== KEY UPDATE FAILED:", err.Error())
			return false
		}
		
		// Update log table now
		_, err = updateLogStatement.Exec(uid, cur_query)
		if checkErr(err) {
			log.Println("=== LOG UPDATE FAILED:", err.Error())
			return false
		}
	
		return true
	} else {
		// In Zombie mode. So send all updates to update_log 
		// table and don't write it to store or log.
		_,err := updateLogTableStatement.Exec(key, value, uid)
		if checkErr(err) {
			log.Println("=== REPLACE ON UPDATE LOG TABLE FAILED:", err.Error())
			return false
		}
		return true
	} 
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
func FetchLocalUID(path string) int64 {
	// Not from log or update_log
    st,err := database.QueryRow("SELECT MAX(uid) FROM store")
	if err != nil {
		log.Println)"[Recovery] failed during query FetchLocalUID"
	}
    var luid string
    st.Scan(&luid)
    return int64(luid)
}

// Apply a given query if it's latest for the key.
// return false only if failed applying.
func ApplyQuery(query string) bool {
	//fetch the uid of the query,
	tmp := strings.SplitN(query, " ", -1)
	// Get last element and remove '(),' from it.
	var re = regexp.MustCompile(`(^\(|\)|,)`)
    quid := int64(re.ReplaceAllString(tmp[len(tmp)-1],""))
	qval := re.ReplaceAllString(tmp[len(tmp)-2],"")
	qkey := re.ReplaceAllString(tmp[len(tmp)-3],"")
	
	// check with db and see if quid is larger,
	row, err := db.QueryRow("SELECT key FROM store WHERE key=? AND uid>?", qkey, quid)
	if checkErr(err) {
		log.Println("[Recovery] Select query on STORE failed == ApplyQuery")
		return false
	}
	var tmpkey string
	row.Scan(&tmpkey)
	if tmpkey == "" {
		// if either key does not exist or uid < quid, so need to update db and push to log.
		// Just invoke updateKey for now.
		if UpdateKey(qkey, qval, quid, db) {
			return true
		}
		return false
		// ADDON: This could be improved by applying the query directly
	} else {
		// uid > quid means key in db is latest so don't apply query.
		return true
	}
	return false

	//TODO: Add logs to this function
}

// Searched entire log table for uid and returns query
func SearchQueryLog(uid int64) string {
	
	row := db.QueryRow("SELECT query FROM log WHERE uid=?", uid)

	if checkErr(err) {
		log.Println("[Recovery] Select query on LOG failed == SearchQueryLog")
		return ""
	}
	var tmpQuery string
	row.Scan(&tmpQuery)
	if tmpQuery != "" {
		return tmpQuery
	}
	return ""
}


/*
Purely SQL approach: Works only in sqlite though

* create table if not exists tmp (key string PRIMARY KEY, value string, uid INT);
* insert into tmp select key, value, max(uid) 
* from (select * from store union select * from update_tbl) group by key;
drop table store;
* ALTER TABLE tmp RENAME TO store;

 Results might not be guranteed due to functional dependency maybe try
 FULL OUTER JOIN, MERGE or INSERT .. ON CONFLICT


 * With CTEs
 WITH cte1 AS (
         SELECT * FROM A
          UNION
         SELECT * FROM B
     )
   , cte2 AS (
         SELECT t.*, ROW_NUMBER() OVER (PARTITION BY key ORDER BY uid DESC) AS n
           FROM cte1 AS t
     )
SELECT *
  FROM cte2
 WHERE n = 1
;
*/

// Recovery complete so now apply all Update log
// Only apply if uid for a key is greater than one existing.
func ApplyUpdateLog() bool{
	// read entire update log and row by row 
	//rows, err := db.Query("SELECT * FROM update_log")
	return true
}