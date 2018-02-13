package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"flag"
	"fmt"
	"github.com/GuruSystems/framework/client"
	pb "github.com/GuruSystems/framework/proto/keyvalueserver"
)

// static variables for flag parser
var (
	action = flag.String("action", "get", "get or put")
	key    = flag.String("key", "foo", "the key to store the value under")
	value  = flag.String("value", "bar", "the value of the key to store")
)

func main() {
	flag.Parse()
	conn, err := client.DialWrapper("keyvalueserver.KeyValueService")
	if err != nil {
		fmt.Println("failed to dial: %v", err)
		return
	}
	defer conn.Close()
	ctx := client.SetAuthToken()

	client := pb.NewKeyValueServiceClient(conn)
	if *action == "put" {
		req := pb.PutRequest{Key: *key, Value: "bar"}
		_, err := client.Put(ctx, &req)
		if err != nil {
			fmt.Println("failed to put key to store:", err)
		}
	} else if *action == "get" {
		req := pb.GetRequest{Key: *key}
		resp, err := client.Get(ctx, &req)
		if err != nil {
			fmt.Println("failed to get key from store:", err)
		} else {
			fmt.Printf("Value of key %s: \"%s\"\n", *key, resp.Value)
		}
	}
}
