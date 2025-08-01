// main.go
//
// This Go application implements a gRPC server with a health checking service
// and a streaming endpoint that sends the current time. The health status
// can be toggled via an HTTP endpoint.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/gogochickenleg/envoy_hck/protos"
)

// server is used to implement the TimeServiceServer.
type server struct {
	pb.UnimplementedTimeServiceServer
}

// StreamTime sends the current time to the client every 10 seconds.
func (s *server) StreamTime(req *pb.TimeRequest, stream pb.TimeService_StreamTimeServer) error {
	log.Println("StreamTime request received")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			log.Println("Client disconnected")
			return nil
		case t := <-ticker.C:
			if err := stream.Send(&pb.TimeResponse{CurrentTime: t.Format(time.RFC3339)}); err != nil {
				log.Printf("Error sending time: %v", err)
				return status.Errorf(codes.Internal, "failed to send time: %v", err)
			}
			log.Printf("Sent time: %s", t.Format(time.RFC3339))
		}
	}
}

var (
	// Mutex to protect access to the health status
	mu sync.Mutex
	// The current health status of the service
	isHealthy = true
)

func main() {
	// Define command-line flags for ports
	grpcPort := flag.Int("grpc-port", 50051, "The gRPC server port")
	httpPort := flag.Int("http-port", 8081, "The HTTP server port for health toggling")
	flag.Parse()

	// --- gRPC Server ---
	grpcAddr := fmt.Sprintf(":%d", *grpcPort)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	// Register the TimeService
	pb.RegisterTimeServiceServer(s, &server{})

	// Register the health service.
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	// Register reflection service on gRPC server.
	reflection.Register(s)

	// Set the initial health status for our service.
	// An empty service name means it applies to the entire server.
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	go func() {
		log.Println("gRPC server listening at", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// --- HTTP Server for Health Toggle ---
	http.HandleFunc("/toggle-health", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		isHealthy = !isHealthy

		var statusString string
		if isHealthy {
			healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
			statusString = "HEALTHY"
		} else {
			healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			statusString = "UNHEALTHY"
		}

		log.Printf("Health status toggled to: %s", statusString)
		fmt.Fprintf(w, "Health status is now %s\n", statusString)
	})

	httpAddr := fmt.Sprintf(":%d", *httpPort)
	log.Printf("Health toggle server listening at %s", httpAddr)
	if err := http.ListenAndServe(httpAddr, nil); err != nil {
		log.Fatalf("failed to start HTTP server: %v", err)
	}
}
