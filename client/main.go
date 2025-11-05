package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "multi-process-docker/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	socketPath     = "/tmp/grpc.sock"
	retryDelay     = 2 * time.Second
	requestDelay   = 5 * time.Second
	maxRetries     = 10
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
)

func main() {
	log.Println("Starting gRPC Client...")

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v. Shutting down...", sig)
		cancel()
	}()

	// Connect to server with retries
	var conn *grpc.ClientConn
	var err error

	for i := 0; i < maxRetries; i++ {
		if ctx.Err() != nil {
			log.Println("Shutdown requested, stopping connection attempts")
			return
		}

		log.Printf("Attempting to connect to server (attempt %d/%d)...", i+1, maxRetries)

		dialCtx, dialCancel := context.WithTimeout(ctx, dialTimeout)
		conn, err = grpc.DialContext(
			dialCtx,
			"unix://"+socketPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		dialCancel()

		if err == nil {
			log.Println("Successfully connected to gRPC server via UDS")
			break
		}

		log.Printf("Failed to connect: %v. Retrying in %v...", err, retryDelay)
		select {
		case <-time.After(retryDelay):
			continue
		case <-ctx.Done():
			log.Println("Shutdown requested, stopping connection attempts")
			return
		}
	}

	if err != nil {
		log.Fatalf("Failed to connect after %d attempts: %v", maxRetries, err)
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)

	// Request counter
	requestNum := 0

	// Main loop - make requests periodically
	ticker := time.NewTicker(requestDelay)
	defer ticker.Stop()

	// Make first request immediately
	makeRequests(ctx, client, &requestNum)

	for {
		select {
		case <-ticker.C:
			makeRequests(ctx, client, &requestNum)
		case <-ctx.Done():
			log.Println("Client shutting down gracefully...")
			return
		}
	}
}

func makeRequests(ctx context.Context, client pb.GreeterClient, requestNum *int) {
	*requestNum++

	// SayHello request
	log.Printf("\n--- Request #%d: SayHello ---", *requestNum)
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.SayHello(reqCtx, &pb.HelloRequest{
		Name: "Docker Client",
	})

	if err != nil {
		log.Printf("Error calling SayHello: %v", err)
		return
	}

	log.Printf("Response: %s (Server request count: %d)", resp.Message, resp.Count)

	// Every 3rd request, also test streaming
	if *requestNum%3 == 0 {
		log.Printf("\n--- Request #%d: StreamMessages ---", *requestNum)
		streamCtx, streamCancel := context.WithTimeout(ctx, requestTimeout)
		defer streamCancel()

		stream, err := client.StreamMessages(streamCtx, &pb.StreamRequest{
			Count: 5,
		})

		if err != nil {
			log.Printf("Error calling StreamMessages: %v", err)
			return
		}

		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("Stream completed")
				break
			}
			if err != nil {
				log.Printf("Error receiving stream: %v", err)
				break
			}
			log.Printf("  Received: %s (index: %d)", msg.Message, msg.Index)
		}
	}
}
