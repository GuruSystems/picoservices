package main

import (
	"bufio"
	"flag"
	"fmt"
	pb "golang.conradwood.net/auth/proto"
	"golang.conradwood.net/client"
	"golang.org/x/net/context"
	//	"io/ioutil"
	"os"
	"strings"
)

func bail(err error, msg string) {
	if err == nil {
		return
	}
	fmt.Printf("%s: %s\n", msg, err)
	os.Exit(10)
}

func readLine(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	name := strings.TrimSpace(text)
	return name
}

func main() {
	flag.Parse()
	conn, err := client.DialWrapper("auth.AuthenticationService")

	if err != nil {
		fmt.Println("failed to dial: %v", err)
		os.Exit(10)
	}
	defer conn.Close()
	ctx := context.Background()
	aclient := pb.NewAuthenticationServiceClient(conn)
	user := readLine("Username: ")
	pw := readLine("Password: ")
	fmt.Printf("Attempting to authenticate %s with %s...\n", user, pw)
	cr, err := aclient.AuthenticatePassword(ctx, &pb.AuthenticatePasswordRequest{Email: user, Password: pw})
	bail(err, "Failed to get auth challenge")
	fmt.Printf("Result: %v\n", cr)
	tok := cr.Token
	fmt.Printf("tok: %s\n", tok)
	client.SaveToken(tok)
}
