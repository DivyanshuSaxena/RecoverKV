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

// server is used to implement RecoverKV service.
type server struct {
	pb.UnimplementedRecoverKVServer
}

// getValue implements RecoverKV.getValue
func (s *server) getValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	log.Printf("Received: %v", in.GetKey())
	return &pb.Response{Value: "RandomValue", SuccessCode: 0}, nil
}

// setValue implements RecoverKV.setValue
func (s *server) setValue(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	log.Printf("Received: %v", in.GetKey())
	return &pb.Response{Value: "RandomValue", SuccessCode: 0}, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterRecoverKVServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
