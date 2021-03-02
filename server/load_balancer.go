package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net"
	"strings"
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
// serverID is the index of the server in the serverList
// mode can take one of: 1: ALIVE, 0: ZOMBIE, -1: DEAD
type ServerInstance struct {
	serverID     int
	name         string
	conn         pb.InternalClient
	recPort      string
	mode         int32
	blockedPeers []int
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
	serverID := serverNameMap[serverName]
	serverList[serverID].blockedPeers = blockedPeers

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
	_, err := serverList[serverID].conn.StopServer(privateCtx, emptyMsg)

	// check it it exists, if not successCode = -1
	if err != nil {
		// TODO: Check if this will respond back in case the server has crashed
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("server stop failed")
	}

	// COMPLETED: Change input server name status to unavailable globally as well
	if cleanType == 1 {
		serverList[serverID].mode = -1
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
		if serverList[serverID].mode == 1 {
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

	var successCode int32 = 0
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

	// Find any alive server
	serverID := clientData.servers[rand.Intn(nodes)]
	for serverList[serverID].mode != 1 {
		serverID = clientData.servers[rand.Intn(nodes)]
	}
	serverToContact := serverList[serverID]

	// Send request to the respective server
	// TODO: Use the timeout
	privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := serverToContact.conn.SetValue(privateCtx, &pb.InternalRequest{QueryID: 0, Key: key, Value: val})
	if err == nil {
		val = resp.GetValue()
		successCode = resp.GetSuccessCode()
	}

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
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

	// Contact ALL alive servers and update value,successCode for this key
	log.Printf("SetValue Received: %v:%v\n", in.GetKey(), in.GetValue())

	// [!] Also generate an ID for this Query
	// TODO: Handle concurrent queries (Use locks)
	queryID = queryID + 1
	for _, server := range serverList {
		if server.mode == 1 {
			// Send request to the respective server
			// TODO: Use the timeout
			privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			resp, err := server.conn.SetValue(privateCtx, &pb.InternalRequest{QueryID: queryID, Key: in.GetKey(), Value: in.GetValue()})
			if err == nil {
				// TODO: Do something if inconsistent values read from different servers
				val = resp.GetValue()
				successCode = resp.GetSuccessCode()
			}
		}
	}

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

func (lb *loadBalancer) MarkMe(ctx context.Context, in *pb.MarkStatus) (*pb.Ack, error) {
	// TODO: Any corner cases (or error handling) required here?
	serverName := in.GetServerName()
	updatedStatus := in.GetNewStatus()

	// Update status in global map
	serverID := serverNameMap[serverName]
	serverList[serverID].mode = updatedStatus

	return &pb.Ack{}, nil
}

func (lb *loadBalancer) FetchAlivePeers(ctx context.Context, in *pb.ServerInfo) (*pb.AlivePeersResponse, error) {
	// TODO: Any corner cases (or error handling) required here?
	serverName := in.GetServerName()
	serverID := serverNameMap[serverName]

	var aliveList string = ""
	// Iterate over all servers, and append into a string
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

	return &pb.AlivePeersResponse{AliveList: aliveList}, nil
}

func main() {

	////////////////////////////////////////////////////
	//TODO: Establish connection with all nodes and store a global mapping
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
		s := ServerInstance{serverID: i, name: name, conn: c, recPort: recPort[i], mode: 1}
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
