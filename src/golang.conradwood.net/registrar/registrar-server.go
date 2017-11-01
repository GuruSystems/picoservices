package main

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	pb "golang.conradwood.net/registrar/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/peer"
	"log"
	"net"
)

// static variables for flag parser
var (
	port = flag.Int("port", 10000, "The server port")
)

func main() {
	flag.Parse() // parse stuff. see "var" section above
	listenAddr := fmt.Sprintf(":%d", *port)
	fmt.Println("Starting VPN Manager service on ", listenAddr)
	lis, err := net.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	s := new(VpnManagerServer)
	pb.RegisterVpnManagerServer(grpcServer, s) // created by proto

	grpcServer.Serve(lis)
}

/**********************************
* implementing the functions here:
***********************************/
type VpnManagerServer struct {
	wtf int
}

// in C we put methods into structs and call them pointers to functions
// in java/python we also put pointers to functions into structs and but call them "objects" instead
// in Go we don't put functions pointers into structs, we "associate" a function with a struct.
// (I think that's more or less the same as what C does, just different Syntax)
func (s *VpnManagerServer) CreateVpn(ctx context.Context, CreateRequest *pb.CreateRequest) (*pb.CreateResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	fmt.Println(peer.Addr, "called createvpn")
	resp := pb.CreateResponse{}
	resp.Certificate = "I am a fake certificate"
	return &resp, nil
}

func (s *VpnManagerServer) Ping(ctx context.Context, pr *pb.PingRequest) (*pb.PingResponse, error) {
	fmt.Println("pong")
	return nil, nil
}
