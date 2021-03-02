package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net"
	"strings"

	"google.golang.org/grpc"
	pb "recoverKV/gen/recoverKV"
)

/*
* Servers can be in three modes.
* 	DEAD - 		Not responding or marked to be killed
*	ZOMBIE - 	responding but recovering; Only supports PUT and
			 	can't be used to recover other ZOMBIE servers.
* 	ALIVE -		Responds to all requests and can be used to
				recover ZOMBIE servers.

Load balancer must maintain 
---------------------------------------------------------------------------------
ID 		|ip_addr 		|serv_port		|rec_port 		| mode 		| peer_list	|
---------------------------------------------------------------------------------
peer_list = initially everyone but this can change as client requests.
*/

// Defines the port to run and total nodes in system
const (
	port  = ":50050"
	nodes = 3
)

// Holds server states
type ServerInstances struct {
	////////////////////////////////////////////////////
	//TODO: servers map[string]RecoverKVClient <--- this is what we should used  for contacting servers
	////////////////////////////////////////////////////
	servers []string
}

// Contains mapping of client ID <-> server instances
type clientMAP map[string]ServerInstances

var (
	table = make(clientMAP)
)

type loadBalancer struct {
	pb.UnimplementedRecoverKVServer
}

func (lb *loadBalancer) PartitionServer(ctx context.Context, in *pb.PartitionRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetObjectID()
	serverName := in.GetServerName()
	reachable := in.GetReachable()
	log.Printf("Received in stop server: %v:%v:%v\n", clientID, serverName, reachable)

	serverData, prs := table[clientID]
	if !prs {
		log.Printf("Client ID does not exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Client ID does not exists!")
	}

	// Check if client is allowed to interact with the server name
	found := false
	for _, allowedServer := range serverData.servers {
		if serverName == allowedServer {
			found = true
		}
	}
	if !found {
		log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, serverName)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
	}

	////////////////////////////////////////////////////
	//TODO: Check if client can interact with servers in reachable list
	//TODO: if no, successCode -1
	//TODO: If yes, Contact server and update its local membership table

	// serverData[serverName] <- send partition call to this
	// check it it exists, if not successCode = -1
	log.Println(serverData)
	////////////////////////////////////////////////////

	// Server Successfully Partitioned
	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) StopServer(ctx context.Context, in *pb.KillRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetObjectID()
	serverName := in.GetServerName()
	cleanType := in.GetCleanType()
	log.Printf("Received in stop server: %v:%v:%v\n", clientID, serverName, cleanType)

	serverData, prs := table[clientID]
	if !prs {
		log.Printf("Client ID does not exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Client ID does not exists!")
	}

	// Check if client is allowed to interact with the server name
	found := false
	for _, allowedServer := range serverData.servers {
		if serverName == allowedServer {
			found = true
		}
	}
	if !found {
		log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, serverName)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
	}

	////////////////////////////////////////////////////
	//TODO: Send a stop GRPC call to that particular server. It will exit based on
	// the clean type.

	// serverData[serverName] <- send kill call to this
	// check it it exists, if not successCode = -1

	//TODO: Change input server name status to unavailable globally as well
	////////////////////////////////////////////////////

	// Server Successfully shutdown
	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) InitLBState(ctx context.Context, in *pb.StateRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetObjectID()
	serverList := in.GetServersList()
	log.Printf("Received in Init: %v:%v\n", clientID, serverList)

	_, prs := table[clientID]
	if prs {
		log.Printf("Client ID already exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Client ID already exists!")
	}

	//TODO: Instead of adding server names we can just add grpc server objects, which
	// we initialzed in main

	// Create a new conn state
	serverListSlice := strings.Split(serverList, ",")
	serverData := ServerInstances{
		servers: serverListSlice[:len(serverListSlice)-1],
	}

	////////////////////////////////////////////////////
	//TODO: Add the grpc server objcts to the table map
	for _, serverName := range serverData.servers {
		log.Printf("Server to contact: %v\n", serverName)
		//TODO: On failure with grpc conn, return -1 successcode
	}
	////////////////////////////////////////////////////

	// GRPC connection to servers successful, save state
	table[clientID] = serverData

	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) FreeLBState(ctx context.Context, in *pb.StateRequest) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetObjectID()
	log.Printf("Received in Free LB: %v\n", clientID)

	serverData, prs := table[clientID]
	if !prs {
		log.Printf("Client ID does not exists!: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Client ID does not exists!")
	}

	////////////////////////////////////////////////////
	//TODO: Close GRPC connection with everyone in servers list
	for _, serverName := range serverData.servers {
		log.Printf("Server to contact: %v\n", serverName)
		//TODO: On failure with grpc shutdown, return -1 successcode
	}
	////////////////////////////////////////////////////

	// GRPC connection to servers successfully closed, free state
	delete(table, clientID)

	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) GetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetObjectID()
	log.Printf("Object ID Received in Get: %v\n", clientID)

	serverData, prs := table[clientID]
	if !prs {
		log.Printf("Cannot find Client ID: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Client ID does not exists!")
	}

	////////////////////////////////////////////////////
	//TODO: Contact ANY ALIVE server grpc object and get value,successCode for this key
	// Retry twice if request failed. If server did not respond mark server as DEAD!
	// Next retry same query from another ALIVE server
	log.Printf("GetValue Received: %v\n", in.GetKey())
	log.Printf("Server to contact: %v\n", serverData.servers[rand.Intn(nodes)])

	val := "NULL"
	////////////////////////////////////////////////////

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

func (lb *loadBalancer) SetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {

	var successCode int32 = 0

	clientID := in.GetObjectID()
	log.Printf("Object ID Received in Set: %v\n", clientID)

	serverData, prs := table[clientID]
	if !prs {
		log.Printf("Cannot find Client ID: %v\n", clientID)
		return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Client ID does not exists!")
	}

	////////////////////////////////////////////////////
	//TODO: Contact ALL alive servers and update value,successCode for this key
	// [!] Also generate an ID for this Query
	// if server does not respond back, don't bother. However, if server didn't respond
	// twice in a row then mark that server DEAD!
	log.Printf("SetValue Received: %v:%v\n", in.GetKey(), in.GetValue())

	for _, serverName := range serverData.servers {
		log.Printf("Server to contact: %v\n", serverName)
	}

	val := "NULL"
	////////////////////////////////////////////////////

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

func main() {

	////////////////////////////////////////////////////
	//TODO: Establish connection with all nodes and store a global mapping
	// of name <host:port> <-> client object. We should also store a bit
	// if they are alive or dead (dead being categorized by timeout or fail-stop)
	// This makes sure we always make three conn (independent of number of client joining)
	////////////////////////////////////////////////////

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
