// Package main implements a server for Greeter service.
package main

import (
	"context"
	"log"
	"net"

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

// getValue implements RecoverKV.getValue
// Returns (val, 0) if the key is found
// 				 ("", 1) if the key is absent
func (s *server) getValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	key := in.GetKey()
	log.Printf("Received: %v", key)

	var successCode int32 = 0
	val, prs := table[key]
	if !prs {
		successCode = 1
	}

	return &pb.Response{Value: val, SuccessCode: successCode}, nil
}

// setValue implements RecoverKV.setValue
// Returns (old_value, 0) if key present
// 				 (new_value, 1) if key absent
func (s *server) setValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	key := in.GetKey()
	newVal := in.GetValue()
	log.Printf("Received: %v", key)

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
	// Start the server and listen for requests
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterRecoverKVServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	log.Printf("Started server successfully")
}
