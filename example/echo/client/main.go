package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/xxpbb/brpc-go"
	"google.golang.org/protobuf/proto"

	pb "github.com/xxpbb/brpc-go/example/echo"
)

var (
	addr = flag.String("addr", "localhost:8000", "the address to connect to")
)

func main() {
	flag.Parse()

	// Set up a connection to the server.
	conn, err := brpc.Dial(*addr)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewEchoServiceClient(conn)

	// Contact the server and print out its response.
	for range time.Tick(time.Second) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		attach := make([]byte, 0)
		r, err := c.Echo(ctx, &pb.EchoRequest{Message: proto.String("hello world")}, brpc.WithRequestAttachment([]byte("abc")), brpc.WithResponseAttachment(&attach))
		if err != nil {
			log.Printf("could not echo: %v", err)
			cancel()
			continue
		}
		log.Printf("EchoResponse: %s, attachment: %s\n", r.GetMessage(), attach)
		cancel()
	}
}
