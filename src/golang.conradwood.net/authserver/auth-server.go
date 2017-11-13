package main

import (
	"errors"
	"flag"
	"fmt"
	"golang.conradwood.net/auth"
	pb "golang.conradwood.net/auth/proto"
	"golang.conradwood.net/server"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"os"
)

// static variables for flag parser
var (
	backend  = flag.String("backend", "none", "backend to use: all|none|postgres|file")
	port     = flag.Int("port", 4998, "The server port")
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
	} else if *backend == "none" {
		authBE = &NilAuthenticator{}
	} else if *backend == "any" {
		authBE = &AnyAuthenticator{}
	} else {
		fmt.Sprintf("Invalid backend \"%s\"\n", *backend)
		os.Exit(10)
	}

	sd := server.ServerDef{
		Port: *port,
		// we ARE the authentication service so don't insist on authenticated calls
		NoAuth: true,
	}
	sd.Register = st
	err = server.ServerStartup(sd)
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

func getUserFromToken(token string) (*auth.User, error) {
	if token == "" {
		fmt.Println("Cannot get user from token without a token")
		return nil, errors.New("Missing token")
	}
	user, err := authBE.Authenticate(token)
	if err != nil {
		fmt.Println("Failed to authenticate ", err)
		return nil, err
	}
	if user == "" {
		fmt.Println("Authenticate failed. (no result but no error)")
		return nil, errors.New("Internal authentication-server error")
	}
	au, err := getUserByID(user)
	return au, err
}

func getUserByID(userid string) (*auth.User, error) {
	a, err := authBE.GetUserDetail(userid)
	return a, err
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
	au, err := getUserFromToken(req.Token)
	fmt.Printf("Verified as user: %v (%s)\n", au, err)
	if err != nil {
		return nil, err
	}
	resp := &pb.VerifyResponse{}
	if au != nil {
		resp.UserID = au.ID
	}
	return resp, nil
}

func (s *AuthServer) GetUserDetail(ctx context.Context, req *pb.GetDetailRequest) (*pb.GetDetailResponse, error) {
	fmt.Printf("backend \"%s\" has been asked to get details for user#\"%s\"\n", *backend, req.UserID)
	if req.UserID == "" {
		return nil, errors.New("Missing token")
	}
	au, err := getUserByID(req.UserID)
	if err != nil {
		return nil, err
	}
	gd := pb.GetDetailResponse{UserID: au.ID,
		Email:     au.Email,
		FirstName: au.FirstName,
		LastName:  au.LastName,
	}
	return &gd, nil
}
