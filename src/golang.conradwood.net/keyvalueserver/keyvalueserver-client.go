package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	pb "golang.conradwood.net/keyvalueserver/proto"
	"log"
)

// static variables for flag parser
var (
	serverAddr = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
	port       = flag.Int("port", 10000, "The server port")
)

func main() {
	flag.Parse()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	fmt.Println("Connecting to server...")
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewVpnManagerClient(conn)
	req := pb.CreateRequest{Name: "clientvpn", Access: "testaccess"}
	resp, err := client.CreateVpn(context.Background(), &req)
	if err != nil {
		log.Fatalf("fail to createvpn: %v", err)
	}
	fmt.Printf("Response to createvpn: %v\n", resp)
}
