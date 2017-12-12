package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	"golang.conradwood.net/cmdline"
	pb "golang.conradwood.net/registrar/proto"
	"log"
	"os"
)

// static variables for flag parser
var (
	name = flag.String("name", "", "name of a service, if set output will be filtered to only include services with this name")
)

func main() {
	flag.Parse()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	fmt.Println("Connecting to server...")
	reg := cmdline.GetRegistryAddress()
	conn, err := grpc.Dial(reg, opts...)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewRegistryClient(conn)
	na := flag.NArg()
	if na > 1 {
		if flag.Arg(0) == "shutdown" {
			for i := 1; i < na; i++ {
				s := flag.Arg(i)
				fmt.Printf("Shutting down service \"%s\"\n", s)
				sh := pb.ShutdownRequest{ServiceName: s}
				client.ShutdownService(context.Background(), &sh)
			}
			os.Exit(0)
		}
	}
	req := pb.ListRequest{}
	req.Name = *name
	resp, err := client.ListServices(context.Background(), &req)
	if err != nil {
		log.Fatalf("failed to list services: %v", err)
	}
	fmt.Printf("%d services registered\n", len(resp.Service))
	for _, getr := range resp.Service {
		fmt.Printf("Service: %s (%s)\n", getr.Service.Name, getr.Service.Gurupath)
		for _, addr := range getr.Location.Address {
			api := ApiToString(addr.ApiType)
			fmt.Printf("   %s:%d (%s)\n", addr.Host, addr.Port, api)
		}
	}
}

func ApiToString(pa []pb.Apitype) string {
	deli := ""
	res := ""
	for _, apitype := range pa {
		res = fmt.Sprintf("%s%s%s", res, deli, apitype)
		deli = ", "
	}
	return res
}
