package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	pb "recoverKV/gen/recoverKV"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
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
// Also, defines the server tied with this load balancer
// If masterLB = backend, then the current instance is the master
var (
	ip_port     string
	nodes       int
	backend     int
	masterLB    int
	backendConn pb.InternalClient
	masterConn  pb.InternalClient
)

// NodeInfo read from the configuration
type NodeInfo struct {
	IpAddr     string `json:"ipAddr"`
	ServPort   int    `json:"servPort"`
	RecPort    int    `json:"recPort"`
	RouterPort int    `json:"routerPort"`
}

// Configuration of the cluster read from the config file
type Configuration struct {
	Servers []NodeInfo `json:"servers"`
}

// ServerInstance holds server states
// serverID		- is the index of the server in the serverList
// routerName	- ipAddr:routerPort of the server instance
// routerConn	- the connection to the router of the server (for all PUT and GET requests)
// mode 		- can take one of: 1: ALIVE, 0: ZOMBIE, -1: DEAD
// lock 		- is a Mutex lock over the struct so that concurrent operations on the struct are safe.
// 						Enforces mutual exclusion over mode and blockedPeers
type ServerInstance struct {
	serverID     int
	routerName   string
	routerConn   pb.InternalClient
	servPort     string
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
	queryTimeMu         = sync.Mutex{}
	clientMap           = make(clientMAP)
	serverNameMap       = make(serverNameMAP) // Client sends the name of the router
	serverList    []ServerInstance
	address       []string
	recPort       []string
	servPort      []string
)

type loadBalancer struct {
	pb.UnimplementedRecoverKVServer
}

type loadBalancerInternal struct {
	pb.UnimplementedInternalServer
}

// Data types and variables required for concurrent Puts
type setValueChan struct {
	serverID int
	act      string
	val      string
	code     int32
}

const concurrentPuts bool = true

var queryFile *os.File

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
		if serverName == serverList[allowedServerID].routerName {
			found = 1
		}
	}

	return found
}

