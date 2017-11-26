package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	"strings"
	//	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	"bufio"
	"crypto/tls"
	"crypto/x509"
	pb "golang.conradwood.net/auth/proto"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"os"
	"os/user"
)

// static variables for flag parser
var (
	serverAddr = flag.String("server_addr", "127.0.0.1:4998", "The server address in the format of host:port")
	crt        = "/etc/cnw/certs/rpc-client/certificate.pem"
	key        = "/etc/cnw/certs/rpc-client/privatekey.pem"
	ca         = "/etc/cnw/certs/rpc-client/ca.pem"
	token      = flag.String("token", "", "user token to authenticate with")
)

func readLine(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	name := strings.TrimSpace(text)
	return name
}

func bail(err error, msg string) {
	if err == nil {
		return
	}
	fmt.Printf("%s: %s\n", msg, err)
	os.Exit(10)
}

func main() {
	flag.Parse()
	roots := x509.NewCertPool()
	FrontendKey, err := ioutil.ReadFile(key)
	bail(err, "Failed to load key")

	FrontendCert, err := ioutil.ReadFile(crt)
	bail(err, "Failed to load cert")
	roots.AppendCertsFromPEM(FrontendCert)
	ImCert, err := ioutil.ReadFile(ca)
	bail(err, "Failed to load ca")
	roots.AppendCertsFromPEM(ImCert)

	// Create credentials
	//	creds := credentials.NewClientTLSFromCert(roots, "")
	cert, err := tls.X509KeyPair(FrontendCert, FrontendKey)

	creds := credentials.NewTLS(&tls.Config{
		ServerName:         *serverAddr,
		Certificates:       []tls.Certificate{cert},
		RootCAs:            roots,
		InsecureSkipVerify: true,
	})
	fmt.Println("Connecting to server...", *serverAddr, creds)
	//conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure())
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println("fail to dial: %v", err)
		return
	}
	defer conn.Close()
	fmt.Println("Creating client...")
	client := pb.NewAuthenticationServiceClient(conn)
	tok := ResolveAuthToken(*token)

	ctx := context.Background()

	// if TLS is f*** we break at the first RPC call

	if *token == "" {
		user := readLine("Username: ")
		pw := readLine("Password: ")
		fmt.Printf("Attempting to authenticate %s with %s...\n", user, pw)
		cr, err := client.AuthenticatePassword(ctx, &pb.AuthenticatePasswordRequest{Email: user, Password: pw})
		bail(err, "Failed to get auth challenge")
		fmt.Printf("Result: %v\n", cr)
		tok = cr.Token
	}

	req := pb.VerifyRequest{Token: tok}
	fmt.Println("RPC call to auth server...")
	resp, err := client.VerifyUserToken(ctx, &req)
	if err != nil {
		fmt.Printf("failed to verify user token: %v\n", err)
		return
	}
	fmt.Printf("Response to verify token: %v\n", resp)
	gdr := pb.GetDetailRequest{UserID: resp.UserID}
	det, err := client.GetUserDetail(ctx, &gdr)
	if err != nil {
		fmt.Printf("failed to retrieve user %i: %s\n", resp.UserID, err)
	}
	fmt.Println("User: ", det)
}

func ResolveAuthToken(token string) string {
	var tok string
	var btok []byte
	var fname string
	fname = "n/a"
	usr, err := user.Current()
	if err == nil {
		fname = fmt.Sprintf("%s/.picoservices/tokens/%s", usr.HomeDir, token)
		btok, _ = ioutil.ReadFile(fname)
	}
	if (err != nil) || (len(btok) == 0) {
		tok = token
	} else {
		tok = string(btok)
		fmt.Printf("Using token from %s\n", fname)
	}
	return strings.TrimSpace(tok)
}
