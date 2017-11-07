package main

import (
	"errors"
	"flag"
	"fmt"
	"golang.conradwood.net/auth"
	pb "golang.conradwood.net/auth/proto"
	"golang.conradwood.net/compound"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// static variables for flag parser
var (
	backend  = flag.String("backend", "none", "backend to use: all|none|postgres|file")
	port     = flag.Int("port", 10000, "The server port")
	dbhost   = flag.String("dbhost", "postgres", "hostname of the postgres database rdms")
	dbdb     = flag.String("database", "rpcusers", "database to use for authentication")
	dbuser   = flag.String("dbuser", "root", "username for the database to use for authentication")
	dbpw     = flag.String("dbpw", "pw", "password for the database to use for authentication")
	tokendir = flag.String("tokendir", "/srv/picoservices/tokendir", "directory with token<->user files")
	authBE   auth.Authenticator
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
	var err error
	if *backend == "postgres" {
		authBE, err = NewPostgresAuthenticator(*dbhost, *dbuser, *dbpw, *dbdb)
		if err != nil {
			fmt.Println("Failed to create postgres authenticator", err)
			return err
		}
	} else if *backend == "file" {
		authBE, err = NewFileAuthenticator(*tokendir)
		if err != nil {
			fmt.Println("Failed to create file authenticator", err)
			return err
		}
	}
	sd := compound.ServerDef{
		Port: *port,
		// we ARE the authentication service so don't insist on authenticated calls
		NoAuth: true,
	}
	sd.Register = st
	err = compound.ServerStartup(sd)
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
	fmt.Printf("backend \"%s\" has been asked by \"%s\" to verify token: \"%s\"\n", *backend, peer.Addr, req.Token)
	resp := &pb.VerifyResponse{}
	if *backend == "none" {
		return nil, errors.New("backend \"none\" never authenticates anyone")
	} else if *backend == "any" {
		resp.UserID = "backend-any-user"
		return resp, nil
	} else if (*backend == "postgres") || (*backend == "file") {
		user, err := authBE.Authenticate(req.Token)
		if err != nil {
			fmt.Println("Failed to authenticate ", err)
			return nil, err
		}
		resp.UserID = user.ID
		return resp, nil
	} else {
		return nil, errors.New(fmt.Sprintf("backend \"%s\" is not implemented", *backend))
	}
}
