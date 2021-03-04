package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
	"net"
	"os"
	"sync"
	"math/rand"

	pb "recoverKV/gen/recoverKV"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
	"strconv"
	"strings"
)

var ip_addr string
var serv_port string
var rec_port string
var server_id string
var lb_ip_addr string
var lb_port string

rand.Seed(time.Now().Unix()) 
var rec_cmplt bool
var rec_mu sync.Mutex
rec_cond :=sync.NewCond(&rec_mu)
db_path := "./data/recover.db"
// Refer persist.go for log path

// setter and getter for the mode can be helpful for LB
server_mode := "DEAD" // 3 modes, DEAD->ZOMBIE->ALIVE

// BigMAP is the type for the (key-value) pairs table
type BigMAP map[string]string

var (
	table   = make(BigMAP)
	tableMu = sync.Mutex{}
)
var db *sql.DB

// server is used to implement RecoverKV service.
type server struct {
	pb.UnimplementedRecoverKVServer
}

// GetValue implements RecoverKV.GetValue
// Returns (val, 0) if the key is found
// 				 ("", 1) if the key is absent
func (s *server) GetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	key := in.GetKey()
	//log.Printf("GetValue Received: %v\n", key)
	var successCode int32 = 0

	tableMu.Lock()
	val, prs := table[key]
	tableMu.Unlock()

	if !prs {
		successCode = 1
	}

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

// SetValue implements RecoverKV.SetValue
// Returns (old_value, 0) if key present
// 				 (new_value, 1) if key absent
func (s *server) SetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	key := in.GetKey()
	newVal := in.GetValue()
	//TODO: Verify
	uid := in.GetObjectID()
	//log.Printf("SetValue Received: %v:%v\n", key, newVal)
	var successCode int32 = 0

	tableMu.Lock()
	val, prs := table[key]
	table[key] = newVal
	tableMu.Unlock()

	if !prs {
		val = newVal
		successCode = 1
	}

	// TODO: check if bellow true of false before returning.
	UpdateKey(key, newVal, uid, db)
	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

func (s *server) stopServer(ctx context.Context, in *emptypb.Empty) (*pb.Response, error) {

	go func (){
		// wait for 3 seconds
		time.Sleep(3)
		// then exit the parent process
		os.Exit(0)
	}()

	return &pb.Response{Value: "", SuccessCode: 0}, nil
}

// PrintStartMsg prints the start message for the server
func PrintStartMsg() {
	name := `
	___                        _  ____   __
	| _ \___ __ _____ _____ _ _| |/ /\ \ / /
	|   / -_) _/ _ \ V / -_) '_| ' <  \ V / 
	|_|_\___\__\___/\_/\___|_| |_|\_\  \_/ `

	fmt.Println(string("\033[36m"), name)
	fmt.Println()
	fmt.Println("Server started successfully,\n" + 
				"Server id:          \t"+ server_id+
				"\nServe address:    \t"+ ip_addr+":"+serv_port+
				"\nRecovery address: \t"+ ip_addr+":"+rec_port)
}

// Recovery request handler 
func (s server) FetchQueries(in *pb.RecRequest, srv pb.StreamService_FetchResponseServer) error {
	fmt.Println("[Recovery] Responding to server "+in.GetAddress())
	log.Println("[Recovery] Responding to server "+in.GetAddress())

	ferr := 0
	// Parse the received missing UIDs from the recovering node
	missingList := strings.Split(in.GetMissingUIDs(), "|")
	for _, missingRange := range missingList {
		// Extract the individual ranges
		rangeBound := strings.Split(in.GetMissingUIDs(), "-")
		// Can send multiple queries to the db here. But rather sending a single one.
		rows := GetMissingQueriesForPeer(rangeBound[0], rangeBound[1])
		var tmpQuery string
		for rows.Next() {
			rows.Scan(&tmpQuery)
			if tmpQuery == "" {
				ferr = 1
			}
			resp := pb.RecResponse{Query: tmpQuery, FoundError: ferr}
			if err := srv.Send(&resp); err != nil {
				log.Printf("[Recovery] Send error %v", err)
			}
		}
	}

	///////////////////////////////////////////////////////////////////////////
	// TODO: Remove this entire block, once confirmed that it isn't needed
	// From the given id search for it in the log table.
	// ADDON: This could be further improved by having multiple threads divide the search requests
	// 		 by each invoking searchQueryLog and respond back as and when they find the query. 
	// Making this ascending order so that on caller crash it will not be left with holes.
	for qid := in.FromId-in.QLength; qid < in.FromId ; qid++ {
		que := SearchQueryLog(qid)
		if que == "" {
			// if globalID is not seen in log sleep for 3 seconds and try again
			time.Sleep(3 * time.Second)	
			// hopefully by now log table has the qid we are interested in.
			que = SearchQueryLog(qid)
			// if still nil return client error
			if que == nil {
				que = ""
				ferr = 1
			}
		}
		resp := pb.RecResponse{Query: que, FoundError: ferr}
		if err := srv.Send(&resp); err != nil {
			log.Printf("[Recovery] Send error %v", err)
		}
	}
	///////////////////////////////////////////////////////////////////////////


	return nil
}

