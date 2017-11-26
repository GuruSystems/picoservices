package client

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	pb "golang.conradwood.net/registrar/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"os/user"
	"strings"
)

var (
	cert               = []byte{1, 2, 3}
	displayedTokenInfo = false
	Registry           = flag.String("registrar", "localhost:5000", "address of the registrar server (for lookups)")
	/*
		clientcrt          = flag.String("clientcert", "/etc/cnw/certs/rfc-client/certificate.pem", "Client certificate")
		clientkey          = flag.String("clientkey", "/etc/cnw/certs/rfc-client/privatekey.pem", "client private key")
		clientca           = flag.String("clientca", "/etc/cnw/certs/rfc-client/ca.pem", "Certificate Authority")
	*/
	token = flag.String("token", "user_token", "The authentication token (cookie) to authenticate with. May be name of a file in ~/.picoservices/tokens/, if so file contents shall be used as cookie")
)

// given a service name we look up its address in the registry
// and return a connection to it.
// it's a replacement for the normal "dial" but instead of an address
// it takes a service name
func DialWrapper(servicename string) (*grpc.ClientConn, error) {
	fmt.Printf("Using registrar @%s\n", *Registry)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(*Registry, opts...)
	if err != nil {
		fmt.Printf("Error dialling servicename %s @ %s\n", servicename, Registry)
		return nil, err
	}
	defer conn.Close()
	client := pb.NewRegistryClient(conn)
	req := pb.GetRequest{}
	req.Service = &pb.ServiceDescription{Name: servicename}
	resp, err := client.GetServiceAddress(context.Background(), &req)
	if err != nil {
		fmt.Printf("Error getting service address %s: %s\n", servicename, err)
		return nil, err
	}
	if (resp.Location == nil) || (len(resp.Location.Address) == 0) {
		fmt.Printf("Received no address for service \"%s\" - is it running?\n", servicename)
		return nil, errors.New("no address for service")
	}
	sa := resp.Location.Address[0]
	serverAddr := fmt.Sprintf("%s:%d", sa.Host, sa.Port)
	fmt.Printf("Dialling service \"%s\" at \"%s\"\n", servicename, serverAddr)

	creds := GetClientCreds()
	cc, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))

	//	opts = []grpc.DialOption{grpc.WithInsecure()}
	// cc, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		fmt.Printf("Error dialling servicename %s @ %s\n", servicename, serverAddr)
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

func SetAuthToken() context.Context {
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
	md := metadata.Pairs("token", tok,
		"clid", "itsme",
	)

	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return ctx
}
