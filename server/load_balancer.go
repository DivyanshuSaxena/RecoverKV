package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	pb "recoverKV/gen/recoverKV"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
* Servers can be in three modes.
* 	DEAD - 		Not responding or marked to be killed
*	ZOMBIE - 	responding but recovering; Only supports PUT and
			 	can't be used to recover other ZOMBIE servers.
* 	ALIVE -		Responds to all requests and can be used to
				recover ZOMBIE servers.

Load balancer must maintain
---------------------------------------------------------------------
ID 		|ip_addr 		|serv_port		|rec_port 		| mode 		| peer_list	|
---------------------------------------------------------------------
peer_list = initially everyone but this can change as client requests.
*/

// Defines the port to run and total nodes in system
const (
	port  = ":50050"
	nodes = 3
)

// ServerInstance holds server states
// serverID	- is the index of the server in the serverList
// mode 		- can take one of: 1: ALIVE, 0: ZOMBIE, -1: DEAD
// lock 		- is a Mutex lock over the struct so that concurrent operations on the struct are safe.
// 						Enforces mutual exclusion over mode and blockedPeers
type ServerInstance struct {
	serverID     int
	name         string
	conn         pb.InternalClient
	recPort      string
	mode         int32
	blockedPeers []int
	lock         sync.Mutex
}

// ClientPermission holds server ids that a client can contact
type ClientPermission struct {
	// We are holding the server ids (to serverMap) here. The objects need not be duplicated
	servers []int
}

// Contains mapping of client ID <-> client permission instances
type clientMAP map[string]ClientPermission

// Contains mapping of server Names <-> server IDs
type serverNameMAP map[string]int

var (
	queryID       int64 = 0
	queryIDMu           = sync.Mutex{}
	clientMap           = make(clientMAP)
	serverNameMap       = make(serverNameMAP)
	serverList          = make([]ServerInstance, nodes)
	address             = [3]string{":50051", ":50052", ":50053"}
	recPort             = [3]string{":50054", ":50055", ":50056"}
)

type loadBalancer struct {
	pb.UnimplementedRecoverKVServer
}

type loadBalancerInternal struct {
	pb.UnimplementedInternalServer
}

// CanContactServer checks whether a given clientID can contact the given server.
// Returns 1 if it can, 0 if not. And returns -1 if clientID is invalid.
func CanContactServer(clientID string, serverName string) int {
	clientData, prs := clientMap[clientID]
	if !prs {
		return -1
	}

	// Check if client is allowed to interact with the server name
	found := 0
	for _, allowedServerID := range clientData.servers {
		// ServerInstance.name is Read Only -- hence, not needed to be protected by a lock
		if serverName == serverList[allowedServerID].name {
			found = 1
		}
	}

	return found
}

// Redial redials connection to a dead server
func Redial(serverID int) {
	log.Println("Opening new connection: ", serverList[serverID].name)
	// Update connection
	conn, err := grpc.Dial(serverList[serverID].name, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v\n", err)
	}
	serverList[serverID].conn = pb.NewInternalClient(conn)
}

// StartServer startes the server with serverId given
func StartServer(serverID int) {

	server := serverList[serverID]
	tmpList := strings.Split(server.name, ":")
	ipAddr := tmpList[0]
	servPort := tmpList[1]

	// Sleep to allow server exit
	time.Sleep(time.Second)
	cmd := exec.Command("./run_server.sh", ipAddr, servPort, server.recPort[1:], "localhost", port[1:], "0")
	cmd.Stdout = os.Stdout
	log.Println("Started dead server")
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Update mode to ZOMBIE
	serverList[serverID].lock.Lock()
	serverList[serverID].mode = 0
	serverList[serverID].lock.Unlock()

	//Redial(serverID)
}

