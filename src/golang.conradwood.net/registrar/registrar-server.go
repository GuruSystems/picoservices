package main

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"container/list"
	"errors"
	"flag"
	pb "golang.conradwood.net/registrar/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/peer"
	"log"
	"net"
)

// static variables for flag parser
var (
	port     = flag.Int("port", 5000, "The server port")
	services *list.List
)

func main() {
	flag.Parse() // parse stuff. see "var" section above
	listenAddr := fmt.Sprintf(":%d", *port)
	fmt.Println("Starting Registry Service on ", listenAddr)
	lis, err := net.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	services = list.New()

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	s := new(RegistryService)
	pb.RegisterRegistryServer(grpcServer, s) // created by proto

	grpcServer.Serve(lis)
}

/**********************************
* helpers
***********************************/
func FindService(sd *pb.ServiceDescription) *pb.ServiceLocation {
	for e := services.Front(); e != nil; e = e.Next() {
		srvloc := e.Value.(*pb.ServiceLocation)
		if srvloc.Service.Name == sd.Name {
			return srvloc
		}
	}
	return nil
}
func AddService(sd *pb.ServiceDescription, hostname string, port int32) {
	if sd.Name == "" {
		fmt.Printf("NO NAME: %v\n", sd)
		return
	}

	sl := FindService(sd)
	if sl == nil {
		sl = new(pb.ServiceLocation)
		sl.Service = new(pb.ServiceDescription)
		*sl.Service = *sd
		services.PushFront(sl)
	}

	// append address to service location
	sa := new(pb.ServiceAddress)
	sa.Host = hostname
	sa.Port = port
	sl.Address = append(sl.Address, sa)
	fmt.Printf("Registered service %s (%s) at %s:%d\n", sd.Name, sd.Type, hostname, port)
}

/**********************************
* implementing the functions here:
***********************************/
type RegistryService struct {
	wtf int
}

// in C we put methods into structs and call them pointers to functions
// in java/python we also put pointers to functions into structs and but call them "objects" instead
// in Go we don't put functions pointers into structs, we "associate" a function with a struct.
// (I think that's more or less the same as what C does, just different Syntax)
func (s *RegistryService) GetServiceAddress(ctx context.Context, gr *pb.GetRequest) (*pb.GetResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
		return nil, errors.New("Error getting peer from contextn")
	}
	fmt.Printf("%s called get service address for service %s\n", peer.Addr, gr.Service.Name)
	sl := FindService(gr.Service)
	if sl == nil {
		fmt.Printf("Service \"%s\" is not currently registered\n", gr.Service.Name)
		return nil, errors.New("service not registered")
	}
	resp := pb.GetResponse{}
	resp.Service = gr.Service
	resp.Location = sl
	return &resp, nil
}

func (s *RegistryService) RegisterService(ctx context.Context, pr *pb.ServiceLocation) (*pb.GetResponse, error) {

	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
		return nil, errors.New("Error getting peer from context")
	}
	peerhost, peerport, err := net.SplitHostPort(peer.Addr.String())
	if err != nil {
		return nil, errors.New("Invalid peer")
	}
	fmt.Printf("Connection from host %s on port %s\n", peerhost, peerport)
	if len(pr.Address) == 0 {
		return nil, errors.New("Missing address!")
	}
	if pr.Service.Name == "" {
		return nil, errors.New("Missing servicename!")
	}
	for _, address := range pr.Address {
		fmt.Printf("  reported: \"%s\" @ \"%s, port %d\"\n", pr.Service.Name, address.Host, address.Port)
		host := address.Host
		if host == "" {
			host = peerhost
		}
		AddService(pr.Service, host, address.Port)
	}
	rr := new(pb.GetResponse)
	return rr, nil
}

func (s *RegistryService) ListServices(ctx context.Context, pr *pb.ListRequest) (*pb.ListResponse, error) {
	lr := new(pb.ListResponse)
	lr.Service = []*pb.GetResponse{}
	// one GetResponse per element
	for e := services.Front(); e != nil; e = e.Next() {
		getr := pb.GetResponse{}
		lr.Service = append(lr.Service, &getr)
		sloc := e.Value.(*pb.ServiceLocation)
		getr.Location = sloc
		sd := sloc.Service
		getr.Service = sd
		fmt.Println("Listing service: ", getr)
	}
	return lr, nil
}
