package compound

import (
	"flag"
	"fmt"
	"google.golang.org/grpc"
)

var (
	registrar = flag.String("registrar", "localhost:5000", "address of the registrar server")
)

func DialWrapper(servicename string) (*grpc.ClientConn, error) {
	serverAddr := registrar
	fmt.Printf("Using registrar @%s\n", *serverAddr)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		fmt.Printf("Error dialling servicename %s @ %s\n", servicename, serverAddr)
		return nil, err
	}
	return conn, nil
}
