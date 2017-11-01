package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	//"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	"golang.conradwood.net/compound"
	pb "golang.conradwood.net/keyvalueserver/proto"
)

// static variables for flag parser
var (
	action = flag.String("action", "get", "get or put")
	key    = flag.String("key", "foo", "the key to store the value under")
	value  = flag.String("value", "bar", "the value of the key to store")
)

func main() {
	flag.Parse()
	conn, err := compound.DialWrapper("keyvalueserver")
	if err != nil {
		fmt.Println("fail to dial: %v", err)
		return
	}
	defer conn.Close()
	client := pb.NewKeyValueServiceClient(conn)
	if *action == "put" {
		req := pb.PutRequest{Key: "foo", Value: "bar"}
		_, err := client.Put(context.Background(), &req)
		if err != nil {
			fmt.Println("fail to put key to store:", err)
		}
	} else if *action == "get" {
		req := pb.GetRequest{Key: "foo"}
		resp, err := client.Get(context.Background(), &req)
		if err != nil {
			fmt.Println("fail to get key from store:", err)
		}
		fmt.Printf("Value of key %s: \"%s\"\n", resp.Value)
	}
}