func rpcRequestLogs(peer_addr string, count int, from int) bool{
	// send query to DB
	str := GetHolesInLogTable()

	// dial peer
	conn, err := grpc.Dial(peer_addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("[Recovery] Failed to connect with peer %v : %v",peer_addr, err)
		return false
	}

	// creating stream
	client := pb.NewStreamServiceClient(conn)
	in := &pb.RecRequest{MissingUIDs: str, Address: peer_addr}
	stream, err := client.FetchQueries(context.Background(), in)
	if err != nil {
		log.Fatalf("[Recovery] Open stream error for peer %v : %v", peer_addr, err)
		return false
	}
	done := make(chan bool)
	// ADDON: This could be extended to multiple threads processing queries in parallel.
	//		  Since order of replay anyway doesn't matter.
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				done <- true //means stream is finished
				return
			}
			if err != nil {
				log.Fatalf("[Recovery] Cannot receive %v", err)
				return
			}
			if resp.FoundError == 1 {
				log.Fatal("[Recovery] Buddy server has an error.")
				return
			}
			if ApplyQuery(resp.Query) {
				log.Fatalf("[Recovery] failed to apply query %x : ABORTING recovery.", resp.Query)
			}
			fmt.Printf("[Recovery] Replayed query %s -- ", resp.Query)
		}
	}()

	if <-done {
		log.Println("[Recovery] All queries are replayed now.")
		return true
	} else {
		return false
	}

}

/*
************************************************************
Recovery stage:
************************************************************
1. Don't serve GETs yet! PUTs can be served. 
	* However, PUTs go to a different table. update_log
2. First check with LB for latest global uid
3. subtract cur_uid - latest_global_uid to get new set of queries
4. Next broadcast request to a random server for new queries
	* Not broadcasting to all to reduce wasting cycles.
5. Receive and apply each query if uid of key X is > current uid of key X
	* uid is sort of our logical clock.
6. Recovery stage is almost done once all queries from healer is applied.
	* Next apply queries from update_log if uid > current uid of key in data_table
	* In parallel set server to ALIVE so new update requests go to data_table
	* Once above is done, now invoke LoadKV to load db to memory.
	* Finally, to enable GETs report to LB that you're ALIVE.
* Time this and report..
*/
func recoveryStage(local_latest_uid int64) {
	// Combine step 1 & 2 in a single func in LB. 
	global_uid := LB.markMeZombie()

	//Step 3. // Note not error checking FetchLocalUID
	lagging := global_uid - local_latest_uid
	if lagging == 0 {
		// nothing to recover
		rec_mu.Lock()
		rec_cmplt = true
		rec_mu.Unlock()
		rec_cond.broadcast()
		return
	}

	// Step 4.
	// fetch peer list from LB, it should also ensure 
	// these peers are not in recovery themselves.
	// This is of the form peer_ip:port
	peers := LB.fetchALIVEpeers()
	// ADDON: Bellow can be extended to make multiple requests from different peers
	// Track percentage completion of each recovery and kill other routines after
	// a threshold.
	
	// Step 5.
	if !rpcRequestLogs(peers[rand.Intn(len(peers))], lagging, global_uid) {
		rec_mu.Lock()
		rec_cmplt = false
		rec_mu.Unlock()
		rec_cond.broadcast()
		return
	} else {
		// Done processing all queries, so mark completion
		rec_mu.Lock()
		rec_cmplt = true
		rec_mu.Unlock()
		rec_cond.broadcast()
		return
	}
}

// Takes 6 arguments
// unique id of the server. ip address, serve port, recovery port, LB ip addr, LB port
func main() {
	//fmt.Println("Starting server execution")

	//parse arguments -- Not checking, pray user passed correctly!
	server_id = os.Args[1]
	ip_addr = os.Args[2]
	serv_port = os.Args[3]
	rec_port = os.Args[4] 
	ip_serv_port := ip_addr+":"+serv_port
	ip_rec_port := ip_addr+":"+rec_port
	lb_ip_addr = os.Args[5]
	lb_port = os.Args[6]

	// If the file doesn't exist, create it or append to the file
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
	defer file.Close()

	// Start the server and listen for requests
	lis, err := net.Listen("tcp", ip_serv_port)
	if err != nil {
		log.Fatalf("Failed to listen: %v\n", err)
	}
	// Main LB facing server
	s := grpc.NewServer()

	var ret bool
	db, ret = InitDB(db_path)
	if ret {
			pb.RegisterRecoverKVServer(s, &server{})
			PrintStartMsg(port)
			if err := s.Serve(lis); err != nil {
				log.Fatalf("Failed to serve: %v\n", err)
			}
			server_mode = "ZOMBIE"
			rec_cmplt = false
			go recoveryStage(FetchLocalUID())
			// In another thread wait on completion and 
			// if rec_cmplt is true then enable GET queries.
			go func(){
				rec_mu.Lock()
				defer rec_mu.Unlock()
				// Wait on rec_cmplt until it's state changes.
				for !rec_cmplt {
						// This sleeps the thread so not really busy looping.
						rec_cond.Wait()
				}
				// time.Sleep(rec_state_chk * time.Second) // No need to wait
				if rec_cmplt {
					// Recovery success
					// Step 6.
					server_mode = "ALIVE"
					// Merge update log table to data table, only if uid > one in the data_table.
					// if ApplyUpdateLogPureSql{ // This is pure SQL version, check perf difference.
					if ApplyUpdateLog() {
						// Any new requests comming in will be made on data_table
						// load the stored data to table
						if table.LoadKV(db_path, db) {
							LB.markMeAlive()
							fmt.Println("-- Server finished recovery stage, mode change: ZOMBIE->ALIVE --")
						}
					} else {
						log.Println("Applying recent update log failed.")
					}
				} else {
					// Recovery failed because either,
					//		1. The alive server failed during our recovery.
					//		2. 	LOG replay failed for some reason.
					// Try again from another server.
					log.Fatalf("[Recovery] Recovery failed -- RETRYING AGAIN")
					// TODO: First delete the new LOG file 
					//go recoveryStage()
				}
			}()

			// Now start the recovery server on ip_rec_port

	} else {
		log.Fatalf("Server failed to start == DB not initialized.")
	}
	
}
