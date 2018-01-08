package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"golang.conradwood.net/cmdline"
	pb "golang.conradwood.net/registrar/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"
)

var (
	cert               = []byte{1, 2, 3}
	displayedTokenInfo = false

	token     = flag.String("token", "user_token", "The authentication token (cookie) to authenticate with. May be name of a file in ~/.picoservices/tokens/, if so file contents shall be used as cookie")
	errorList []*errorCache
	errorLock sync.Mutex
)

type errorCache struct {
	servicename string
	lastOccured time.Time
	lastPrinted time.Time
}

func SaveToken(tk string) error {

	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Unable to get current user: %s\n", err)
		return err
	}
	cfgdir := fmt.Sprintf("%s/.picoservices/tokens", usr.HomeDir)
	fname := fmt.Sprintf("%s/%s", cfgdir, *token)
	if _, err := os.Stat(fname); !os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("File %s exists already", fname))
	}
	os.MkdirAll(cfgdir, 0600)
	fmt.Printf("Saving new token to %s\n", fname)
	err = ioutil.WriteFile(fname, []byte(tk), 0600)
	if err != nil {
		fmt.Printf("Failed to save token to %s: %s\n", fname, err)
	}
	return err
}

// opens a tcp connection to a gurupath.
func DialTCPWrapper(gurupath string) (net.Conn, error) {
	reg := cmdline.GetRegistryAddress()
	//fmt.Printf("Using registrar @%s\n", reg)
	opts := getDialopts()
	conn, err := grpc.Dial(reg, opts...)
	if err != nil {
		printError(gurupath, fmt.Sprintf("Error dialling registry %s @ %s\n", gurupath, reg))
		return nil, err
	}
	defer conn.Close()
	rcl := pb.NewRegistryClient(conn)
	gt := &pb.GetTargetRequest{Gurupath: gurupath, ApiType: pb.Apitype_tcp}
	lr, err := rcl.GetTarget(context.Background(), gt)
	if err != nil {
		s := fmt.Sprintf("Error getting TCP target for gurupath %s: %s", gurupath, err)
		printError(gurupath, s)
		return nil, errors.New(s)
	}
	if len(lr.Service) == 0 {
		s := fmt.Sprintf("No TCP target found for path %s", gurupath)
		printError(gurupath, s)
		return nil, errors.New(s)
	}
	svr := lr.Service[0]
	svl := svr.Location
	if len(svl.Address) == 0 {
		s := fmt.Sprintf("No TCP location found for path %s", gurupath)
		printError(gurupath, s)
		return nil, errors.New(s)
	}
	adr := svl.Address[0]
	tc, err := net.Dial("tcp", fmt.Sprintf("%s:%d", adr.Host, adr.Port))
	return tc, err
}

// given a service name we look up its address in the registry
// and return a connection to it.
// it's a replacement for the normal "dial" but instead of an address
// it takes a service name
func DialWrapper(servicename string) (*grpc.ClientConn, error) {
	reg := cmdline.GetRegistryAddress()
	//fmt.Printf("Using registrar @%s to dial %s (grpc)\n", reg, servicename)
	opts := getDialopts()
	conn, err := grpc.Dial(reg, opts...)
	if err != nil {
		printError(servicename, fmt.Sprintf("Error dialling servicename %s @ %s\n", servicename, reg))
		return nil, err
	}
	defer conn.Close()
	rcl := pb.NewRegistryClient(conn)
	gt := &pb.GetTargetRequest{Name: servicename, ApiType: pb.Apitype_grpc}
	lr, err := rcl.GetTarget(context.Background(), gt)
	if err != nil {
		printError(servicename, fmt.Sprintf("Error getting grpc service address %s: %s\n", servicename, err))
		return nil, err
	}
	if len(lr.Service) == 0 {
		s := fmt.Sprintf("No grpc target found for name %s", servicename)
		printError(servicename, s)
		return nil, errors.New(s)
	}
	svr := lr.Service[0]
	svl := svr.Location
	if len(svl.Address) == 0 {
		s := fmt.Sprintf("No grpc location found for name %s - is it running?", servicename)
		printError(servicename, s)
		return nil, errors.New(s)
	}
	sa := svl.Address[0]
	return DialService(sa)
}

