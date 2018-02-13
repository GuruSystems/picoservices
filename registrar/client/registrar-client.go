package main

// see: https://grpc.io/docs/tutorials/basic/go.html

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"flag"
	"golang.org/x/net/context"
	//	"net"
	"github.com/GuruSystems/framework/cmdline"
	pb "github.com/GuruSystems/framework/proto/registrar"
	"log"
	"os"
)

// static variables for flag parser
var (
	deploypath = flag.String("deployment_path", "", "deployment path to lookup (requires \"apitype\")")
	apitype    = flag.String("apitype", "", "apitype to look up")
	name       = flag.String("name", "", "name of a service, if set output will be filtered to only include services with this name")
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
	if *apitype != "" {
		lookup(client)
		os.Exit(0)
	}
	req := pb.ListRequest{}
	req.Name = *name
	resp, err := client.ListServices(context.Background(), &req)
	if err != nil {
		log.Fatalf("failed to list services: %v", err)
	}
	printResponse(resp)
}
func printResponse(lr *pb.ListResponse) {
	fmt.Printf("%d services registered\n", len(lr.Service))
	for _, getr := range lr.Service {
		fmt.Printf("Service: %s (%s)\n", getr.Service.Name, getr.Service.Gurupath)
		for _, addr := range getr.Location.Address {
			api := ApiToString(addr.ApiType)
			fmt.Printf("   %s:%d (%s)\n", addr.Host, addr.Port, api)
		}
	}
}

func lookup(client pb.RegistryClient) {
	v, ok := pb.Apitype_value[*apitype]
	if !ok {
		fmt.Printf("Invalid apitype %s\n", *apitype)
		fmt.Printf("Valid types: ")
		for name, _ := range pb.Apitype_value {
			fmt.Printf("%s ", name)
		}
		fmt.Printf("\n")
		os.Exit(10)
	}
	x := *deploypath
	if x == "" {
		x = *name
	}
	fmt.Printf("Finding api endpoint for %s (type %s)\n", x, pb.Apitype_name[v])
	gt := &pb.GetTargetRequest{Gurupath: *deploypath,
		Name:    *name,
		ApiType: pb.Apitype(v)}
	lr, err := client.GetTarget(context.Background(), gt)
	if err != nil {
		fmt.Printf("Failed to lookup api endpoint for %s (type %s): %s\n", *deploypath, pb.Apitype_name[v], err)
		os.Exit(10)
	}
	printResponse(lr)
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
