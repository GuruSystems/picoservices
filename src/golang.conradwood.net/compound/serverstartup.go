package compound

import (
	"flag"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"time"
	//	"github.com/golang/protobuf/proto"
	"crypto/tls"
	"crypto/x509"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	//	"google.golang.org/grpc/peer"
	"golang.conradwood.net/auth"
	apb "golang.conradwood.net/auth/proto"
	pb "golang.conradwood.net/registrar/proto"
	"google.golang.org/grpc/codes"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
)

var (
	servercrt        = flag.String("rpc_server_certificate", "/etc/grpc/server/certificate.pem", "filename of the server certificate to be used for incoming connections to this rpc server")
	servercertkey    = flag.String("rpc_server_certkey", "/etc/grpc/server/privatekey.pem", "the key for the server certificate to be used for incoming connections to this rpc server")
	serverca         = flag.String("rpc_server_ca", "/etc/grpc/server/ca.pem", "filename of the the CA certificate which signed both client and server certificate")
	Registry         = flag.String("registry", "localhost:5000", "Registrar server address")
	serveraddr       = flag.String("address", "", "Address (default: derive from connection to registrar. does not work well with localhost)")
	authconn         *grpc.ClientConn
	register_refresh = flag.Int("register_refresh", 10, "registration refresh interval in seconds")
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
	names  []string
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
	nctx, err := authenticate(ctx, meta)
	if err != nil {
		return nil, err
	}
	return handler(nctx, req)
}

// we must not return useful errormessages here,
// so we print them to stdout instead and return a generic message
func authenticate(ctx context.Context, meta metadata.MD) (context.Context, error) {
	if len(meta["token"]) != 1 {
		fmt.Println("Invalid number of tokens: ", len(meta["token"]))
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	token := meta["token"][0]
	if authconn == nil {
		fmt.Println("No authenticator available")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	client := apb.NewAuthenticationServiceClient(authconn)
	req := &apb.VerifyRequest{Token: token}
	resp, err := client.VerifyUserToken(ctx, req)
	if err != nil {
		return nil, err
	}
	// should never happen - but it's auth, so extra check doesn't hurt
	if resp.UserID == "" {
		fmt.Println("BUG: a user was authenticated but no userid returned!")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	ai := auth.AuthInfo{UserID: resp.UserID}
	fmt.Printf("Authenticated user \"%s\".\n", resp.UserID)
	nctx := context.WithValue(ctx, "authinfo", ai)
	return nctx, nil
}
func GetUserID(ctx context.Context) auth.AuthInfo {
	ai := ctx.Value("authinfo").(auth.AuthInfo)
	return ai
}
func GetAuthClient() (apb.AuthenticationServiceClient, error) {
	if authconn == nil {
		fmt.Println("No authenticator available")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	client := apb.NewAuthenticationServiceClient(authconn)
	return client, nil
}

func registerMe(def ServerDef) error {
	for _, name := range def.names {
		//fmt.Println("Registered Server: ", name)
		err := AddRegistry(name, def.Port)
		if err != nil {
			return fmt.Errorf("Failed to register %s with registry server", name, err)
		}
	}
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
	var grpcServer *grpc.Server
	if def.NoAuth {
		grpcServer = grpc.NewServer(grpc.Creds(creds))
	} else {
		// Create the gRPC server with the credentials
		grpcServer = grpc.NewServer(grpc.Creds(creds),
			grpc.UnaryInterceptor(UnaryAuthInterceptor),
			grpc.StreamInterceptor(StreamAuthInterceptor),
		)

		// set up a connection to our authentication service
		authconn, err = DialWrapper("auth.AuthenticationService")
		if err != nil {
			return fmt.Errorf("Failed to connect to authserver")
		}
	}

	grpc.EnableTracing = true
	def.Register(grpcServer)
	if err != nil {
		return fmt.Errorf("grpc register error: %s", err)
	}
	for name, _ := range grpcServer.GetServiceInfo() {
		def.names = append(def.names, name)
	}

	// start period re-registration
	registerMe(def)
	ticker := time.NewTicker(time.Duration(*register_refresh) * time.Second)
	go func() {
		for _ = range ticker.C {
			registerMe(def)
		}
	}()
	// something odd?
	reflection.Register(grpcServer)
	// Serve and Listen
	err = startHttpServe(def, grpcServer)

	// Create the channel to listen on

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %s", listenAddr, err)
	}
	err = grpcServer.Serve(lis)
	if err != nil {
		return fmt.Errorf("grpc serve error: %s", err)
	}
	return nil
}

// this services the /service-info/ url
func serveServiceInfo(w http.ResponseWriter, req *http.Request, sd ServerDef) {
	for _, name := range sd.names {
		w.Write([]byte(name))
	}
}

// this services the /pleaseshutdown url
func pleaseShutdown(w http.ResponseWriter, req *http.Request, sd ServerDef) {
	os.Exit(0)
}
func startHttpServe(sd ServerDef, grpcServer *grpc.Server) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/service-info/", func(w http.ResponseWriter, req *http.Request) {
		serveServiceInfo(w, req, sd)
	})
	mux.HandleFunc("/internal/pleaseshutdown", func(w http.ResponseWriter, req *http.Request) {
		pleaseShutdown(w, req, sd)
	})
	gwmux := runtime.NewServeMux()
	mux.Handle("/", gwmux)
	serveSwagger(mux)

	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", sd.Port))
	if err != nil {
		panic(err)
	}

	BackendCert, err := ioutil.ReadFile(sd.Certificate)
	if err != nil {
		return fmt.Errorf("Failed to read certificate from file \"%s\": %s", sd.Certificate, err)
	}
	BackendKey, err := ioutil.ReadFile(sd.Key)
	if err != nil {
		return fmt.Errorf("Failed to read key from file \"%s\": %s", sd.Key, err)
	}
	cert, err := tls.X509KeyPair(BackendCert, BackendKey)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", sd.Port),
		Handler: grpcHandlerFunc(grpcServer, mux),
		TLSConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			NextProtos:         []string{"h2"},
			InsecureSkipVerify: true,
		},
	}

	fmt.Printf("grpc on port: %d\n", sd.Port)
	err = srv.Serve(tls.NewListener(conn, srv.TLSConfig))
	return err
}
func serveSwagger(mux *http.ServeMux) {
	//fmt.Println("serverSwagger??", mux)
}

// this function is called by http and works out wether it's a grpc or http-serve request
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/internal/") {
			otherHandler.ServeHTTP(w, r)
		} else {
			fmt.Println("Req: ", path)
			grpcServer.ServeHTTP(w, r)
		}
	})
}

func AddRegistry(name string, port int) error {
	//fmt.Printf("Registering service %s with registry server\n", name)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(*Registry, opts...)
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
	if *serveraddr != "" {
		req.Address[0].Host = *serveraddr
	}
	resp, err := client.RegisterService(context.Background(), &req)
	if err != nil {
		fmt.Println("failed to register services:", err)
		return err
	}
	if resp == nil {
		fmt.Println("Registration failed with no error provided.")
	}
	//fmt.Printf("Response to register service: %v\n", resp)
	return nil
}