// StartServer startes the server with serverId given
func StartServer(serverID int) {
	server := serverList[serverID]

	// Sleep to allow server exit
	time.Sleep(time.Second)
	// tmp := strings.Split(ip_port, ":") // parse ip and port

	// TODO: Update server start script
	cmd := exec.Command("./run_server.sh", strconv.Itoa(serverID), server.servPort, "0")
	cmd.Stdout = os.Stdout
	log.Println("Started dead server ", server.routerName)
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func (lb *loadBalancer) PartitionServer(ctx context.Context, in *pb.PartitionRequest) (*pb.Response, error) {
	// Method deprecated now. No longer support partitioning from the client.

	var successCode int32 = 0

	clientID := in.GetClientID()
	serverName := in.GetServerName()
	reachable := in.GetReachable()
	reachableList := strings.Split(reachable, ",")
	log.Printf("Received in partition server: %v:%v:%v\n", clientID, serverName, reachable)

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
		reachableMap := make(map[string]struct{}, len(reachableList))
		for _, reachableName := range reachableList {
			ret := CanContactServer(clientID, reachableName)
			if ret != 1 {
				// COMPLETED: if no, successCode -1
				log.Printf("Client ID %v cannot interact with this server: %v\n", clientID, reachableName)
				return &pb.Response{Value: "", SuccessCode: -1}, errors.New("Permission model error")
			}
			reachableMap[reachableName] = struct{}{}
		}

		// add blocked peers
		for nameI := range serverNameMap {
			if _, prs := reachableMap[nameI]; !prs {
				blockedPeers = append(blockedPeers, serverNameMap[nameI])
			}
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

		// NOTE: Incorrect connection here - method deprecated for now.
		_, err := serverList[serverID].routerConn.PartitionServer(privateCtx, emptyMsg)
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
	// Method executed by Master

	// If call received here: then, this instance must be the Master. If not -- send error
	if masterLB != backend {
		return &pb.Response{Value: serverList[masterLB].routerName, SuccessCode: 2}, errors.New("node not master")
	}

	// Following logic -- executed by the Master LB
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

	if serverID == backend {
		// serverList[serverID] <- send kill call to this
		privateCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// ServerInstance.conn is Read only -- hence, not protected by a lock
		emptyMsg := new(emptypb.Empty)
		_, err := backendConn.StopServer(privateCtx, emptyMsg)

		// check it the server has exited, if not successCode = -1
		if err != nil {
			// TODO: Check if this will respond back in case the server has crashed
			log.Printf("Error: %v\n", err)
			return &pb.Response{Value: "", SuccessCode: -1}, errors.New("server stop failed")
		}
	} else {
		// Send request to each of the slave routers so they can process it accordingly.
		for i := 0; i < len(serverList); i++ {
			if i == backend {
				continue
			}
			_, err := serverList[i].routerConn.StopServer(context.Background(), &emptypb.Empty{})
			if err != nil {
				// TODO: Check if this will respond back in case the server has crashed
				log.Printf("Error: %v\n", err)
				return &pb.Response{Value: "", SuccessCode: -1}, errors.New("server stop failed")
			}
		}
	}

	// Change input server name status to unavailable globally as well
	if cleanType == 1 {
		serverList[serverID].lock.Lock()
		serverList[serverID].mode = -1
		serverList[serverID].lock.Unlock()

		// Start the server irrespective of whether the server is the current master's backend or not.
		go StartServer(serverID)
	}

	// Server Successfully shutdown
	return &pb.Response{Value: "", SuccessCode: successCode}, nil
}

func (lb *loadBalancer) InitLBState(ctx context.Context, in *pb.StateRequest) (*pb.Response, error) {
	log.Debugf("%v\n", serverList)
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
	log.Debug(serverListSlice)

	for i, serverName := range serverListSlice {
		serverID := serverNameMap[serverName]

		// Check if the requested server is alive
		serverList[serverID].lock.Lock()
		mode := serverList[serverID].mode
		serverList[serverID].lock.Unlock()

		log.Printf("Mode for server %v is %v\n", serverID, mode)
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
	log.Debug("InitLB: client servers: ", clientMap[clientID])

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
	// Method executed by Master

	// If call received here: then, this instance must be the Master. If not -- send error
	if masterLB != backend {
		return &pb.Response{Value: serverList[masterLB].routerName, SuccessCode: 2}, errors.New("node not master")
	}

	// Following logic -- executed by the Master LB
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
	log.Debugf("GetValue Received: %v\n", key)

	maxTries := 500
	// In case a server times out, retry connection for maximum maxTries
	for i := 0; i < maxTries; i++ {

		// Find any alive server
		serverID := clientData.servers[rand.Intn(len(clientData.servers))]

		serverList[serverID].lock.Lock()
		mode := serverList[serverID].mode
		serverList[serverID].lock.Unlock()

		if mode != 1 {
			// time.Sleep(50 * time.Millisecond)
			log.Debugf("Found dead server %v at iter %v\n", serverID, i)
			continue
		}

		log.Debugf("Found alive server %v at iter %v\n", serverID, i)

		if serverID == backend {
			// Send request to the attached backend Server
			resp, err := backendConn.GetValue(context.Background(), &pb.InternalRequest{QueryID: 0, Key: key, Value: val})
			if err != nil {
				log.Printf("error while contacting backend %v: %v\n", serverID, err)
				statusErr, _ := status.FromError(err)
				log.Printf("%v %v\n", err, statusErr)

				// If timed out, mark node as DEAD
				if statusErr.Code() == codes.Unavailable {
					log.Printf("Code unavailable while contacting server %v\n", serverList[serverID].routerName)

					serverList[serverID].lock.Lock()
					serverList[serverID].mode = -1
					serverList[serverID].lock.Unlock()

					StartServer(serverID)
				}
				continue
			}

			log.Debug("GetValue Response: ", resp.GetValue())
			return &pb.Response{Value: resp.GetValue(), SuccessCode: resp.GetSuccessCode()}, nil
		}

		// ServerInstance.conn is Read only -- no lock needed for safety
		resp, err := serverList[serverID].routerConn.GetValue(context.Background(), &pb.InternalRequest{QueryID: 0, Key: key, Value: val})
		if err != nil {
			log.Printf("error while contacting backend %v: %v\n", serverID, err)
			statusErr, _ := status.FromError(err)
			log.Printf("%v %v\n", err, statusErr)

			// If timed out, mark node as DEAD
			if statusErr.Code() == codes.Unavailable {
				log.Printf("Code unavailable while contacting server %v\n", serverList[serverID].routerName)

				serverList[serverID].lock.Lock()
				serverList[serverID].mode = -1
				serverList[serverID].lock.Unlock()

				StartServer(serverID)
			}
			continue
		}

		log.Debug("GetValue Response: ", resp.GetValue())
		return &pb.Response{Value: resp.GetValue(), SuccessCode: resp.GetSuccessCode()}, nil
	}

	log.Printf("No successful connection for get could be made\n")
	// No successful connection could be made
	return &pb.Response{Value: "", SuccessCode: -1}, errors.New("operation failed")
}

func (lb *loadBalancer) SetValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	// Method executed by Master

	// If call received here: then, this instance must be the Master. If not -- send error
	if masterLB != backend {
		return &pb.Response{Value: serverList[masterLB].routerName, SuccessCode: 2}, errors.New("node not master")
	}

	funcStart := time.Now()

	// Following logic -- executed by the Master LB
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
	log.Debugf("SetValue Received: %v:%v\n", in.GetKey(), in.GetValue())

	// [!] Also generate an ID for this Query
	var savedQueryID int64 = 0
	queryIDMu.Lock()
	queryID = queryID + 1
	savedQueryID = queryID
	queryIDMu.Unlock()

	// Count number of successful writes
	successfulPuts := 0

	log.Debugf("%v In SetValue, server list: %v", time.Now().Unix(), serverList)

	// CONCURRENT PUTS - Use a channel to send requests to all servers in parallel
	var wg sync.WaitGroup
	c := make(chan setValueChan)

	// Add threads to wait group
	wg.Add(len(serverList))
	go func() {
		wg.Wait()
		close(c)
	}()

	for serverID := range serverList {
		go func(id int) {
			defer wg.Done()

			if serverList[id].mode != -1 {
				// Send request to the respective server. If connection unavailable, start server
				// start := time.Now()

				if id == backend {
					resp, _ := backendConn.SetValue(context.Background(), &pb.InternalRequest{QueryID: savedQueryID, Key: in.GetKey(), Value: in.GetValue()})
					if serverList[id].mode == 1 {
						c <- setValueChan{id, "success", resp.GetValue(), resp.GetSuccessCode()}
					}
				} else {
					resp, err := serverList[id].routerConn.SetValue(context.Background(), &pb.InternalRequest{QueryID: savedQueryID, Key: in.GetKey(), Value: in.GetValue()})
					// elapsed := time.Since(start)
					// log.Printf("Put request to %v took %s", id, elapsed)

					if err != nil {
						statusErr, _ := status.FromError(err)
						log.Printf("error while contacting %v: %v || Code: %v\n", serverList[id].routerName, err, statusErr.Code())

						// If timed out, mark node as DEAD
						if statusErr.Code() == codes.Unavailable {
							c <- setValueChan{id, "start", "", -2}
						}
					} else {
						// COMPLETED: Do something if inconsistent values read from different servers
						// log.Printf("Put succeeded with resp: %v\n", resp)
						// Inconsistency can be caused due to recovering nodes. Hence, return the value of an ALIVE node
						// Currently, server does not fetch the uid from the data table
						if serverList[id].mode == 1 {
							c <- setValueChan{id, "success", resp.GetValue(), resp.GetSuccessCode()}
						}
					}
				}
			} else {
				c <- setValueChan{id, "dead", "", -2}
			}
		}(serverID)
	}
	// loopElapsed := time.Since(loopStart)
	// log.Printf("Loop took total %s", loopElapsed)

	for st := range c {
		// chanElapsed := time.Since(loopStart)
		// log.Printf("Received in channel in %s", chanElapsed)
		if st.act == "start" {
			serverID := st.serverID
			log.Printf("Code unavailable while contacting server %v\n", serverList[serverID].routerName)

			serverList[serverID].lock.Lock()
			serverList[serverID].mode = -1
			serverList[serverID].lock.Unlock()

			go StartServer(serverID)
		} else if st.act == "success" {
			// TODO: How should we evaluate successful puts? In case of recovering and alive nodes
			successfulPuts = successfulPuts + 1
			val = st.val
			successCode = st.code
		}
	}
	funcElapsed := time.Since(funcStart)
	writeString := fmt.Sprintf("%v\n", funcElapsed)
	queryFile.WriteString(writeString)

	log.Debugf("SetValue successful puts: %v\n", successfulPuts)
	if successfulPuts > 0 {
		return &pb.Response{Value: val, SuccessCode: successCode}, nil
	}

	// No successful put could be made
	return &pb.Response{Value: "", SuccessCode: -2}, errors.New("operation failed")
}

func (lb *loadBalancerInternal) MarkMe(ctx context.Context, in *pb.MarkStatus) (*pb.Ack, error) {
	// Same method used by master as well as slave LBs
	serverName := in.GetServerName()
	updatedStatus := in.GetNewStatus()

	log.Debugf("MarkMe: name %v\n", serverName)
	serverID := serverNameMap[serverName]

	var globalUID int64

	// Logic -- Performed by the load balancer tied with the backend
	if serverID == backend {
		// Check if the gRPC connection is healed or not
		emptyMsg := new(emptypb.Empty)
		flag := false
		log.Debug("Blocking MarkMe")
		for !flag {
			_, err := backendConn.PingServer(context.Background(), emptyMsg)
			if err != nil {
			} else {
				flag = true
				break
			}
		}
		log.Debug("MarkMe unblocked")

		// Forward call to the master LB for registering the state update (if self not master)
		if backend != masterLB {
			ack, _ := masterConn.MarkMe(context.Background(), in)
			globalUID = ack.GetGlobalUID()
		}
	}

	// Update status in global map
	log.Debug("MarkMe: Reached LB\n")
	serverList[serverID].lock.Lock()
	serverList[serverID].mode = updatedStatus
	log.Printf("Marked server id %v as %v\n", serverID, updatedStatus)
	serverList[serverID].lock.Unlock()

	// QueryID set by the MasterLB := Send the current max global ID back to the server
	if backend == masterLB {
		queryIDMu.Lock()
		globalUID = queryID
		queryIDMu.Unlock()
		log.Debugf("%v\n", serverList)
	}

	return &pb.Ack{GlobalUID: globalUID}, nil
}

func (lb *loadBalancerInternal) FetchAlivePeers(ctx context.Context, in *pb.ServerInfo) (*pb.AlivePeersResponse, error) {
	// Method used by the corresponding LB of the server, acting as the router.
	// COMPLETED: Any corner cases (or error handling) required here?
	serverName := in.GetServerName()
	serverID := serverNameMap[serverName]

	var aliveList string = ""
	// Iterate over all servers, and append into a string
	// TODO: Remove - Concurrency safety -- execute the block with a lock over the ServerInstance Object
	// serverList[serverID].lock.Lock()
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
			recoveryAddress := strings.Split(peerInstance.routerName, ":")[0] + ":" + peerInstance.recPort
			if !found {
				if len(aliveList) == 0 {
					aliveList = aliveList + recoveryAddress
				} else {
					aliveList = aliveList + "," + recoveryAddress
				}
			}
		}
	}
	// serverList[serverID].lock.Unlock()

	return &pb.AlivePeersResponse{AliveList: aliveList}, nil
}

