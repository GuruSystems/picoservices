package server

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
	"golang.conradwood.net/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	//	"google.golang.org/grpc/peer"
	"golang.conradwood.net/auth"
	apb "golang.conradwood.net/auth/proto"
	pb "golang.conradwood.net/registrar/proto"
	"google.golang.org/grpc/codes"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	/*
		servercrt        = flag.String("rpc_server_certificate", "/etc/grpc/server/certificate.pem", "`filename` of the server certificate to be used for incoming connections to this rpc server")
		servercertkey    = flag.String("rpc_server_certkey", "/etc/grpc/server/privatekey.pem", "`filename` of the key for the server certificate to be used for incoming connections to this rpc server")
		serverca         = flag.String("rpc_server_ca", "/etc/grpc/server/ca.pem", "`filename` of the the CA certificate which signed both client and server certificate")
	*/
	registry         = flag.String("registry", "localhost:5000", "Registrar server address (to register with)")
	serveraddr       = flag.String("address", "", "Address (default: derive from connection to registrar. does not work well with localhost)")
	authconn         *grpc.ClientConn
	register_refresh = flag.Int("register_refresh", 10, "registration refresh interval in `seconds`")
	usercache        = make(map[string]*UserCache)
	ctrmetrics       = make(map[string]*uint64)
	registered       = make(map[string]string)
	stopped          bool
)

type UserCache struct {
	UserID  string
	created time.Time
}

type Register func(server *grpc.Server) error

type ServerDef struct {
	Port        int
	Certificate []byte
	Key         []byte
	CA          []byte
	Register    Register
	// set to true if this server does NOT require authentication (default: it does need authentication)
	NoAuth bool
	names  []string
}

func CheckCookie(cookie string) bool {
	return true
}

