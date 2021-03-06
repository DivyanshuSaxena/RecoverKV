package main

import (
	"database/sql"
	"errors"
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
var updateLogStatement *sql.Stmt

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
func UpdateKey(key string, value string, uid int64, database *sql.DB) bool {

	// Prepare query for log file
	cur_query := strings.Replace(prep_query, "?", fmt.Sprintf("'%v'", key), 1)
	cur_query = strings.Replace(cur_query, "?", fmt.Sprintf("'%v'", value), 1)
	cur_query = strings.Replace(cur_query, "?", strconv.FormatInt(uid, 10), 1)

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

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[len(s)-1]; s[0] == c && (c == '"' || c == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// Apply a given query if it's latest for the key.
// return false only if failed applying.
func ApplyQuery(query string) error {
	//fetch the uid of the query,
	tmp := strings.SplitN(query, " ", -1)

	// Get last element and remove '(),' from it.
	var re = regexp.MustCompile(`(^\(|\)|,)`)
	quidStr, _ := strconv.Atoi(re.ReplaceAllString(tmp[len(tmp)-1], ""))
	quid := int64(quidStr)
	qval := re.ReplaceAllString(tmp[len(tmp)-2], "")
	qkey := re.ReplaceAllString(tmp[len(tmp)-3], "")

	qval = trimQuotes(qval)
	qkey = trimQuotes(qkey)

	// check with db and see if quid is larger,
	row := db.QueryRow("SELECT key FROM store WHERE key=? AND uid > ?", qkey, quid)
	var tmpkey string
	err := row.Scan(&tmpkey)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if tmpkey == "" || err == sql.ErrNoRows {

		// if either key does not exist or uid < quid, so need to update db and push to log.
		// Just invoke updateKey for now.
		if UpdateKey(qkey, qval, quid, db) {
			return nil
		}

		return errors.New("ApplyQuery: Failed to update key")
		// ADDON: This could be improved by applying the query directly
	} else {
		// uid > quid means key in db is latest so don't apply query.
		return nil
	}
	return errors.New("ApplyQuery: Failed to update key")
}

func FetchMaxLocalUID() int64 {
	st := db.QueryRow("SELECT MAX(uid) FROM store")
	var luid string
	st.Scan(&luid)
	tmp, _ := strconv.Atoi(luid)
	return int64(tmp)
}

// TODO: Check the holes-finding-SQL query here
func GetHolesInLogTable(global_uid int64) string {

	// if first time, then just return
	if global_uid == 0 {
		return "none"
	}

	query := `
	SELECT a AS id, b AS next_id FROM (
		SELECT a1.uid AS a , MIN(a2.uid) AS b
		FROM store AS a1 LEFT JOIN store AS a2
			ON a2.uid > a1.uid
		GROUP BY a1.uid) AS tab
	WHERE b > a + 1`
	rows, err := db.Query(query)
	if checkErr(err) {
		log.Println("[Recovery] Finding gaps in log table failed")
		return ""
	}

	ret_str := ""
	var least_prs_id string
	var max_prs_id string
	for rows.Next() {
		err = rows.Scan(&least_prs_id, &max_prs_id)
		if err != nil && err != sql.ErrNoRows {
			log.Println("Error in GetHoles")
		}
		if ret_str == "" {
			ret_str += (least_prs_id + "-" + max_prs_id)
		} else {
			ret_str += ("|" + least_prs_id + "-" + max_prs_id)
		}
	}
	// This is to ensure there aren't any trailing holes upto global UID.
	if ret_str == "" {
		return fmt.Sprintf("%d-%d", FetchMaxLocalUID(), global_uid)
	}
	ret_str += fmt.Sprintf("|%d-%d", FetchMaxLocalUID(), global_uid)
	return ret_str
}

// TODO: Check the peer-find-relevant-qids query here
func GetMissingQueriesForPeer(start string, end string) *sql.Rows {
	// be inclusive
	rows, err := db.Query("SELECT query FROM log WHERE uid >=? AND uid <= ?", start, end)
	if checkErr(err) {
		log.Println("[Recovery] Finding gaps in log table failed")
		return nil
	}
	return rows
}
