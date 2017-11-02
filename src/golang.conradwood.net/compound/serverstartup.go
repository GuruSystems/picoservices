package compound

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
	pb "golang.conradwood.net/registrar/proto"
	"google.golang.org/grpc/codes"

	"io/ioutil"
	"net"
)

var (
	servercrt     = flag.String("certificate", "/etc/grpc/server/certificate.pem", "filename of the server certificate")
	servercertkey = flag.String("certkey", "/etc/grpc/server/privatekey.pem", "the key for the server certificate")
	serverca      = flag.String("ca", "/etc/grpc/server/ca.pem", "filename of the the CA certificate which signed both client and server certificate")
	registry      = flag.String("registry", "localhost:5000", "Registry server address")
	auth          Authenticator
)

type Register func(server *grpc.Server) error

type ServerDef struct {
	Port        int
	Certificate string
	Key         string
	CA          string
	Register    Register
	// set to true if this server does NOT require authentication (default: it does need authentication)
	NoAuth bool
}

func CheckCookie(cookie string) bool {
	return true
}

func (s *ServerDef) init() {
	if s.Certificate == "" {
		s.Certificate = *servercrt
	}
	if s.Key == "" {
		s.Key = *servercertkey
	}
	if s.CA == "" {
		s.CA = *serverca
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

// we must not return useful errormessages here,
// so we print them to stdout instead and return a generic message
func authenticate(meta metadata.MD) error {
	if len(meta["token"]) != 1 {
		fmt.Println("Invalid number of tokens: ", len(meta["token"]))
		return grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	token := meta["token"][0]
	if auth == nil {
		fmt.Println("No authenticator available")
		return grpc.Errorf(codes.Unauthenticated, "invalid token")
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
	var srv *grpc.Server
	if def.NoAuth {
		srv = grpc.NewServer(grpc.Creds(creds))
	} else {
		// Create the gRPC server with the credentials
		srv = grpc.NewServer(grpc.Creds(creds),
			grpc.UnaryInterceptor(UnaryAuthInterceptor),
			grpc.StreamInterceptor(StreamAuthInterceptor),
		)
		// add an authenticator
		auth, err = NewPostgresAuthenticator(*dbhost, *dbdb, *dbuser, *dbpw)
		if err != nil {
			return fmt.Errorf("Failed to init authenticator: %s", err)
		}
	}

	grpc.EnableTracing = true
	def.Register(srv)
	if err != nil {
		return fmt.Errorf("grpc register error: %s", err)
	}
	for name, _ := range srv.GetServiceInfo() {
		fmt.Println("Registered Server: ", name)
		err = AddRegistry(name, def.Port)
		if err != nil {
			return fmt.Errorf("Failed to register %s with registry server", name, err)
		}
	}
	// something odd?
	reflection.Register(srv)
	// Serve and Listen
	err = srv.Serve(lis)
	if err != nil {
		return fmt.Errorf("grpc serve error: %s", err)
	}
	return nil
}

func AddRegistry(name string, port int) error {
	fmt.Printf("Registering service %s with registry server\n", name)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(*registry, opts...)
	if err != nil {
		fmt.Println("failed to dial registry server: %v", err)
		return err
	}
	defer conn.Close()
	client := pb.NewRegistryClient(conn)
	req := pb.ServiceLocation{}
	req.Service = &pb.ServiceDescription{}
	req.Service.Name = name
	req.Address = []*pb.ServiceAddress{{Port: int32(port)}}
	resp, err := client.RegisterService(context.Background(), &req)
	if err != nil {
		fmt.Println("failed to register services:", err)
		return err
	}
	fmt.Printf("Response to register service: %v\n", resp)
	return nil
}