func (s *ServerDef) init() {
	if len(s.Certificate) == 0 {
		s.Certificate = Certificate
	}
	if len(s.Key) == 0 {
		s.Key = Privatekey
	}
	if len(s.CA) == 0 {
		s.CA = Ca
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

// return userid
func getUserFromCache(token string) string {
	uc := usercache[token]
	if uc == nil {
		return ""
	}
	if time.Since(uc.created) > (time.Minute * 5) {
		return ""
	}
	return uc.UserID

}
func addUserToCache(token string, id string) {
	uc := UserCache{UserID: id, created: time.Now()}
	usercache[token] = &uc
}

// we must not return useful errormessages here,
// so we print them to stdout instead and return a generic message
func authenticate(ctx context.Context, meta metadata.MD) (context.Context, error) {
	var err error
	if len(meta["token"]) != 1 {
		fmt.Println("RPCServer: Invalid number of tokens: ", len(meta["token"]))
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	token := meta["token"][0]
	if authconn == nil {
		authconn, err = client.DialWrapper("auth.AuthenticationService")
		if err != nil {
			fmt.Printf("Could not establish connection to auth service:%s\n", err)
			return nil, err
		}
	}
	uc := getUserFromCache(token)
	if uc != "" {
		ai := auth.AuthInfo{UserID: uc}
		nctx := context.WithValue(ctx, "authinfo", ai)
		return nctx, nil
	}
	authc := apb.NewAuthenticationServiceClient(authconn)
	req := &apb.VerifyRequest{Token: token}
	repeat := 4
	var resp *apb.VerifyResponse
	for {
		resp, err = authc.VerifyUserToken(ctx, req)
		if err == nil {
			break
		}

		fmt.Printf("(%d) VerifyUserToken() failed: %s (%v)\n", repeat, err, authconn)
		if repeat <= 1 {
			return nil, err
		}

		fmt.Printf("Due to failure (%s) verifying token we re-connect...\n", err)
		authconn, err = client.DialWrapper("auth.AuthenticationService")
		if err != nil {
			fmt.Printf("Resetting the connection to auth service did not help either:%s\n", err)
			return nil, err
		}
		authc = apb.NewAuthenticationServiceClient(authconn)

		repeat--
	}
	// should never happen - but it's auth, so extra check doesn't hurt
	if (resp == nil) || (resp.UserID == "") {
		fmt.Println("RPCServer: BUG: a user was authenticated but no userid returned!")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	addUserToCache(token, resp.UserID)
	ai := auth.AuthInfo{UserID: resp.UserID}
	fmt.Printf("RPCServer: Authenticated user \"%s\".\n", resp.UserID)
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
		if registered[name] == "" {
			fmt.Printf("Registering Service: \"%s\"\n", name)
		}
		id, err := AddRegistry(name, def.Port)
		if err != nil {
			return fmt.Errorf("Failed to register %s with registry server", name, err)
		}
		registered[name] = id
	}
	return nil

}

func stopping() {
	if stopped {
		return
	}
	fmt.Printf("Server shutdown - deregistering services\n")
	opts := []grpc.DialOption{grpc.WithInsecure()}
	rconn, err := grpc.Dial(GetRegistryAddress(), opts...)
	if err != nil {
		fmt.Printf("failed to dial registry server: %v", err)
		return
	}
	defer rconn.Close()
	c := pb.NewRegistryClient(rconn)

	for key, value := range registered {
		fmt.Printf("Deregistering Service \"%s\" => %s\n", key, value)
		_, err := c.DeregisterService(context.Background(), &pb.DeregisterRequest{ServiceID: value})
		if err != nil {
			fmt.Printf("Failed to deregister Service \"%s\" => %s: %s\n", key, value, err)
		}
	}
	stopped = true
}

// this is our typical gRPC server startup
// it sets ourselves up with our own certificates
// which is set for THIS SERVER, so installed/maintained
// together with the server (rather than as part of this software)
// it also configures the rpc server to expect a token to identify
// the user in the rpc metadata call
func ServerStartup(def ServerDef) error {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		stopping()
		os.Exit(0)
	}()
	stopped = false
	defer stopping()
	def.init()
	listenAddr := fmt.Sprintf(":%d", def.Port)
	fmt.Println("Starting server on ", listenAddr)

	BackendCert := Certificate
	BackendKey := Privatekey
	ImCert := Ca
	cert, err := tls.X509KeyPair(BackendCert, BackendKey)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %v\n", err)
	}
	roots := x509.NewCertPool()
	FrontendCert := Certificate
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
		authconn, err = client.DialWrapper("auth.AuthenticationService")
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
	p := req.URL.Path
	if strings.HasPrefix(p, "/internal/service-info/name") {
		for _, name := range sd.names {
			w.Write([]byte(name))
		}
	} else if strings.HasPrefix(p, "/internal/service-info/metrics") {
		fmt.Printf("Request path: \"%s\"\n", p)
		m := strings.TrimPrefix(p, "/internal/service-info/metrics")
		m = strings.TrimLeft(m, "/")
		up := ctrmetrics[m]
		if up == nil {
			fmt.Printf("Metric request for unknown metric: \"%s\"\n", m)
			return
		}
		fmt.Fprintf(w, "%d", *up)
	} else {
		fmt.Printf("Invalid path: \"%s\"\n")
	}
}

// this services the /pleaseshutdown url
func pleaseShutdown(w http.ResponseWriter, req *http.Request, sd ServerDef) {
	stopping()
	fmt.Fprintf(w, "OK\n")
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

	BackendCert := Certificate
	BackendKey := Privatekey
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
			//fmt.Println("Req: ", path)
			grpcServer.ServeHTTP(w, r)
		}
	})
}

func GetRegistryAddress() string {
	res := *registry
	if !strings.Contains(res, ":") {
		res = fmt.Sprintf("%s:5000", res)
	}
	return res
}
func AddRegistry(name string, port int) (string, error) {
	//fmt.Printf("Registering service %s with registry server\n", name)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(GetRegistryAddress(), opts...)
	if err != nil {
		fmt.Println("failed to dial registry server: %v", err)
		return "", err
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
		fmt.Printf("RegisterService(%s) failed: %s\n", req.Service.Name, err)
		fmt.Printf("  Published address: \"%s\"\n", req.Address[0].Host)
		fmt.Printf("  Registry:   %s\n", GetRegistryAddress())
		return "", err
	}
	if resp == nil {
		fmt.Println("Registration failed with no error provided.")
	}
	//fmt.Printf("Response to register service: %v\n", resp)
	return resp.ServiceID, nil
}

// expose an ever-increasing counter with the given metric
func ExposeMetricCounter(name string, value *uint64) error {
	ctrmetrics[name] = value
	return nil
}
