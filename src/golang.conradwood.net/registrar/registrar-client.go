package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	pb "golang.conradwood.net/registrar/proto"
	"log"
	"os"
)

// static variables for flag parser
var (
	serverAddr = flag.String("server_addr", "127.0.0.1:5000", "The server address in the format of host:port")
	port       = flag.Int("port", 5000, "The server port")
)

func main() {
	flag.Parse()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	fmt.Println("Connecting to server...")
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewRegistryClient(conn)
	na := flag.NArg()
	if na > 1 {
		if flag.Arg(0) == "shutdown" {
			for i := 1; i < na; i++ {
				s := flag.Arg(i)
				fmt.Printf("Shutting down service \"%s\"\n", s)
				sh := pb.ShutdownRequest{ServiceName: s}
				client.ShutdownService(context.Background(), &sh)
			}
			os.Exit(0)
		}
	}
	req := pb.ListRequest{}
	resp, err := client.ListServices(context.Background(), &req)
	if err != nil {
		log.Fatalf("failed to list services: %v", err)
	}
	fmt.Printf("%d services registered\n", len(resp.Service))
	for _, getr := range resp.Service {
		fmt.Printf("Service: %s\n", getr.Service.Name)
		for _, addr := range getr.Location.Address {
			fmt.Printf("   %s:%d\n", addr.Host, addr.Port)
		}
	}
}
