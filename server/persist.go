package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"regexp"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var prep_query string
var prep_query_log string

//type MyMap map[string]string

var updateStatement *sql.Stmt
var updateLogStatement *sql.Smt

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
		_, err := updateLogTableStatement.Exec(key, value, uid)
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
		tableMu.Lock()
		(*table)[key] = value
		tableMu.Unlock()
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

// Apply a given query if it's latest for the key.
// return false only if failed applying.
func ApplyQuery(query string) bool {
	//fetch the uid of the query,
	tmp := strings.SplitN(query, " ", -1)
	// Get last element and remove '(),' from it.
	var re = regexp.MustCompile(`(^\(|\)|,)`)
	quid := int64(re.ReplaceAllString(tmp[len(tmp)-1], ""))
	qval := re.ReplaceAllString(tmp[len(tmp)-2], "")
	qkey := re.ReplaceAllString(tmp[len(tmp)-3], "")

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

func FetchMaxLocalUID(path string) int64 {
	st,err := database.QueryRow("SELECT MAX(uid) FROM store")
	if err != nil {
		log.Println)"[Recovery] failed during query FetchLocalUID"
	}
	var luid string
	st.Scan(&luid)
	return int64(luid)
}

// TODO: Check the holes-finding-SQL query here
func GetHolesInLogTable(global_uid int64) string {
	query := `
	SELECT a AS id, b AS next_id FROM (
		SELECT a1.uid AS a , MIN(a2.uid) AS b
		FROM store AS a1 LEFT JOIN store AS a2
			ON a2.uid > a1.uid
		GROUP BY a1.uid) AS tab
	WHERE b > a + 1`
	rows, err := db.Query(query)
	if checkErr(err) {
		log.Println("[Recovery] Finding gas in log table failed")
		return status
	}

	ret_str := ""
	var least_prs_id string
	var max_prs_id string
	for rows.Next() {
		rows.Scan(&least_prs_id, &max_prs_id)
		if ret_str == "" {
			ret_str += (least_prs_id + "-" + max_prs_id)
		} else {
			ret_str += ("|" + least_prs_id + "-" + max_prs_id)
		}
	}
	// This is to ensure there aren't any trailing holes upto global UID.
	append(ret_str, fmt.Sprintf("|%d-%d", FetchMaxLocalUID(), global_uid))
	return ret_str
}

// TODO: Check the peer-find-relevant-qids query here
func GetMissingQueriesForPeer(start string, end string) *Rows {
	rows, err := db.QueryRow("SELECT query FROM log WHERE uid>? AND uid<?", start, end)
	if checkErr(err) {
		log.Println("[Recovery] Finding gaps in log table failed")
		return nil
	}
	return rows
}
