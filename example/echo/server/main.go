package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/xxpbb/brpc-go"

	pb "github.com/xxpbb/brpc-go/example/echo"
)

var (
	port = flag.Int("port", 8000, "The server port")
)

// server is used to implement echo.EchoServiceServer.
type server struct {
	pb.UnimplementedEchoServiceServer
}

// Echo implements echo.EchoServiceServer.
func (s *server) Echo(ctx context.Context, in *pb.EchoRequest) (*pb.EchoResponse, error) {
	cntl := brpc.GetControllerFromContext(ctx)
	log.Printf("Received: %v, attachment: %s", in.GetMessage(), cntl.GetRequestAttachment())
	cntl.SetResponseAttachment(cntl.GetRequestAttachment())
	return &pb.EchoResponse{Message: in.Message}, nil
}

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := brpc.NewServer()
	pb.RegisterEchoServiceServer(s, &server{})

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
