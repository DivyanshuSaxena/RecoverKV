package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"google.golang.org/grpc"
	"database/sql"
	pb "recoverKV/gen/recoverKV"
)

const (
	port = ":50051"
)

type BigMAP map[string]string
var table = make(BigMAP)
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
	log.Printf("GetValue Received: %v\n", key)

	var successCode int32 = 0
	val, prs := table[key]
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
	log.Printf("SetValue Received: %v:%v\n", key, newVal)

	var successCode int32 = 0
	val, prs := table[key]
	table[key] = newVal
	
	if !prs {
		val = newVal
		successCode = 1
	}

	// TODO: Make it work with async (tests failing)
	UpdateKey(key, newVal, db)
	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

func PrintStartMsg(port string){
	name := `
	___                        _  ____   __
	| _ \___ __ _____ _____ _ _| |/ /\ \ / /
	|   / -_) _/ _ \ V / -_) '_| ' <  \ V / 
	|_|_\___\__\___/\_/\___|_| |_|\_\  \_/ `	
	
				fmt.Println(string("\033[36m"),name)
				fmt.Println()
				fmt.Println("Server started successfully on port"+port)	
}

func main() {
	fmt.Println("Starting server execution")

	// If the file doesn't exist, create it or append to the file
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
	defer file.Close()

	// Start the server and listen for requests
	lis, err := net.Listen("tcp", "localhost"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()
	// start db test.db
	var ret bool
	db, ret = InitDB("/tmp/test.db")
	if ret {
		// load the stored data to table
		if table.LoadKV("/tmp/test.db", db) {
			pb.RegisterRecoverKVServer(s, &server{})
			PrintStartMsg(port)
			if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v\n", err)
			}
		}
	} else {
		log.Fatalf("Server failed to start == DB not initialized.")
	}
}
