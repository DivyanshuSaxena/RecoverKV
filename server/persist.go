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
/*
* log path changes every time server restarts,
* so to keep it unique let's use UNIX timestamp.
*/
log_path := "./log/"+strconv.Itoa(int(time.Now().Unix()))
var flog *os.File

//type MyMap map[string]string

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
		log.Println("=== TABLE CREATION FAILED:", err.Error())
		return nil, false
	}
	prep_query = "REPLACE INTO store (key, value, uid) VALUES (?, ?, ?)"
	updateStatement, err = database.Prepare(prep_query)
	if checkErr(err) {
		log.Println("=== UPDATE QUERY PREPATION FAILED:", err.Error())
		return nil, false
	}
	log.Println("=== DB ", dbPath, " SUCCESSFULLY INITIALIZED ===")

	flog, err := os.OpenFile(log_path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if checkErr(err) {
		log.Println("=== LOG file creation failed")
		return nil, false
	}
	return database, true
}

// UpdateKey updates the `value` for `key` in DB.
// It also stores the uid of last query that modified it.
func UpdateKey(key string, value string, int64 uid, database *sql.DB) bool {

	_, err := updateStatement.Exec(key, value, uid)

	if checkErr(err) {
		log.Println("=== KEY UPDATE FAILED:", err.Error())
		return false
	}

	// Prepare query for log file
    cur_query := strings.Replace(prep_query, "?", fmt.Sprintf("'%v'", val1), 1)
    cur_query = strings.Replace(cur_query, "?", fmt.Sprintf("'%v'", val2), 1)
    cur_query = strings.Replace(cur_query, "?", strconv.FormatInt(val3, 10), 1)
	
	// TODO: Make this a routine? Might affect correctness.
	if !logQuery(cur_query) {
		log.Println("=== LOG UPDATE FAILED for Key", key)
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
    st,err := database.QueryRow("SELECT MAX(uid) FROM store")
	if err != nil {
		log.Println)"[Recovery] failed during query FetchLocalUID"
	}
    var luid string
    st.Scan(&luid)
    return int64(luid)
}

// Logs query to a log file. Each query will be on a single line.
func logQuery(query string) bool {
	if _, err = flog.WriteString(query+"\n"); err != nil {
		return false
  }
  return true
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
	row := db.QueryRow("SELECT key FROM store WHERE key=? AND uid>?", qkey, quid)
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

// Searches uid for a given file and returns that line
func searchFile(uid int64, filepath string) string{
	var squery string
	f, err := os.Open(filepath)
	if err != nil {
		log.Println("[Recovery] Failed to open file during searchFile"+filepath+":"+err)
		return ""
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), strconv.Itoa(uid)) {
			squery = scanner.Text()
			break
		}
	}

	if err := scanner.Err(); err != nil {
		// Handle the error
		log.Println("[Recovery] Error in scanner searchFile"+filepath+":"+err)
		return ""
	}
	return ""
}


// Searches the entire log file for uid and returns that line.
// How many log files should this search, in what order
func SearchQueryLog(uid int64) string{
	// It'll begin by searching the current log.
	dir := "./log/"
	files, err := ioutil.ReadDir(dir)
	// A list of recently modified files in log dir
	sort.Slice(files, func(i,j int) bool{
		return files[i].ModTime().Unix() < files[j].ModTime().Unix()
	})
	// Now iterate over each and check for uid
	for _, file := range files {
		filepath := "./log/"+file.Name()
		query = searchFile(filepath)
		if query != "" {
			return query
			break
		}
	}
	return ""
	/**
	* ADDON: Multiple threads can divide the recency file list and 
	*		search in parallel to find the uid soon.
	*/
}



// All data is written twice, so compaction is important.
// TODO: One fine day
func compactLogs(path string) bool