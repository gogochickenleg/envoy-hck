// main.go
//
// This Go application implements a gRPC server with a health checking service
// and a streaming endpoint that sends the current time. The health status
// can be toggled via an HTTP endpoint.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
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
	// --- Load TLS credentials ---
	serverCert, err := tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
	if err != nil {
		log.Fatalf("failed to load server cert: %v", err)
	}

	caCert, err := os.ReadFile("certs/ca.crt")
	if err != nil {
		log.Fatalf("failed to read ca cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert, // Require clients to present a cert from our CA
	}

	creds := credentials.NewTLS(tlsConfig)

	// --- gRPC Server ---
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.Creds(creds)) // Apply TLS credentials to the server

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

	log.Println("Health toggle server listening at :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("failed to start HTTP server: %v", err)
	}
}
