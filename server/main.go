package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	pb "recoverKV/gen/recoverKV"
)

const (
	port = ":50051"
)

var table = make(map[string]string)

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
	log.Printf("SetValue Received: %v\n", key)

	var successCode int32 = 0
	val, prs := table[key]
	if !prs {
		table[key] = newVal
		val = newVal
		successCode = 1
	}

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

func main() {
	fmt.Println("Starting server execution")

	// If the file doesn't exist, create it or append to the file
	file, err := os.OpenFile("server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)

	// Start the server and listen for requests
	lis, err := net.Listen("tcp", "localhost"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()
	pb.RegisterRecoverKVServer(s, &server{})
	fmt.Println("Server started successfully on port"+port)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v\n", err)
	}
}
