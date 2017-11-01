package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	"crypto/x509"
	pb "golang.conradwood.net/auth/proto"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
)

// static variables for flag parser
var (
	serverAddr = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
	crt        = "/etc/cnw/certs/rfc-client/certificate.pem"
	key        = "/etc/cnw/certs/rfc-client/privatekey.pem"
	ca         = "/etc/cnw/certs/rfc-client/ca.pem"
)

func main() {
	flag.Parse()
	//creds, err := credentials.NewClientTLSFromFile(crt, "")
	/*
		//certificate, err := tls.LoadX509KeyPair(crt, key)
		if err != nil {
			fmt.Println("could not load client key pair: %s", err)
			return
		}
	*/
	/*	creds := credentials.NewTLS(&tls.Config{
			ServerName:   *serverAddr, // NOTE: this is required!
			Certificates: []tls.Certificate{certificate},
		})
	*/

	roots := x509.NewCertPool()
	FrontendCert, _ := ioutil.ReadFile(crt)
	roots.AppendCertsFromPEM(FrontendCert)
	ImCert, _ := ioutil.ReadFile(ca)
	roots.AppendCertsFromPEM(ImCert)

	// Create credentials
	creds := credentials.NewClientTLSFromCert(roots, "")

	fmt.Println("Connecting to server...", *serverAddr, creds)
	//conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure())
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println("fail to dial: %v", err)
		return
	}
	defer conn.Close()
	fmt.Println("Creating client...")
	client := pb.NewRFCManagerClient(conn)
	req := pb.CreateRequest{Name: "clientvpn", Access: "testaccess"}
	md := metadata.Pairs("token", "valid-token")
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	fmt.Println("RPC call...")
	resp, err := client.CreateRFC(ctx, &req)
	if err != nil {
		fmt.Printf("failed to create rfc: %v", err)
	}
	fmt.Printf("Response to create rfc: %v\n", resp)
}