func (lb *loadBalancerInternal) GetValue(ctx context.Context, in *pb.InternalRequest) (*pb.InternalResponse, error) {
	// Simply forward to the associated backend
	resp, err := backendConn.GetValue(context.Background(), in)
	return resp, err
}

func (lb *loadBalancerInternal) SetValue(ctx context.Context, in *pb.InternalRequest) (*pb.InternalResponse, error) {
	// Simply forward to the associated backend
	resp, err := backendConn.SetValue(context.Background(), in)
	return resp, err
}

func (lb *loadBalancerInternal) StopServer(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	// Simply forward to the associated backend
	resp, err := backendConn.StopServer(context.Background(), in)
	return resp, err
}

// Takes two arguments
// (index of the current node in the cluster config, num_nodes, master)
// Read these arguments from a configuration file "cluster.json":
// node_1_ip:serverPort, node_2_ip:serverPort,...recPort1, recPort2,...
func main() {

	rand.Seed(time.Now().UnixNano())

	Formatter := new(log.TextFormatter)
	Formatter.TimestampFormat = "02-01-2006 15:04:05"
	Formatter.FullTimestamp = true
	log.SetFormatter(Formatter)

	// Enable debug mode
	log.SetLevel(log.DebugLevel)

	////////////////////////////////////////////////////
	// Establish connection with all nodes and store a global mapping
	// of name <host:port> <-> client object. We should also store a bit
	// if they are alive or dead (dead being categorized by timeout or fail-stop)
	// This makes sure we always make three conn (independent of number of client joining)
	////////////////////////////////////////////////////
	backend, _ = strconv.Atoi(os.Args[1])
	nodes, _ = strconv.Atoi(os.Args[2])
	masterLB, _ = strconv.Atoi(os.Args[3])

	// Read the configuration file
	file, _ := os.Open("cluster.json")
	defer file.Close()

	bytes, _ := ioutil.ReadAll(file)

	var configuration Configuration
	json.Unmarshal(bytes, &configuration)
	fmt.Printf("%v\n", configuration)

	// Populate node details
	for i := 0; i < nodes; i++ {
		routerAddress := configuration.Servers[i].IpAddr + ":" + strconv.Itoa(configuration.Servers[i].RouterPort)
		address = append(address, routerAddress)
		recPort = append(recPort, strconv.Itoa(configuration.Servers[i].RecPort))
		servPort = append(servPort, strconv.Itoa(configuration.Servers[i].ServPort))
	}

	// Set up distinct connections to the servers.
	serverList = make([]ServerInstance, nodes)
	log.Printf("Num nodes: %v\n", nodes)
	for i := 0; i < nodes; i++ {
		// Start connection to the router of the respective server
		log.Printf("Server name: %v\n", address[i])

		// Save into global data structures
		serverNameMap[address[i]] = i

		if i == backend {
			// Get the port to run LB on
			ip_port = address[i]

			// Create the backend connection
			name := configuration.Servers[i].IpAddr + ":" + strconv.Itoa(configuration.Servers[i].ServPort)
			conn, err := grpc.Dial(name, grpc.WithInsecure())
			if err != nil {
				log.Fatalf("Could not connect: %v\n", err)
			}
			defer conn.Close()

			backendConn = pb.NewInternalClient(conn)
			log.Printf("Added server id %v\n", i)
			s := ServerInstance{serverID: i, routerName: address[i], recPort: recPort[i], servPort: servPort[i], mode: 0, lock: sync.Mutex{}}
			serverList[i] = s
			continue
		}

		// Assumes that the servers are already running
		conn, err := grpc.Dial(address[i], grpc.WithInsecure())
		if err != nil {
			log.Fatalf("Could not connect: %v\n", err)
		}
		defer conn.Close()

		c := pb.NewInternalClient(conn)
		log.Printf("Added server id %v\n", i)
		s := ServerInstance{serverID: i, routerName: address[i], routerConn: c, recPort: recPort[i], servPort: servPort[i], mode: 0, lock: sync.Mutex{}}
		serverList[i] = s

		// If i is the master, create the masterConn
		if i == masterLB && masterLB != backend {
			// Create the connection to the master
			masterAddress := configuration.Servers[i].IpAddr + ":" + strconv.Itoa(configuration.Servers[i].RouterPort)
			conn, err := grpc.Dial(masterAddress, grpc.WithInsecure())
			if err != nil {
				log.Fatalf("Could not connect: %v\n", err)
			}
			defer conn.Close()

			masterConn = pb.NewInternalClient(conn)
			log.Printf("Added master LB\n")
		}
	}

	// Open file for writing
	queryFile, _ = os.Create("/tmp/" + ip_port)
	defer queryFile.Close()

	// Start the load balancer and listen for requests
	lis, err := net.Listen("tcp", ip_port)
	if err != nil {
		log.Fatalf("Failed to listen: %v\n", err)
	}
	lb := grpc.NewServer()

	log.Println("Recover LB listening on " + ip_port)

	pb.RegisterRecoverKVServer(lb, &loadBalancer{})
	pb.RegisterInternalServer(lb, &loadBalancerInternal{})
	if err := lb.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v\n", err)
	}

}
