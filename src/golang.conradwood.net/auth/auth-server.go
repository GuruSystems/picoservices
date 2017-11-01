package main

import (
	"flag"
	"fmt"
	pb "golang.conradwood.net/auth/proto"
	"golang.conradwood.net/compound"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// static variables for flag parser
var (
	port = flag.Int("port", 10000, "The server port")
	crt  = "/etc/cnw/certs/rfc-server/certificate.pem"
	key  = "/etc/cnw/certs/rfc-server/privatekey.pem"
	ca   = "/etc/cnw/certs/rfc-server/ca.pem"
)

func main() {
	flag.Parse() // parse stuff. see "var" section above
	err := start()
	if err != nil {
		fmt.Printf("Failed. ", err)
	}
}

func st(server *grpc.Server) error {
	s := new(RFCManagerServer)
	// Register the handler object
	pb.RegisterRFCManagerServer(server, s)
	return nil
}
func start() error {
	sd := common.ServerDef{
		Port: *port,
	}
	sd.Register = st
	err := common.ServerStartup(sd)
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
	}
	fmt.Printf("Done\n")
	return nil
}

/**********************************
* implementing the functions here:
***********************************/
type RFCManagerServer struct {
	wtf int
}

// in C we put methods into structs and call them pointers to functions
// in java/python we also put pointers to functions into structs and but call them "objects" instead
// in Go we don't put functions pointers into structs, we "associate" a function with a struct.
// (I think that's more or less the same as what C does, just different Syntax)
func (s *RFCManagerServer) CreateRFC(ctx context.Context, CreateRequest *pb.CreateRequest) (*pb.CreateResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	fmt.Println(peer.Addr, "called createrfc")
	rfc := stupid.CreateNew()
	fmt.Println("RFC: ", rfc)
	resp := pb.CreateResponse{}
	resp.Certificate = "I am a fake certificate"
	return &resp, nil
}
