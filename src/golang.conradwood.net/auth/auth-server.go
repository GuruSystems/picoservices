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
	port   = flag.Int("port", 10000, "The server port")
	dbhost = flag.String("dbhost", "postgres", "hostname of the postgres database rdms")
	dbdb   = flag.String("database", "rpcusers", "database to use for authentication")
	dbuser = flag.String("dbuser", "root", "username for the database to use for authentication")
	dbpw   = flag.String("dbpw", "pw", "password for the database to use for authentication")
)

func main() {
	flag.Parse() // parse stuff. see "var" section above
	err := start()
	if err != nil {
		fmt.Printf("Failed. ", err)
	}
}

func st(server *grpc.Server) error {
	s := new(AuthServer)

	// Register the handler object
	pb.RegisterAuthenticationServiceServer(server, s)
	return nil
}
func start() error {
	sd := compound.ServerDef{
		Port:   *port,
		NoAuth: true,
	}
	sd.Register = st
	err := compound.ServerStartup(sd)
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
	}
	fmt.Printf("Done\n")
	return nil
}

/**********************************
* implementing the functions here:
***********************************/
type AuthServer struct {
	wtf int
}

// in C we put methods into structs and call them pointers to functions
// in java/python we also put pointers to functions into structs and but call them "objects" instead
// in Go we don't put functions pointers into structs, we "associate" a function with a struct.
// (I think that's more or less the same as what C does, just different Syntax)
func (s *AuthServer) VerifyUserToken(ctx context.Context, req *pb.VerifyRequest) (*pb.VerifyResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	fmt.Println(peer.Addr, "called verify token")
	resp := pb.VerifyResponse{}
	return &resp, nil
}
