package main

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.conradwood.net/compound"
	pb "golang.conradwood.net/keyvalueserver/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/peer"
)

// static variables for flag parser
var (
	port = flag.Int("port", 10000, "The server port")
)

// callback from the compound initialisation
func st(server *grpc.Server) error {
	s := new(KeyValueServer)
	// Register the handler object
	pb.RegisterKeyValueServiceServer(server, s)
	return nil
}

func main() {
	flag.Parse() // parse stuff. see "var" section above
	sd := compound.ServerDef{
		Port: *port,
	}
	sd.Register = st
	err := compound.ServerStartup(sd)
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
	}
	fmt.Printf("Done\n")
	return
}

/**********************************
* implementing the functions here:
***********************************/
type KeyValueServer struct {
	wtf int
}

// in C we put methods into structs and call them pointers to functions
// in java/python we also put pointers to functions into structs and but call them "objects" instead
// in Go we don't put functions pointers into structs, we "associate" a function with a struct.
// (I think that's more or less the same as what C does, just different Syntax)
func (s *KeyValueServer) Put(ctx context.Context, PutRequest *pb.PutRequest) (*pb.PutResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	fmt.Println(peer.Addr, "called put")
	resp := pb.PutResponse{}
	return &resp, nil
}

func (s *KeyValueServer) Get(ctx context.Context, pr *pb.GetRequest) (*pb.GetResponse, error) {
	fmt.Println("pong")
	return nil, nil
}
