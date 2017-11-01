package common

import (
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	//	"github.com/golang/protobuf/proto"
	"crypto/tls"
	"crypto/x509"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	//	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/codes"
	"io/ioutil"
	"net"
)

var (
	crt    = flag.String("certificate", "/etc/grpc/server/certificate.pem", "filename of the server certificate")
	key    = flag.String("key", "/etc/grpc/server/privatekey.pem", "the key for the server certificate")
	ca     = flag.String("ca", "/etc/grpc/server/ca.pem", "filename of the the CA certificate which signed both client and server certificate")
	dbhost = flag.String("dbhost", "postgres", "hostname of the postgres database rdms")
	dbdb   = flag.String("database", "rpcusers", "database to use for authentication")
	dbuser = flag.String("dbuser", "root", "username for the database to use for authentication")
	dbpw   = flag.String("dbpw", "pw", "password for the database to use for authentication")

	auth Authenticator
)

type Register func(server *grpc.Server) error

type ServerDef struct {
	Port        int
	Certificate string
	Key         string
	CA          string
	Register    Register
}

func CheckCookie(cookie string) bool {
	return true
}

func (s *ServerDef) init() {
	if s.Certificate == "" {
		s.Certificate = *crt
	}
	if s.Key == "" {
		s.Key = *key
	}
	if s.CA == "" {
		s.CA = *ca
	}
}
func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return grpc.Errorf(codes.Unauthenticated, "stream authentication is not yet implemented")
}

// we authenticate a client here
func UnaryAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpc.Errorf(codes.Unauthenticated, "missing context metadata")
	}
	err := authenticate(meta)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func authenticate(meta metadata.MD) error {
	if len(meta["token"]) != 1 {
		return grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	token := meta["token"][0]
	if auth == nil {
		return grpc.Errorf(codes.Unauthenticated, "No authenticator enabled (in server)")
	}
	user, err := auth.Authenticate(token)
	if err != nil {
		return err
	}
	fmt.Printf("Authenticated user \"%s\".\n", user)
	return nil
}

// this is our typical gRPC server startup
// it sets ourselves up with our own certificates
// which is set for THIS SERVER, so installed/maintained
// together with the server (rather than as part of this software)
// it also configures the rpc server to expect a token to identify
// the user in the rpc metadata call
func ServerStartup(def ServerDef) error {
	def.init()
	listenAddr := fmt.Sprintf(":%d", def.Port)
	fmt.Println("Starting server on ", listenAddr)

	// Create the channel to listen on
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %s", listenAddr, err)
	}
	BackendCert, err := ioutil.ReadFile(def.Certificate)
	if err != nil {
		return fmt.Errorf("Failed to read certificate from file \"%s\": %s", def.Certificate, err)
	}
	BackendKey, err := ioutil.ReadFile(def.Key)
	if err != nil {
		return fmt.Errorf("Failed to read key from file \"%s\": %s", def.Key, err)
	}
	ImCert, err := ioutil.ReadFile(def.CA)
	if err != nil {
		return fmt.Errorf("Failed to read CA certificate from file \"%s\": %s", def.CA, err)
	}

	cert, err := tls.X509KeyPair(BackendCert, BackendKey)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %v\n", err)
	}
	roots := x509.NewCertPool()
	FrontendCert, _ := ioutil.ReadFile(def.Certificate)
	roots.AppendCertsFromPEM(FrontendCert)
	roots.AppendCertsFromPEM(ImCert)

	creds := credentials.NewServerTLSFromCert(&cert)

	// Create the gRPC server with the credentials
	srv := grpc.NewServer(grpc.Creds(creds),
		grpc.UnaryInterceptor(UnaryAuthInterceptor),
		grpc.StreamInterceptor(StreamAuthInterceptor),
	)
	//srv := grpc.NewServer()
	grpc.EnableTracing = true
	def.Register(srv)
	if err != nil {
		return fmt.Errorf("grpc register error: %s", err)
	}
	for name, _ := range srv.GetServiceInfo() {
		fmt.Println("Registered Server: ", name)
	}
	// something odd?
	reflection.Register(srv)
	// add an authenticator
	auth, err = NewPostgresAuthenticator(*dbhost, *dbdb, *dbuser, *dbpw)
	if err != nil {
		return fmt.Errorf("Failed to init authenticator: %s", err)
	} // Serve and Listen
	err = srv.Serve(lis)
	if err != nil {
		return fmt.Errorf("grpc serve error: %s", err)
	}
	return nil
}
