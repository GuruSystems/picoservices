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
	"golang.conradwood.net/cmdline"
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
	serveraddr = flag.String("address", "", "Address (default: derive from connection to registrar. does not work well with localhost)")
	deploypath = flag.String("deployment_gurupath", "", "The deployment path by which other programs can refer to this deployment. expected is: a path of the format: \"namespace/groupname/repository/buildid\"")

	authconn         *grpc.ClientConn
	register_refresh = flag.Int("register_refresh", 10, "registration refresh interval in `seconds`")
	usercache        = make(map[string]*UserCache)
	ctrmetrics       = make(map[string]*uint64)
	registered       []*serverDef
	stopped          bool
	ticker           *time.Ticker
)

const (
	COOKIE_NAME = "Auth-Token"
)

type UserCache struct {
	UserID  string
	created time.Time
}

type Register func(server *grpc.Server) error

// no longer exported - please use NewServerDef instead
type serverDef struct {
	Port        int
	Certificate []byte
	Key         []byte
	CA          []byte
	Register    Register
	// set to true if this server does NOT require authentication (default: it does need authentication)
	NoAuth        bool
	name          string
	types         []pb.Apitype
	registered_id string
	DeployPath    string
}

func (s *serverDef) toString() string {
	return fmt.Sprintf("Port #%d: %s (%v)", s.Port, s.name, s.types)
}
func NewTCPServerDef(name string) *serverDef {
	sd := NewServerDef()
	sd.types = sd.types[:0]
	sd.types = append(sd.types, pb.Apitype_tcp)
	sd.name = name
	return sd
}

func NewHTMLServerDef(name string) *serverDef {
	sd := NewServerDef()
	sd.types = sd.types[:0]
	sd.types = append(sd.types, pb.Apitype_html)
	sd.name = name
	return sd
}

func NewServerDef() *serverDef {
	res := &serverDef{}
	res.registered_id = ""
	res.Key = Privatekey
	res.Certificate = Certificate
	res.CA = Ca
	res.DeployPath = *deploypath
	res.types = append(res.types, pb.Apitype_status)
	res.types = append(res.types, pb.Apitype_grpc)
	return res
}
func CheckCookie(cookie string) bool {
	return true
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

// given a request will return authinfo or nil
// if it fails to verify it, it'll be an error
// not having a cookie is not an error - it's nil auth
// having a cookie and it's bad is not an error - nil auth
// having a cookie and we cannot talk to the auth service is an error
func HttpAuthInterceptor(r *http.Request) (context.Context, error) {
	c, err := r.Cookie(COOKIE_NAME)
	if err != nil {
		// no cookie
		return nil, nil
	}
	// got cookie - auth it
	return authenticateToken(r.Context(), c.Value)
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
	if len(meta["token"]) != 1 {
		fmt.Println("RPCServer: Invalid number of tokens: ", len(meta["token"]))
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	token := meta["token"][0]
	return authenticateToken(ctx, token)
}
func authenticateToken(ctx context.Context, token string) (context.Context, error) {
	var err error
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

func stopping() {
	if stopped {
		return
	}
	fmt.Printf("Server shutdown - deregistering services\n")
	opts := []grpc.DialOption{grpc.WithInsecure()}
	rconn, err := grpc.Dial(cmdline.GetRegistryAddress(), opts...)
	if err != nil {
		fmt.Printf("failed to dial registry server: %v", err)
		return
	}
	defer rconn.Close()
	c := pb.NewRegistryClient(rconn)

	// value is a serverdef
	for _, sd := range registered {
		fmt.Printf("Deregistering Service \"%s\"\n", sd.toString())
		_, err := c.DeregisterService(context.Background(), &pb.DeregisterRequest{ServiceID: sd.registered_id})
		if err != nil {
			fmt.Printf("Failed to deregister Service \"%s\": %s\n", sd.toString(), err)
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
func ServerStartup(def *serverDef) error {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		stopping()
		os.Exit(0)
	}()
	stopped = false
	defer stopping()
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
	if len(grpcServer.GetServiceInfo()) > 1 {
		return fmt.Errorf("cannot register multiple(%d) names", len(grpcServer.GetServiceInfo()))
	}

	for name, _ := range grpcServer.GetServiceInfo() {
		def.name = name
	}
	AddRegistry(def)
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
func serveServiceInfo(w http.ResponseWriter, req *http.Request, sd *serverDef) {
	p := req.URL.Path
	if strings.HasPrefix(p, "/internal/service-info/name") {
		w.Write([]byte(sd.name))
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
func pleaseShutdown(w http.ResponseWriter, req *http.Request, sd *serverDef) {
	stopping()
	fmt.Fprintf(w, "OK\n")
	fmt.Printf("Received request to shutdown.\n")
	os.Exit(0)
}
func startHttpServe(sd *serverDef, grpcServer *grpc.Server) error {
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

func UnregisterPortRegistry(port []int) error {
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(cmdline.GetRegistryAddress(), opts...)
	if err != nil {
		fmt.Println("failed to dial registry server: %v", err)
		return err
	}
	defer conn.Close()
	client := pb.NewRegistryClient(conn)
	var ps []int32
	for _, p := range port {
		ps = append(ps, int32(p))
	}
	psr := pb.ProcessShutdownRequest{Port: ps}
	_, err = client.InformProcessShutdown(context.Background(), &psr)
	return err
}
func AddRegistry(sd *serverDef) (string, error) {
	// start period re-registration
	if ticker == nil {
		ticker = time.NewTicker(time.Duration(*register_refresh) * time.Second)
		go func() {
			for _ = range ticker.C {
				reRegister()
			}
		}()
	}

	//fmt.Printf("Registering service %s with registry server\n", name)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(cmdline.GetRegistryAddress(), opts...)
	if err != nil {
		fmt.Println("failed to dial registry server: %v", err)
		return "", err
	}
	defer conn.Close()
	client := pb.NewRegistryClient(conn)
	req := pb.ServiceLocation{}
	req.Service = &pb.ServiceDescription{}
	req.Service.Name = sd.name
	req.Service.Gurupath = sd.DeployPath
	req.Address = []*pb.ServiceAddress{{Port: int32(sd.Port)}}
	if *serveraddr != "" {
		req.Address[0].Host = *serveraddr
	}

	// all addresses get the same apitype
	for _, svcadr := range req.Address {
		for _, apitype := range sd.types {
			svcadr.ApiType = append(svcadr.ApiType, apitype)
		}
	}

	resp, err := client.RegisterService(context.Background(), &req)
	if err != nil {
		fmt.Printf("RegisterService(%s) failed: %s\n", req.Service.Name, err)
		fmt.Printf("  Published address: \"%s\"\n", req.Address[0].Host)
		fmt.Printf("  Registry:   %s\n", cmdline.GetRegistryAddress())
		return "", err
	}
	if resp == nil {
		fmt.Println("Registration failed with no error provided.")
	}
	if sd.registered_id == "" {
		registered = append(registered, sd)
	}
	sd.registered_id = resp.ServiceID
	//fmt.Printf("Response to register service: %v\n", resp)
	return resp.ServiceID, nil
}

func reRegister() {
	for _, sd := range registered {
		AddRegistry(sd)
	}

}

// expose an ever-increasing counter with the given metric
func ExposeMetricCounter(name string, value *uint64) error {
	ctrmetrics[name] = value
	return nil
}
