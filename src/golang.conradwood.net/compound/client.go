package compound

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
)

var (
	//	Registry  = flag.String("registrar", "localhost:5000", "address of the registrar server (for lookups)")
	clientcrt = flag.String("clientcert", "/etc/cnw/certs/rfc-client/certificate.pem", "Client certificate")
	clientkey = flag.String("clientkey", "/etc/cnw/certs/rfc-client/privatekey.pem", "client private key")
	clientca  = flag.String("clientca", "/etc/cnw/certs/rfc-client/ca.pem", "Certificate Authority")
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

	roots := x509.NewCertPool()
	FrontendCert, _ := ioutil.ReadFile(*clientcrt)
	roots.AppendCertsFromPEM(FrontendCert)
	ImCert, _ := ioutil.ReadFile(*clientca)
	roots.AppendCertsFromPEM(ImCert)

	cert, err := tls.LoadX509KeyPair(*clientcrt, *clientkey)

	// we don't verify the hostname because we use a dynamic registry thingie
	creds := credentials.NewTLS(&tls.Config{
		ServerName:         serverAddr,
		Certificates:       []tls.Certificate{cert},
		RootCAs:            roots,
		InsecureSkipVerify: true,
	})

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

func SetAuthToken(token string) context.Context {
	md := metadata.Pairs("token", "valid-token",
		"clid", "itsme",
	)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return ctx
}
