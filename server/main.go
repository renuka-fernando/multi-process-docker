package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	pb "multi-process-docker/proto"

	"google.golang.org/grpc"
)

const socketPath = "/tmp/grpc.sock"

type server struct {
	pb.UnimplementedGreeterServer
	requestCount atomic.Int32
}

func (s *server) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	count := s.requestCount.Add(1)
	log.Printf("Received SayHello request from: %s (request #%d)", req.Name, count)

	return &pb.HelloReply{
		Message: fmt.Sprintf("Hello, %s! Welcome to gRPC over UDS.", req.Name),
		Count:   count,
	}, nil
}

func (s *server) StreamMessages(req *pb.StreamRequest, stream pb.Greeter_StreamMessagesServer) error {
	log.Printf("Received StreamMessages request for %d messages", req.Count)

	for i := int32(0); i < req.Count; i++ {
		if err := stream.Send(&pb.MessageResponse{
			Message: fmt.Sprintf("Stream message number %d", i+1),
			Index:   i + 1,
		}); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("Completed streaming %d messages", req.Count)
	return nil
}

func main() {
	log.Println("Starting gRPC Server...")

	// Remove existing socket if it exists
	if err := os.RemoveAll(socketPath); err != nil {
		log.Fatalf("Failed to remove existing socket: %v", err)
	}

	// Create Unix Domain Socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on UDS: %v", err)
	}
	defer listener.Close()

	// Set socket permissions
	if err := os.Chmod(socketPath, 0666); err != nil {
		log.Fatalf("Failed to set socket permissions: %v", err)
	}

	log.Printf("gRPC Server listening on Unix Domain Socket: %s", socketPath)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterGreeterServer(grpcServer, &server{})

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("Received signal: %v. Shutting down gracefully...", sig)
		grpcServer.GracefulStop()
	}()

	// Start serving
	log.Println("gRPC Server is ready to accept connections")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	log.Println("gRPC Server stopped")
}
