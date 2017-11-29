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

	pb "golang.conradwood.net/auth/proto"
	// we really only pull it in to get the certificates...
	"golang.conradwood.net/client"
	"io/ioutil"
	"os"
	"os/user"
)

// static variables for flag parser
var (
	serverAddr = flag.String("server_addr", "127.0.0.1:4998", "The server address in the format of host:port")
	usertoken  = flag.String("usertoken", "", "user token to authenticate with")
	email      = flag.String("email", "", "email address of the user to create")
	firstname  = flag.String("firstname", "", "Firstname of the user to create")
	lastname   = flag.String("lastname", "", "Lastname of the user to create")
	username   = flag.String("username", "", "username of the user to create")
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

	creds := client.GetClientCreds()

	fmt.Println("Connecting to server...", *serverAddr, creds)
	//conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure())
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Println("fail to dial: %v", err)
		return
	}
	defer conn.Close()
	fmt.Println("Creating aclient...")
	aclient := pb.NewAuthenticationServiceClient(conn)
	ctx := context.Background()

	if (*email != "") || (*firstname != "") || (*lastname != "") || (*username != "") {
		req := &pb.CreateUserRequest{
			UserName:  *username,
			Email:     *email,
			FirstName: *firstname,
			LastName:  *lastname,
		}
		_, err = aclient.CreateUser(ctx, req)
		if err != nil {
			fmt.Printf("Failed to create user: %s\n", err)
			os.Exit(10)
		}
		os.Exit(0)
	}

	tok := ResolveAuthToken(*usertoken)

	// if TLS is f*** we break at the first RPC call

	if *usertoken == "" {
		user := readLine("Username: ")
		pw := readLine("Password: ")
		fmt.Printf("Attempting to authenticate %s with %s...\n", user, pw)
		cr, err := aclient.AuthenticatePassword(ctx, &pb.AuthenticatePasswordRequest{Email: user, Password: pw})
		bail(err, "Failed to get auth challenge")
		fmt.Printf("Result: %v\n", cr)
		tok = cr.Token
	}

	req := pb.VerifyRequest{Token: tok}
	fmt.Println("RPC call to auth server...")
	resp, err := aclient.VerifyUserToken(ctx, &req)
	if err != nil {
		fmt.Printf("failed to verify user token: %v\n", err)
		return
	}
	fmt.Printf("Response to verify token: %v\n", resp)
	gdr := pb.GetDetailRequest{UserID: resp.UserID}
	det, err := aclient.GetUserDetail(ctx, &gdr)
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
