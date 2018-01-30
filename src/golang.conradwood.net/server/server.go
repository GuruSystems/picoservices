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
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.conradwood.net/cmdline"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

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

	register_refresh = flag.Int("register_refresh", 10, "registration refresh interval in `seconds`")
	usercache        = make(map[string]*UserCache)
	serverDefs       = make(map[string]*serverDef)
	registered       []*serverDef // is registered
	knownServices    []*serverDef // all services, even not known ones
	stopped          bool
	ticker           *time.Ticker
	promHandler      http.Handler
)

func init() {

}

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
	NoAuth               bool
	name                 string
	types                []pb.Apitype
	registered_id        string
	DeployPath           string
	grpc_server_requests *prometheus.CounterVec
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
	/*
		res.Key = Privatekey
		res.Certificate = Certificate
		res.CA = Ca
	*/
	res.DeployPath = *deploypath
	res.types = append(res.types, pb.Apitype_status)
	res.types = append(res.types, pb.Apitype_grpc)
	return res
}

/*
func CheckCookie(cookie string) bool {
	return true
}
*/
func StreamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return grpc.Errorf(codes.Unauthenticated, "stream authentication is not yet implemented")
}

// we authenticate a client here
func UnaryAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, grpc.Errorf(codes.Unauthenticated, "missing context metadata")
	}

	name := ServiceNameFromInfo(info)
	def := getServerDefByName(name)
	if def == nil {
		s := fmt.Sprintf("Service not registered! %s", name)
		fmt.Println(s)
		return nil, errors.New(s)
	}
	method := MethodNameFromInfo(info)
	//fmt.Printf("Method: \"%s\"\n", method)

	def.grpc_server_requests.With(prometheus.Labels{
		"method":      method,
		"servicename": def.name,
	}).Inc()
	nctx, err := authenticate(ctx, meta)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("intercepted and failed call to %v: %s", req, err))
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

		// we postpone connection to authserver until we need it
		// d'oh.
		/*
			// set up a connection to our authentication service
			authconn, err = client.DialWrapper("auth.AuthenticationService")
			if err != nil {
				return fmt.Errorf("Failed to connect to authserver")
			}
		*/

	}

	grpc.EnableTracing = true
	// callback to the callers' specific intialisation
	// (set by the caller of this function)
	def.Register(grpcServer)
	if err != nil {
		fmt.Printf("Serverstartup: failed to register service on startup: %s\n", err)
		return fmt.Errorf("grpc register error: %s", err)
	}
	if len(grpcServer.GetServiceInfo()) > 1 {
		return fmt.Errorf("cannot register multiple(%d) names", len(grpcServer.GetServiceInfo()))
	}
	for name, _ := range grpcServer.GetServiceInfo() {
		def.name = name
	}
	if def.name == "" {
		fmt.Println("Got no server name!")
		return errors.New("Missing servername")
	}
	serverDefs[def.name] = def
	// hook up prometheus
	def.grpc_server_requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_received",
			Help: "requests to log stuff received",
		},
		[]string{"servicename", "method"},
	)
	err = prometheus.Register(def.grpc_server_requests)
	if err != nil {
		s := fmt.Sprintf("Failed to register reqCounter: %s\n", err)
		fmt.Println(s)
		return errors.New(s)
	}

	knownServices = append(knownServices, def)

	fmt.Printf("Adding service to registry...\n")
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
	} else if strings.HasPrefix(p, "/internal/service-info/build") {
		w.Write([]byte(fmt.Sprintf("buildid: %d\nBuild_timestamp:%d\nBuild_date_string:%s\n",
			cmdline.Buildnumber, cmdline.Build_date, cmdline.Build_date_string)))
	} else if strings.HasPrefix(p, "/internal/service-info/metrics") {
		fmt.Printf("Request path: \"%s\"\n", p)
		m := strings.TrimPrefix(p, "/internal/service-info/metrics")
		m = strings.TrimLeft(m, "/")
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
	mux.Handle("/internal/service-info/metrics", promhttp.Handler())
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
	ra := cmdline.GetRegistryAddress()
	conn, err := grpc.Dial(ra, opts...)
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
		fmt.Printf("RegisterService(%s) failed (registry=%s,addr=%s): %s\n", req.Service.Name, ra, req.Address[0].Host, err)
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
	// register any that are not yet registered
	for _, sd := range knownServices {
		if sd.registered_id != "" {
			continue
		}
		AddRegistry(sd)
	}
	// reregister any that are registered already
	for _, sd := range registered {
		AddRegistry(sd)
	}

}

func getServerDefByName(name string) *serverDef {
	return serverDefs[name]
}
func MethodNameFromInfo(info *grpc.UnaryServerInfo) string {
	full := info.FullMethod
	if full[0] == '/' {
		full = full[1:]
	}
	ns := strings.SplitN(full, "/", 2)
	if len(ns) < 2 {
		return ""
	}
	res := ns[1]
	if res[0] == '/' {
		res = res[1:]
	}
	return ns[1]
}
func ServiceNameFromInfo(info *grpc.UnaryServerInfo) string {
	full := info.FullMethod
	if full[0] == '/' {
		full = full[1:]
	}
	ns := strings.SplitN(full, "/", 2)
	return ns[0]
}

// expose an ever-increasing counter with the given metric
// Deprecation: We switched to prometheus
func exposeMetricCounter(name string, value *uint64) error {
	return nil
}
func targetName(name string) string {
	x := strings.Split(name, ".")
	return x[0]
}
