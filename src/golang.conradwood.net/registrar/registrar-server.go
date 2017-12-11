package main

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"container/list"
	"crypto/tls"
	"errors"
	"flag"
	pb "golang.conradwood.net/registrar/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/peer"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type serviceEntry struct {
	loc       *pb.ServiceDescription
	instances []*serviceInstance
}
type serviceInstance struct {
	serviceID       int
	failures        int
	disabled        bool
	firstRegistered time.Time
	lastSuccess     time.Time
	address         pb.ServiceAddress
	apitype         []pb.Apitype
}

// static variables for flag parser
var (
	port      = flag.Int("port", 5000, "The server port")
	keepAlive = flag.Int("keepalive", 2, "keep alive interval in seconds to check each registered service")
	services  *list.List
	idCtr     = 0
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

	ticker := time.NewTicker(time.Duration(*keepAlive) * time.Second)
	go func() {
		for _ = range ticker.C {
			CheckRegistry()
		}
	}()
	grpcServer.Serve(lis)
}

/**********************************
* check registered servers regularly
***********************************/
func CheckRegistry() {
	// check instances
	for e := services.Front(); e != nil; e = e.Next() {
		sloc := e.Value.(*serviceEntry)
		for _, instance := range sloc.instances {
			// we can only verify this if the instance provides apitype "status"
			if !hasApi(instance.apitype, pb.Apitype_status) {
				continue
			}

			err := CheckService(sloc, instance)
			if err != nil {
				fmt.Printf("Service %s@%s:%d failed %d times: %s\n", sloc.loc.Name, instance.address.Host, instance.address.Port, instance.failures, err)
				instance.failures++
			} else {
				instance.failures = 0
				instance.lastSuccess = time.Now()
			}
		}
	}
	removeInvalidInstances()
}
func removeInvalidInstances() {
	// remove failed instances
	for e := services.Front(); e != nil; e = e.Next() {
		se := e.Value.(*serviceEntry)
		for i := 0; i < len(se.instances); i++ {
			instance := se.instances[i]
			if !isValid(instance) {
				se.instances[len(se.instances)-1], se.instances[i] = se.instances[i], se.instances[len(se.instances)-1]
				se.instances = se.instances[:len(se.instances)-1]
				name := fmt.Sprintf("%s@%s:%d", se.loc.Name, instance.address.Host,
					instance.address.Port)
				fmt.Printf("Instance %s removed due to excessive failures (or disabled)\n", name)
				break
			}
		}
	}
}
func isValid(si *serviceInstance) bool {
	if si.disabled {
		return false
	}
	if si.failures < 10 {
		return true
	}
	if time.Since(si.lastSuccess) < (time.Second * 30) {
		return true
	}
	return false
}
func CheckService(desc *serviceEntry, addr *serviceInstance) error {
	url := fmt.Sprintf("https://%s:%d/internal/service-info/name", addr.address.Host, addr.address.Port)
	//	fmt.Printf("Checking service %s@%s\n", desc.Name, url)
	d := 5 * time.Second
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       d,
		ResponseHeaderTimeout: d,
		ExpectContinueTimeout: d,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	sn := string(body)
	if sn != desc.loc.Name {
		fmt.Printf("Reported Service: \"%s\", expected: \"%s\"\n", sn, desc.loc.Name)
		return errors.New("Servicename mismatch")
	}
	return nil
}

func hasApi(ar []pb.Apitype, lf pb.Apitype) bool {
	for _, a := range ar {
		if a == lf {
			return true
		}
	}
	return false
}

/**********************************
* helpers
***********************************/
func FindInstanceById(id int) *serviceInstance {
	for e := services.Front(); e != nil; e = e.Next() {
		sl := e.Value.(*serviceEntry)
		for _, si := range sl.instances {
			if si.serviceID == id {
				return si
			}
		}
	}
	return nil
}

func FindService(sd *pb.ServiceDescription) *serviceEntry {
	for e := services.Front(); e != nil; e = e.Next() {
		sl := e.Value.(*serviceEntry)
		if sl.loc.Name == sd.Name {
			return sl
		}
	}
	return nil
}