func (lb *loadBalancer) PartitionServer(ctx context.Context, in *pb.PartitionRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetClientID()
	serverName := in.GetServerName()
	reachable := in.GetReachable()
	reachableList := strings.Split(reachable, ",")
	log.Printf("Received in stop server: %v:%v:%v\n", clientID, serverName, reachable)

	ret := CanContactServer(clientID, serverName)
	if ret == -1 {
		log.Printf("Client ID does not exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	} else if ret == 0 {
		log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, serverName)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
	}

	blockedPeers := make([]int, 0)
	if len(reachable) == 0 {
		// Set all to be blocked
		for nameI := range serverNameMap {
			if nameI != serverName {
				blockedPeers = append(blockedPeers, serverNameMap[nameI])
			}
		}
	} else {
		// Check if client can interact with servers in reachable list
		for _, reachableName := range reachableList {
			ret := CanContactServer(clientID, reachableName)
			if ret != 1 {
				// COMPLETED: if no, successCode -1
				log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, reachableName)
				return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
			}
			blockedPeers = append(blockedPeers, serverNameMap[reachableName])
		}
	}

	//If yes, Contact server (so that it can heal, if needed) and update its local membership clientMap
	serverID := serverNameMap[serverName]

	serverList[serverID].lock.Lock()
	prevBlockedPeers := serverList[serverID].blockedPeers
	serverList[serverID].blockedPeers = blockedPeers
	serverList[serverID].lock.Unlock()

	// Server can heal from only those it is now reachable from
	if len(reachable) != 0 {
		serverID := serverNameMap[serverName]
		privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		emptyMsg := new(emptypb.Empty)

		_, err := serverList[serverID].conn.PartitionServer(privateCtx, emptyMsg)
		// check it it exists, if not successCode = -1
		if err != nil || privateCtx.Err() == context.DeadlineExceeded {
			// Revert back to old blocked peers
			serverList[serverID].lock.Lock()
			serverList[serverID].blockedPeers = prevBlockedPeers
			serverList[serverID].lock.Unlock()
			return &pb.Response{Value: "", SuccessCode: -1}, errors.New("server cannot be partitioned")
		}
	}

	// Server Successfully Partitioned
	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) StopServer(ctx context.Context, in *pb.KillRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetClientID()
	serverName := in.GetServerName()
	cleanType := in.GetCleanType()
	log.Printf("Received in stop server: %v:%v:%v\n", clientID, serverName, cleanType)

	ret := CanContactServer(clientID, serverName)
	if ret == -1 {
		log.Printf("Client ID does not exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	} else if ret == 0 {
		log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, serverName)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
	}

	// Send a stop GRPC call to that particular server. It will exit based on
	// the clean type.
	serverID := serverNameMap[serverName]

	// serverList[serverID] <- send kill call to this
	privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// ServerInstance.conn is Read only -- hence, not protected by a lock
	emptyMsg := new(emptypb.Empty)
	_, err := serverList[serverID].conn.StopServer(privateCtx, emptyMsg)

	// check it the server has exited, if not successCode = -1
	if err != nil {
		// TODO: Check if this will respond back in case the server has crashed
		log.Printf("Error: %v\n", err)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("server stop failed")
	}

	// Change input server name status to unavailable globally as well
	if cleanType == 1 {
		serverList[serverID].lock.Lock()
		serverList[serverID].mode = -1
		serverList[serverID].lock.Unlock()

		go StartServer(serverID)
	}

	// Server Successfully shutdown
	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) InitLBState(ctx context.Context, in *pb.StateRequest) (*pb.Response, error) {
	log.Printf("%v\n", serverList)
	var successCode int32 = 0

	clientID := in.GetClientID()
	requestList := in.GetServersList()
	log.Printf("Received in Init: %v:%v\n", clientID, requestList)

	_, prs := clientMap[clientID]
	if prs {
		log.Printf("Client ID already exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID already exist")
	}

	// Get the servers that the client requested
	serverListSlice := strings.Split(requestList, ",")

	// Get the serverIDs that must be added in the Client Permission object
	servers := make([]int, len(serverListSlice))
	log.Println(serverListSlice)

	for i, serverName := range serverListSlice {
		serverID := serverNameMap[serverName]

		// Check if the requested server is alive
		serverList[serverID].lock.Lock()
		mode := serverList[serverID].mode
		serverList[serverID].lock.Unlock()

		log.Printf("Mode for server %v is %v %T\n", serverID, mode, mode)
		if mode == 1 {
			servers[i] = serverID
			log.Printf("Server to contact: %v\n", serverName)
		} else {
			// On failure with grpc conn, return -1 successcode
			log.Printf("InitLBState: Sending -1 code\n")
			return &pb.Response{Value: "", SuccessCode: -1}, nil
		}
	}

	// GRPC connection to servers successful, save state
	clientMap[clientID] = ClientPermission{servers: servers}

	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) FreeLBState(ctx context.Context, in *pb.StateRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetClientID()
	log.Printf("Received in Free LB: %v\n", clientID)

	_, prs := clientMap[clientID]
	if !prs {
		log.Printf("Client ID does not exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	}

	// GRPC connection to servers successfully closed (Not needed now)
	// Free client state
	delete(clientMap, clientID)

	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) GetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {

	var val string = ""

	clientID := in.GetClientID()
	// log.Printf("Object ID Received in Get: %v\n", clientID)

	clientData, prs := clientMap[clientID]
	if !prs {
		log.Printf("Cannot find Client ID: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	}

	// Contact ANY ALIVE server grpc object and get value,successCode for this key
	key := in.GetKey()
	log.Printf("GetValue Received: %v\n", key)

	maxTries := 5
	// In case a server times out, retry connection for maximum maxTries
	for i := 0; i < maxTries; i++ {

		// Find any alive server
		serverID := clientData.servers[rand.Intn(len(clientData.servers))]

		serverList[serverID].lock.Lock()
		mode := serverList[serverID].mode
		serverList[serverID].lock.Unlock()

		if mode != 1 {
			continue
		}

		log.Printf("Found alive server %v at iter %v\n", serverID, i)

		// Send request to the respective server
		privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// ServerInstance.conn is Read only -- no lock needed for safety
		resp, err := serverList[serverID].conn.GetValue(privateCtx, &pb.InternalRequest{QueryID: 0, Key: key, Value: val})
		if err != nil {
			log.Printf("error while contacting %v: %v\n", serverList[serverID].name, err)
			statusErr, _ := status.FromError(err)
			log.Printf("%v %v\n", err, statusErr)

			// If timed out, mark node as DEAD
			if statusErr.Code() == codes.DeadlineExceeded {
				log.Printf("Timeout while contacting server %v\n", serverList[serverID].name)

				serverList[serverID].lock.Lock()
				serverList[serverID].mode = -1
				serverList[serverID].lock.Unlock()

				go StartServer(serverID)
			}
			continue
		}

		log.Println("GetValue Response: ", resp.GetValue())
		return &pb.Response{Value: resp.GetValue(), SuccessCode: resp.GetSuccessCode()}, nil
	}

	log.Printf("No successful connection for get could be made\n")
	// No successful connection could be made
	return &pb.Response{Value: "", SuccessCode: -1}, errors.New("operation failed")
}

func (lb *loadBalancer) SetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {

	var successCode int32 = 0
	var val string = "NULL"

	clientID := in.GetClientID()
	// log.Printf("Object ID Received in Set: %v\n", clientID)

	_, prs := clientMap[clientID]
	if !prs {
		log.Printf("Cannot find Client ID: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	}

	// Contact ALL (alive+zombie) servers and update value,successCode for this key
	log.Printf("SetValue Received: %v:%v\n", in.GetKey(), in.GetValue())

	// [!] Also generate an ID for this Query
	var savedQueryID int64 = 0
	queryIDMu.Lock()
	queryID = queryID + 1
	savedQueryID = queryID
	queryIDMu.Unlock()

	// Count number of successful writes
	successfulPuts := 0

	log.Printf("%v In SetValue, server list: %v", time.Now().Unix(), serverList)
	for serverID, _ := range serverList {
		if serverList[serverID].mode != -1 {
			// Send request to the respective server
			privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			// If connection did not time out, proceed with the request
			resp, err := serverList[serverID].conn.SetValue(privateCtx, &pb.InternalRequest{QueryID: savedQueryID, Key: in.GetKey(), Value: in.GetValue()})
			if err != nil {
				log.Printf("error while contacting %v: %v\n", serverList[serverID].name, err)
				statusErr, _ := status.FromError(err)

				// If timed out, mark node as DEAD
				if statusErr.Code() == codes.DeadlineExceeded {
					log.Printf("Timeout while contacting server %v\n", serverList[serverID].name)

					serverList[serverID].lock.Lock()
					serverList[serverID].mode = -1
					serverList[serverID].lock.Unlock()

					go StartServer(serverID)
				}
				// TODO: Restart the server by executing a shell script
			} else {
				// COMPLETED: Do something if inconsistent values read from different servers
				// log.Printf("Put succeeded with resp: %v\n", resp)
				// Inconsistency can be caused due to recovering nodes. Hence, return the value of an ALIVE node
				// Currently, server does not fetch the uid from the data table
				if serverList[serverID].mode == 1 {
					// TODO: How should we evaluate successful puts? In case of recovering and alive nodes
					successfulPuts = successfulPuts + 1
					val = resp.GetValue()
					successCode = resp.GetSuccessCode()
				}
			}
		}
	}

	log.Printf("SetValue successful puts: %v\n", successfulPuts)
	if successfulPuts > 0 {
		return &pb.Response{Value: val, SuccessCode: successCode}, nil
	}

	// No successful put could be made
	return &pb.Response{Value: "", SuccessCode: -2}, errors.New("operation failed")
}

func (lb *loadBalancerInternal) MarkMe(ctx context.Context, in *pb.MarkStatus) (*pb.Ack, error) {
	// COMPLETED: Any corner cases (or error handling) required here?
	serverName := in.GetServerName()
	updatedStatus := in.GetNewStatus()

	log.Printf("MarkMe: name %v\n", serverName)
	// Update status in global map
	serverID := serverNameMap[serverName]

	log.Printf("MarkMe: Reached LB\n")
	serverList[serverID].lock.Lock()
	serverList[serverID].mode = updatedStatus
	log.Printf("Marked server %v as %v\n", serverID, updatedStatus)
	serverList[serverID].lock.Unlock()

	// Send the current max global ID back to the server
	queryIDMu.Lock()
	globalUID := queryID
	queryIDMu.Unlock()
	log.Printf("%v\n", serverList)

	return &pb.Ack{GlobalUID: globalUID}, nil
}

func (lb *loadBalancerInternal) FetchAlivePeers(ctx context.Context, in *pb.ServerInfo) (*pb.AlivePeersResponse, error) {
	// COMPLETED: Any corner cases (or error handling) required here?
	serverName := in.GetServerName()
	serverID := serverNameMap[serverName]

	var aliveList string = ""
	// Iterate over all servers, and append into a string
	// Concurrency safety -- execute the block with a lock over the ServerInstance Object
	serverList[serverID].lock.Lock()
	for _, peerInstance := range serverList {
		if peerInstance.mode == 1 {
			// Server is alive. Check if it is not in blockedPeers
			found := false
			for _, blocked := range serverList[serverID].blockedPeers {
				if peerInstance.serverID == blocked {
					found = true
				}
			}

			// If not found - Add to Alive list
			if !found {
				if len(aliveList) == 0 {
					aliveList = aliveList + peerInstance.name
				} else {
					aliveList = aliveList + "," + peerInstance.name
				}
			}
		}
	}
	serverList[serverID].lock.Unlock()

	return &pb.AlivePeersResponse{AliveList: aliveList}, nil
}

func main() {

	////////////////////////////////////////////////////
	// Establish connection with all nodes and store a global mapping
	// of name <host:port> <-> client object. We should also store a bit
	// if they are alive or dead (dead being categorized by timeout or fail-stop)
	// This makes sure we always make three conn (independent of number of client joining)
	////////////////////////////////////////////////////

	// Set up distinct connections to the servers.
	log.Printf("Num nodes: %v\n", nodes)
	for i := 0; i < nodes; i++ {
		// TODO: Should we run servers as entirely different processes?
		name := "localhost" + address[i]
		log.Printf("Server name: %v\n", name)

		// Save into global data structures
		serverNameMap[name] = i

		// Assumes that the servers are already running
		conn, err := grpc.Dial(name, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("did not connect: %v\n", err)
		}
		defer conn.Close()
		c := pb.NewInternalClient(conn)
		log.Printf("Added server id %v\n", i)
		s := ServerInstance{serverID: i, name: name, conn: c, recPort: recPort[i], mode: 0, lock: sync.Mutex{}}
		serverList[i] = s
	}

	// Start the load balancer and listen for requests
	lis, err := net.Listen("tcp", "localhost"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v\n", err)
	}
	lb := grpc.NewServer()

	log.Println("Recover LB listening on port" + port)

	pb.RegisterRecoverKVServer(lb, &loadBalancer{})
	pb.RegisterInternalServer(lb, &loadBalancerInternal{})
	if err := lb.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v\n", err)
	}

}
