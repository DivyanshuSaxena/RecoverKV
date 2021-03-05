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
	"strings"
	"io"
	"errors"
)

var ip_addr string
var serv_port string
var rec_port string
var server_id string
var lb_ip_addr string
var lb_port string
var ip_serv_port string
var db_path string
// Refer persist.go for log path

//var LB pb.UnimplementedInternalClient
var LB pb.InternalClient

// setter and getter for the mode can be helpful for LB
var server_mode string // 3 modes, DEAD->ZOMBIE->ALIVE

// BigMAP is the type for the (key-value) pairs table
type BigMAP map[string]string

var (
	table   = make(BigMAP)
	tableMu = sync.Mutex{}
)
var db *sql.DB

// server is used to implement RecoverKV service.
type server struct {
	pb.UnimplementedInternalServer
}

// GetValue implements RecoverKV.GetValue
// Returns (val, 0) if the key is found
// 				 ("", 1) if the key is absent
func (s *server) GetValue(ctx context.Context, in *pb.InternalRequest) (*pb.InternalResponse, error) {
	key := in.GetKey()
	//log.Printf("GetValue Received: %v\n", key)
	var successCode int32 = 0

	tableMu.Lock()
	val, prs := table[key]
	tableMu.Unlock()

	if !prs {
		successCode = 1
	}

	return &pb.InternalResponse{Value: val, SuccessCode: successCode}, nil
}

// SetValue implements RecoverKV.SetValue
// Returns (old_value, 0) if key present
// 				 (new_value, 1) if key absent
func (s *server) SetValue(ctx context.Context, in *pb.InternalRequest) (*pb.InternalResponse, error) {
	key := in.GetKey()
	newVal := in.GetValue()
	uid := in.GetQueryID()
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
	return &pb.InternalResponse{Value: val, SuccessCode: successCode}, nil
}

