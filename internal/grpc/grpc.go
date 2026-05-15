package grpc

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
)

// Server represents the gRPC server.
type Server struct {
	port   int
	server *grpc.Server
}

// New creates a new gRPC server.
func New(port int) *Server {
	return &Server{
		port:   port,
		server: grpc.NewServer(),
	}
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	log.Printf("gRPC server starting on port %d", s.port)

	// TODO: Register gRPC services here after protobuf generation
	// Example: pb.RegisterWorkloadServiceServer(s.server, &workloadService{})

	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop stops the gRPC server gracefully.
func (s *Server) Stop() {
	log.Println("gRPC server stopping")
	s.server.GracefulStop()
}