func AddService(sd *pb.ServiceDescription, hostname string, port int32, apitype []pb.Apitype) *serviceInstance {
	if sd.Name == "" {
		fmt.Printf("NO NAME: %v\n", sd)
		return nil
	}

	sl := FindService(sd)
	if sl == nil {
		fmt.Printf("New service! %s\n", sd)
		sln := serviceEntry{}
		sln.loc = sd
		sl = &sln
		sl.instances = make([]*serviceInstance, 0)
		services.PushFront(sl)
	}
	// check if address sa already in location
	for _, instance := range sl.instances {
		if (instance.address.Host == hostname) && (instance.address.Port == port) {
			//fmt.Printf("Re-Registered service %s (%s) at %s:%d\n", sd.Name, sd.Type, hostname, port)
			return instance
		}
	}
	// new instance: append it
	si := new(serviceInstance)
	si.disabled = false
	idCtr++
	si.serviceID = idCtr
	si.firstRegistered = time.Now()
	si.lastSuccess = time.Now()
	si.address = pb.ServiceAddress{Host: hostname, Port: port}
	si.apitype = apitype
	sl.instances = append(sl.instances, si)
	fmt.Printf("Registered service %s at %s:%d (%d)\n", sd.Name, hostname, port, len(sl.instances))
	slx := FindService(sd)
	if len(slx.instances) == 0 {
		fmt.Println("Error, did not save new service")
		os.Exit(10)
	}
	//fmt.Printf("Service: %s with %d instances \n", sl, len(sl.instances))
	return si

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
	resp.Service = sl.loc
	resp.Location = new(pb.ServiceLocation)
	resp.Location.Service = sl.loc
	//var serviceAddresses []pb.ServiceAddress
	for _, in := range sl.instances {
		sa := in.address
		resp.Location.Address = append(resp.Location.Address, &sa)
	}
	return &resp, nil
}
func (s *RegistryService) DeregisterService(ctx context.Context, pr *pb.DeregisterRequest) (*pb.EmptyResponse, error) {
	sid, _ := strconv.Atoi(pr.ServiceID)
	si := FindInstanceById(sid)
	if si == nil {
		return nil, errors.New("No such service to deregister")
	}
	si.disabled = true
	removeInvalidInstances()
	fmt.Printf("Deregistered Service %v\n", si)
	return &pb.EmptyResponse{}, nil
}
func (s *RegistryService) RegisterService(ctx context.Context, pr *pb.ServiceLocation) (*pb.GetResponse, error) {

	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
		return nil, errors.New("Error getting peer from context")
	}
	peerhost, _, err := net.SplitHostPort(peer.Addr.String())
	if err != nil {
		return nil, errors.New("Invalid peer")
	}
	if len(pr.Address) == 0 {
		fmt.Printf("Invalid request (missing address) from peer %s\n", peer)
		return nil, errors.New("Missing address!")
	}
	if pr.Service.Name == "" {
		fmt.Printf("Invalid request (missing servicename) from peer %s\n", peer)
		return nil, errors.New("Missing servicename!")
	}
	if pr.Service.Gurupath == "" {
		fmt.Printf("Warning! no gurupath in registration request. Are you testing? (peer=%s, servicename=%s \n", peer, pr.Service.Name)
	}
	//fmt.Printf("Register service request for service %s from peer %s\n", pr.Service.Name, peer)
	rr := new(pb.GetResponse)
	rr.Service = pr.Service
	rr.Location = new(pb.ServiceLocation)
	rr.Location.Service = pr.Service
	rr.Location.Address = []*pb.ServiceAddress{}

	for _, address := range pr.Address {
		//fmt.Printf("  reported: \"%s\" @ \"%s, port %d\"\n", pr.Service.Name, address.Host, address.Port)
		host := address.Host
		if host == "" {
			host = peerhost
		}
		if host == "127.0.0.1" {
			host = GetLocalIP()
			if host == "" {
				return nil, errors.New("Not registering at localhost")
			}
		}
		si := AddService(pr.Service, host, address.Port, address.ApiType)
		rr.ServiceID = fmt.Sprintf("%d", si.serviceID)
		rr.Location.Address = append(rr.Location.Address, &pb.ServiceAddress{Host: host, Port: address.Port})
	}
	return rr, nil
}
func GetLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Failed to get interfaces: ", err)
		return ""
	}
	res := ""
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		// handle err
		if err != nil {
			fmt.Println("Failed to get address: ", err)
			return ""
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// process IP address
			s := ip.String()
			if (s != "127.0.0.1") && (!strings.Contains(s, ":")) {
				res = s
				break
			}
		}
		if res != "" {
			break
		}
	}
	if res == "" {
		fmt.Printf("Failed to get Local IP from:\n")
		for _, i := range ifaces {
			addrs, err := i.Addrs()
			// handle err
			if err != nil {
				fmt.Println("Failed to get address: ", err)
				return ""
			}
			for _, addr := range addrs {
				fmt.Printf("%s : %s\n", i, addr)
			}
		}
	}
	return res
}
func (s *RegistryService) ListServices(ctx context.Context, pr *pb.ListRequest) (*pb.ListResponse, error) {
	lr := new(pb.ListResponse)
	lr.Service = []*pb.GetResponse{}
	// one GetResponse per element
	for e := services.Front(); e != nil; e = e.Next() {
		se := e.Value.(*serviceEntry)
		if (pr.Name != "") && (pr.Name != se.loc.Name) {
			continue
		}
		fmt.Printf("Service %s has %d instances\n", se.loc.Name, len(se.instances))
		rr := pb.GetResponse{}
		lr.Service = append(lr.Service, &rr)

		rr.Service = se.loc
		rr.Location = new(pb.ServiceLocation)
		rr.Location.Service = rr.Service
		rr.Location.Address = []*pb.ServiceAddress{}
		for _, in := range se.instances {
			sa := &in.address
			rr.Location.Address = append(rr.Location.Address, sa)
			fmt.Printf("Service %s @ %s:%d\n", se.loc.Name, in.address.Host, in.address.Port)
		}
	}
	return lr, nil
}
func (s *RegistryService) ShutdownService(ctx context.Context, pr *pb.ShutdownRequest) (*pb.EmptyResponse, error) {

	sd := pb.ServiceDescription{Name: pr.ServiceName}
	sl := FindService(&sd)
	for _, instance := range sl.instances {
		url := fmt.Sprintf("https://%s:%d/internal/pleaseshutdown",
			instance.address.Host, instance.address.Port)
		d := 5 * time.Second
		tr := &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:          50,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       d,
			ResponseHeaderTimeout: d,
			ExpectContinueTimeout: d,
		}
		client := &http.Client{Transport: tr}
		_, err := client.Get(url)
		if err != nil {
			fmt.Printf("Failed to shutdown: %s\n", err)
		}
	}
	return &pb.EmptyResponse{}, nil
}