func hasApi(ar []pb.Apitype, lf pb.Apitype) bool {
	for _, a := range ar {
		if a == lf {
			return true
		}
	}
	return false
}

// if one needs to, one can still connect explicitly to a service
// this call DOES NOT CLOSE THE CONNECTION
func DialService(sa *pb.ServiceAddress) (*grpc.ClientConn, error) {
	serverAddr := fmt.Sprintf("%s:%d", sa.Host, sa.Port)
	//fmt.Printf("Dialling service at \"%s\"\n", serverAddr)

	creds := GetClientCreds()
	cc, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))

	//	opts = []grpc.DialOption{grpc.WithInsecure()}
	// cc, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		fmt.Printf("Error dialling servicename @ %s\n", serverAddr)
		return nil, err
	}
	//defer cc.Close()

	return cc, nil
}

// get the Client Credentials we use to connect to other RPCs
func GetClientCreds() credentials.TransportCredentials {
	roots := x509.NewCertPool()
	FrontendCert := Certificate //ioutil.ReadFile(*clientcrt)
	roots.AppendCertsFromPEM(FrontendCert)
	ImCert := Ca //ioutil.ReadFile(*clientca)
	roots.AppendCertsFromPEM(ImCert)
	cert, err := tls.X509KeyPair(Certificate, Privatekey)
	//	cert, err := tls.LoadX509KeyPair(*clientcrt, *clientkey)
	if err != nil {
		fmt.Printf("Failed to create client certificates: %s\n", err)
		fmt.Printf("key:\n%s\n", string(Privatekey))
		return nil
	}
	// we don't verify the hostname because we use a dynamic registry thingie
	creds := credentials.NewTLS(&tls.Config{
		ServerName:         "*",
		Certificates:       []tls.Certificate{cert},
		RootCAs:            roots,
		InsecureSkipVerify: true,
	})
	return creds

}
func GetToken() string {
	var tok string
	var btok []byte
	var fname string
	fname = "n/a"
	usr, err := user.Current()
	if err == nil {
		fname = fmt.Sprintf("%s/.picoservices/tokens/%s", usr.HomeDir, *token)
		btok, _ = ioutil.ReadFile(fname)
	}
	if (err != nil) || (len(btok) == 0) {
		tok = *token
	} else {
		tok = string(btok)
		if displayedTokenInfo {
			fmt.Printf("Using token from %s\n", fname)
			displayedTokenInfo = true
		}
	}
	tok = strings.TrimSpace(tok)

	return tok
}

func SetAuthToken() context.Context {
	tok := GetToken()
	md := metadata.Pairs("token", tok,
		"clid", "itsme",
	)
	millis := 5000
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(millis)*time.Millisecond)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return ctx
}

func getErrorCacheByName(name string) *errorCache {
	errorLock.Lock()
	defer errorLock.Unlock()
	for _, ec := range errorList {
		if ec.servicename == name {
			return ec
		}
	}
	ec := &errorCache{servicename: name,
		lastOccured: time.Now(),
	}
	errorList = append(errorList, ec)
	return ec
}

func getDialopts() []grpc.DialOption {
	deadline := 2
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithTimeout(time.Duration(deadline) * time.Second)}
	return opts
}
func printError(path string, msg string) {
	e := getErrorCacheByName(path)
	if e == nil {
		fmt.Println(msg)
		return
	}
	if !e.needsPrinting() {
		return
	}
	fmt.Println(msg)
}

// returns true if this needs printing
// resets counter if it returns true
func (e *errorCache) needsPrinting() bool {
	now := time.Now()
	if now.Sub(e.lastPrinted) < (time.Duration(5) * time.Minute) {
		return false
	}
	e.lastPrinted = now
	return false
}
