package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	pb "recoverKV/gen/recoverKV"

	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
)

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

	////////////////////////////////////////////////////
	// COMPLETED: Check if client can interact with servers in reachable list
	blockedPeers := make([]int, 0)
	for _, reachableName := range reachableList {
		ret := CanContactServer(clientID, reachableName)
		if ret != 1 {
			// COMPLETED: if no, successCode -1
			log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, reachableName)
			return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
		}
		blockedPeers = append(blockedPeers, serverNameMap[reachableName])
	}

	// COMPLETED: If yes, Contact server (IMP: No need to contact server) and update its local membership clientMap
	server := serverList[serverNameMap[serverName]]

	server.lock.Lock()
	server.blockedPeers = blockedPeers
	server.lock.Unlock()

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

	// COMPLETED: Send a stop GRPC call to that particular server. It will exit based on
	// the clean type.
	serverID := serverNameMap[serverName]

	// serverList[serverID] <- send kill call to this
	// TODO: Remove timeout from here if possible
	privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	emptyMsg := new(emptypb.Empty)
	// ServerInstance.conn is Read only -- hence, not protected by a lock
	_, err := serverList[serverID].conn.StopServer(privateCtx, emptyMsg)

	// check it it exists, if not successCode = -1
	if err != nil {
		// TODO: Check if this will respond back in case the server has crashed
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("server stop failed")
	}

	// COMPLETED: Change input server name status to unavailable globally as well
	if cleanType == 1 {
		serverList[serverID].lock.Lock()
		serverList[serverID].mode = -1
		serverList[serverID].lock.Unlock()
	}

	// Server Successfully shutdown
	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) InitLBState(ctx context.Context, in *pb.StateRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetClientID()
	requestList := in.GetServersList()
	log.Printf("Received in Init: %v:%v\n", clientID, requestList)

	_, prs := clientMap[clientID]
	if prs {
		log.Printf("Client ID already exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID already exist")
	}

	// COMPLETED: Instead of adding server names we can just add grpc server objects, which
	// we initialzed in main

	// Get the servers that the client requested
	serverListSlice := strings.Split(requestList, ",")

	// Get the serverIDs that must be added in the Client Permission object
	servers := make([]int, len(serverListSlice))

	for i, serverName := range serverListSlice {
		serverID := serverNameMap[serverName]

		// Check if the requested server is alive
		serverList[serverID].lock.Lock()
		mode := serverList[serverID].mode
		serverList[serverID].lock.Unlock()

		if mode == 1 {
			servers[i] = serverID
			log.Printf("Server to contact: %v\n", serverName)
		} else {
			// COMPLETED: On failure with grpc conn, return -1 successcode
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
	log.Printf("Object ID Received in Get: %v\n", clientID)

	clientData, prs := clientMap[clientID]
	if !prs {
		log.Printf("Cannot find Client ID: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	}

	////////////////////////////////////////////////////
	// COMPLETED: Contact ANY ALIVE server grpc object and get value,successCode for this key
	key := in.GetKey()
	log.Printf("GetValue Received: %v\n", key)

	maxTries := 10
	// In case a server times out, retry connection for maximum maxTries
	for i := 0; i < maxTries; i++ {
		// Find any alive server
		serverID := clientData.servers[rand.Intn(nodes)]

		serverList[serverID].lock.Lock()
		mode := serverList[serverID].mode
		serverList[serverID].lock.Unlock()

		for mode != 1 {
			serverID = clientData.servers[rand.Intn(nodes)]
			serverList[serverID].lock.Lock()
			mode = serverList[serverID].mode
			serverList[serverID].lock.Unlock()
		}
		serverToContact := serverList[serverID]

		// Send request to the respective server
		// COMPLETED: Use the timeout
		privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		// If timed out, mark node as DEAD
		if privateCtx.Err() == context.Canceled {
			log.Printf("Timeout while contacting server %v\n", serverToContact.name)

			serverToContact.lock.Lock()
			serverToContact.mode = -1
			serverToContact.lock.Unlock()
			i = i + 1
		} else {
			// ServerInstance.conn is Read only -- no lock needed for safety
			resp, err := serverList[serverID].conn.GetValue(privateCtx, &pb.InternalRequest{QueryID: 0, Key: key, Value: val})
			if err != nil {
				return &pb.Response{Value: "", SuccessCode: -1}, errors.New("operation failed")
			}
			return &pb.Response{Value: resp.GetValue(), SuccessCode: resp.GetSuccessCode()}, nil
		}
	}

	// No successful connection could be made
	return &pb.Response{Value: "", SuccessCode: -1}, errors.New("operation failed")
}

func (lb *loadBalancer) SetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {

	var successCode int32 = 0
	var val string = "NULL"

	clientID := in.GetClientID()
	log.Printf("Object ID Received in Set: %v\n", clientID)

	_, prs := clientMap[clientID]
	if !prs {
		log.Printf("Cannot find Client ID: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("client ID does not exist")
	}

	// Contact ALL (alive+zombie) servers and update value,successCode for this key
	log.Printf("SetValue Received: %v:%v\n", in.GetKey(), in.GetValue())

	// [!] Also generate an ID for this Query
	// COMPLETED: Handle concurrent queries (Use locks)
	var savedQueryID int64 = 0
	queryIDMu.Lock()
	queryID = queryID + 1
	savedQueryID = queryID
	queryIDMu.Unlock()

	// Resolve conflicting return values
	var latestUID int64 = 0
	// Count number of successful writes
	successfulPuts := 0

	for _, server := range serverList {
		if server.mode != -1 {
			// Send request to the respective server
			// COMPLETED: Use the timeout
			privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			// If timed out, mark node as DEAD
			if privateCtx.Err() == context.Canceled {
				log.Printf("Timeout while contacting server %v\n", server.name)

				server.lock.Lock()
				server.mode = -1
				server.lock.Unlock()
				continue
			}

			// If connection did not time out, proceed with the request
			resp, err := server.conn.SetValue(privateCtx, &pb.InternalRequest{QueryID: savedQueryID, Key: in.GetKey(), Value: in.GetValue()})
			if err == nil {
				// COMPLETED: Do something if inconsistent values read from different servers
				successfulPuts = successfulPuts + 1
				uid := resp.GetQueryID()
				if uid > latestUID {
					val = resp.GetValue()
					successCode = resp.GetSuccessCode()
				}
			}
		}
	}

	if successfulPuts > 1 {
		return &pb.Response{Value: val, SuccessCode: successCode}, nil
	}

	// No successful put could be made
	return &pb.Response{Value: "", SuccessCode: -2}, errors.New("operation failed")
}

func (lb *loadBalancer) MarkMe(ctx context.Context, in *pb.MarkStatus) (*pb.Ack, error) {
	// COMPLETED: Any corner cases (or error handling) required here?
	serverName := in.GetServerName()
	updatedStatus := in.GetNewStatus()

	// Update status in global map
	server := serverList[serverNameMap[serverName]]

	server.lock.Lock()
	server.mode = updatedStatus
	server.lock.Unlock()

	// Send the current max global ID back to the server
	queryIDMu.Lock()
	globalUID := queryID
	queryIDMu.Unlock()

	return &pb.Ack{GlobalUID: globalUID}, nil
}

func (lb *loadBalancer) FetchAlivePeers(ctx context.Context, in *pb.ServerInfo) (*pb.AlivePeersResponse, error) {
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
	// COMPLETED: Establish connection with all nodes and store a global mapping
	// of name <host:port> <-> client object. We should also store a bit
	// if they are alive or dead (dead being categorized by timeout or fail-stop)
	// This makes sure we always make three conn (independent of number of client joining)
	////////////////////////////////////////////////////

	// Set up distinct connections to the servers.
	for i := 0; i < nodes; i++ {
		// TODO: Should we run servers as entirely different processes?
		name := "localhost" + address[i]

		// Save into global data structures
		serverNameMap[name] = i

		// Assumes that the servers are already running
		conn, err := grpc.Dial(name, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("did not connect: %v\n", err)
		}
		defer conn.Close()
		c := pb.NewInternalClient(conn)
		s := ServerInstance{serverID: i, name: name, conn: c, recPort: recPort[i], mode: 1, lock: sync.Mutex{}}
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
	if err := lb.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v\n", err)
	}
}
