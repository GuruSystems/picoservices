package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/GuruSystems/framework/auth"
	pb "github.com/GuruSystems/framework/proto/auth"
	"github.com/GuruSystems/framework/server"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"math/rand"
	"os"
	"time"
)

// static variables for flag parser
var (
	backend  = flag.String("backend", "none", "backend to use: any|none|postgres|file|ldap|psql-ldap")
	port     = flag.Int("port", 4998, "The server port")
	Tokendir = flag.String("tokendir", "/srv/picoservices/tokendir", "directory with token<->user files")
	authBE   auth.Authenticator
	src      = rand.NewSource(time.Now().UnixNano())
)

/**************************************************
* helpers
***************************************************/
//https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
func RandomString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const (
		letterIdxBits = 6                    // 6 bits to represent a letter index
		letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
		letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits

	)

	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

// main

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
		authBE, err = NewPostgresAuthenticator()
		if err != nil {
			fmt.Println("Failed to create postgres authenticator", err)
			return err
		}
	} else if *backend == "file" {
		authBE, err = NewFileAuthenticator(*Tokendir)
		if err != nil {
			fmt.Println("Failed to create file authenticator", err)
			return err
		}
	} else if *backend == "none" {
		authBE = &NilAuthenticator{}
	} else if *backend == "any" {
		authBE = &AnyAuthenticator{}
	} else if *backend == "ldap" {
		authBE = &LdapAuthenticator{}
	} else if *backend == "psql-ldap" {
		authBE, err = NewLdapPsqlAuthenticator()
	} else {
		fmt.Printf("Invalid backend \"%s\"\n", *backend)
		os.Exit(10)
	}

	sd := server.NewServerDef()
	sd.Port = *port
	// we ARE the authentication service so don't insist on authenticated calls
	sd.NoAuth = true

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
type AuthServer struct{}

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
	if req.Token == "" {
		return nil, errors.New("Missing token")
	}
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

func (s *AuthServer) GetUserByToken(ctx context.Context, req *pb.VerifyRequest) (*pb.GetDetailResponse, error) {

	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	fmt.Printf("backend \"%s\" has been asked by \"%s\" to get user for token: \"%s\"\n", *backend, peer.Addr, req.Token)
	if req.Token == "" {
		return nil, errors.New("Missing token")
	}
	au, err := getUserFromToken(req.Token)
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

func (s *AuthServer) AuthenticatePassword(ctx context.Context, in *pb.AuthenticatePasswordRequest) (*pb.VerifyPasswordResponse, error) {
	tk := authBE.CreateVerifiedToken(in.Email, in.Password)
	if tk == "" {
		return nil, errors.New("Access Denied")
	}
	au, err := getUserFromToken(tk)
	fmt.Printf("Verified as user: %v (%s)\n", au, err)
	if err != nil {
		return nil, err
	}
	gd := pb.GetDetailResponse{UserID: au.ID,
		Email:     au.Email,
		FirstName: au.FirstName,
		LastName:  au.LastName,
	}
	r := pb.VerifyPasswordResponse{User: &gd, Token: tk}
	return &r, nil
}
func (s *AuthServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.GetDetailResponse, error) {
	if authBE == nil {
		return nil, errors.New("no authentication backend available")
	}
	if req.UserName == "" {
		return nil, errors.New("Username is required")
	}
	if req.Email == "" {
		return nil, errors.New("Email is required")
	}
	if req.FirstName == "" {
		return nil, errors.New("FirstName is required")
	}
	if req.LastName == "" {
		return nil, errors.New("LastName is required")
	}
	pw, err := authBE.CreateUser(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to create user %s: %s", req.UserName, err))
	}
	gdr := pb.GetDetailResponse{UserID: req.UserName,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Password:  pw,
	}

	return &gdr, nil
}