func (s *server) stopServer(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {

	go func (){
		// wait for 3 seconds
		time.Sleep(3)
		// then exit the parent process
		os.Exit(0)
	}()

	return new(emptypb.Empty),nil
}

func (s *server) partitionServer(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {

	// On partition healing fetch data that is not there.
	// We only talk to reachable Alive servers
	go recoveryStage()
	return new(emptypb.Empty),nil
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
func (s server) FetchQueries(in *pb.RecRequest, srv pb.Internal_FetchQueriesServer) error {
	fmt.Println("[Recovery] Responding to server "+in.GetAddress())
	log.Println("[Recovery] Responding to server "+in.GetAddress())

	var ferr int32
	ferr = 0
	// Parse the received missing UIDs from the recovering node
	missingList := strings.Split(in.GetMissingUIDs(), "|")
	// add a element with 
	for _, missingRange := range missingList {
		// Extract the individual ranges
		rangeBound := strings.Split(missingRange, "-")
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
	return nil
}

func rpcRequestLogs(peer_addr string, global_uid int64) (bool,error){
	// send query to DB
	str := GetHolesInLogTable(global_uid)
	if str == "" {
		return false, fmt.Errorf("[Recovery] failed to get holes.")
	}

	// dial peer
	conn, err := grpc.Dial(peer_addr, grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		return false, fmt.Errorf("[Recovery] Failed to connect with peer %v : %v",peer_addr, err)
	}

	// creating stream
	client := pb.NewInternalClient(conn)
	in := &pb.RecRequest{MissingUIDs: str, Address: peer_addr}
	stream, err := client.FetchQueries(context.Background(), in)
	if err != nil {
		return false, fmt.Errorf("[Recovery] Open stream error for peer %v : %v", peer_addr, err)
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
				log.Printf("[Recovery] Cannot receive %v", err)
				return
			}
			if resp.GetFoundError() == 1 {
				log.Printf("[Recovery] Buddy server has an error.")
				return
			}
			if ApplyQuery(resp.GetQuery()) {
				log.Printf("[Recovery] failed to apply query %x : ABORTING recovery.", resp.Query)
			}
			fmt.Printf("[Recovery] Replayed query %s -- ", resp.GetQuery())
		}
	}()

	if <-done {
		log.Println("[Recovery] All queries are replayed now.")
		// This return true does not guarantee all holes are filled.
		return true, nil
	}
	return false, fmt.Errorf("[Recovery] Failed in rpcRequestLogs")

}

func MarkMe(status int32) (int64,error){
	privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := LB.MarkMe(privateCtx, &pb.MarkStatus{ServerName: ip_serv_port,NewStatus: status})
	if err!=nil {
		log.Println("[Recovery] MarkMe failed during recovery!")
		return 0,errors.New("[Recovery] MarkMe failed during recovery!")
	}
	return resp.GetGlobalUID(), nil
}

func FetchAlivePeers() (string,error){
	privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := LB.FetchAlivePeers(privateCtx, &pb.ServerInfo{ServerName: ip_serv_port})
	if err!=nil {
		log.Println("[Recovery] Fetching alive peers failed during recovery!")
		return "",errors.New("[Recovery] Fetching alive peers failed during recovery!")
	}
	return resp.GetAliveList(), nil
}

/*
************************************************************
Recovery stage:
************************************************************
1. Don't serve GETs yet! PUTs can be served. Mark thyself zombie.
	* However, PUTs go to a different table. update_log
2. Fetch ALIVE peers
3. find holes in log table and send them to peer for processing.
4. Once received all requested queries, apply them.
5. Load db to memory.
6. Mark server ALIVE. To get enable GET requests.
* Time this and report..
*/
func recoveryStage() {

	server_mode = "ZOMBIE"

	// step 1
	global_uid,err := MarkMe(0);
	if err!= nil {
		log.Println("MarkMe failed so quiting!")
		return
	}

	// step 2
	peers,err := FetchAlivePeers()
	if err!=nil{
		log.Println("[Recovery] failed to fetch peers so quiting recovery!")
		return
	}
	peer_list := strings.Split(peers, ",")
	if err !=nil {
		log.Println("[Recovery] Failed to get ALIVE peers or peer list is empty.")
		return
	}

	if len(peer_list) == 0 {
		server_mode = "ALIVE"
		if table.LoadKV(db_path, db) {
			_, err := MarkMe(1);
			if err!=nil {
				log.Println("[Recovery] rec complete but could not mark myself alive!")
				return
			}
			fmt.Println("-- Server finished recovery stage, mode change: ZOMBIE->ALIVE --")
			return
		}
	}
	// ADDON: Bellow can be extended to make multiple requests from different peers
	// Track percentage completion of each recovery and kill other routines after
	// a threshold.
	
	// Step 3 & 4 
	// Make this seq?
	var rec_success bool
	for _,peer := range(peer_list) {
		rec_success, err = rpcRequestLogs(peer, global_uid) 
			// Done processing all queries, so mark completion
			// TODO: Add break here when number of holes requested 
			// 		is satisfied - can achieve this with a counter.
	}

	if rec_success{
		// Recovery success
		// Step 6.
		server_mode = "ALIVE"
			// Any new requests comming in will be made on data_table
			// load the stored data to table
		if table.LoadKV(db_path, db) {
			_, err := MarkMe(1);
			if err!=nil {
				log.Println("[Recovery] rec complete but could not mark myself alive!")
				return
			}
			fmt.Println("-- Server finished recovery stage, mode change: ZOMBIE->ALIVE --")
			return
		} else {
			fmt.Println("Loading to memory failed == Failing server...bye.")
		}
	} else {
		// Recovery failed because either,
		//		1. The healer failed during our recovery.
		//		2. LOG replay failed for some reason.
		// Try again from another server.
		log.Println("[Recovery] Recovery failed because no healers to heal from")
	}
}

// Takes 6 arguments
// unique id of the server. ip address, serve port, recovery port, LB ip addr, LB port
func main() {
	//fmt.Println("Starting server execution")
        server_mode = "DEAD"
	rand.Seed(time.Now().Unix())
	//parse arguments -- Not checking, pray user passed correctly!
	server_id = os.Args[1]
	ip_addr = os.Args[2]
	serv_port = os.Args[3]
	rec_port = os.Args[4] 
	ip_serv_port := ip_addr+":"+serv_port
	ip_rec_port := ip_addr+":"+rec_port
	lb_ip_addr = os.Args[5]
	lb_port = os.Args[6]
	ip_lb_port := lb_ip_addr+":"+lb_port

	db_path = "/tmp/"+serv_port
	// If the file doesn't exist, create it or append to the file
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
	defer file.Close()

	//Client for LB
	conn, err := grpc.Dial(ip_lb_port, grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		log.Fatalf("[Load balancer] Failed to connect with peer %v : %v",ip_lb_port, err)
	}
	LB = pb.NewInternalClient(conn)



	// Start the server and listen for requests
	lis, err := net.Listen("tcp", ip_serv_port)
	if err != nil {
		log.Fatalf("Failed to listen: %v\n", err)
	}
	lis2, err := net.Listen("tcp", ip_rec_port)
	// Main LB facing server
	s := grpc.NewServer()
	s2 := grpc.NewServer()

	var ret bool
	db, ret = InitDB(db_path)
	if ret {
			// Serving GET/PUT
			pb.RegisterInternalServer(s, &server{})
			if err := s.Serve(lis); err != nil {
				log.Fatalf("Failed to serve GET/PUT: %v\n", err)
			}
			// Serving recovery stage
			pb.RegisterInternalServer(s2, &server{})
			if err := s2.Serve(lis2); err != nil {
				log.Fatalf("Failed to serve recover: %v\n", err)
			}

			// Recovery stage 
			go recoveryStage()
			PrintStartMsg()

	} else {
		log.Fatalf("Server failed to start == DB not initialized. Bye.")
	}
	
}
