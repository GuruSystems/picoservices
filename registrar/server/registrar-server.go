package main

import (
	"os"
	"fmt"
	"flag"
	"log"
	"net"
	"time"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"io/ioutil"
	"crypto/tls"
	"container/list"
	//
	"google.golang.org/grpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc/peer"
	//
	pb "github.com/GuruSystems/framework/proto/registrar"
)

// static variables for flag parser
var (
	port         = flag.Int("port", 5000, "The server port")
	keepAlive    = flag.Int("keepalive", 2, "keep alive interval in seconds to check each registered service")
	max_failures = flag.Int("max_failures", 10, "max failures after which service will be deregistered")
	services     *list.List
	idCtr        = 0
)

type serviceEntry struct {
	desc      *pb.ServiceDescription
	instances []*serviceInstance
}
type serviceInstance struct {
	serviceID       int
	failures        int
	disabled        bool
	firstRegistered time.Time
	lastSuccess     time.Time
	lastRefresh     time.Time
	address         pb.ServiceAddress
	apitype         []pb.Apitype
}

func (si *serviceInstance) toString() string {
	s := fmt.Sprintf("%d %s:%d", si.serviceID, si.address.Host, si.address.Port)
	return s
}
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
	fmt.Printf("Serving...\n")
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
			if !instance.hasApi(pb.Apitype_status) {
				continue
			}

			err := CheckService(sloc, instance)
			if err != nil {
				fmt.Printf("Service %s@%s:%d failed %d times: %s\n", sloc.desc.Name, instance.address.Host, instance.address.Port, instance.failures, err)
				instance.failures++
			} else {
				instance.failures = 0
				instance.lastSuccess = time.Now()
			}
		}
	}
	removed := removeInvalidInstances()
	if removed {
		UpdateTargets()
	}
}

// true if some where removed
func removeInvalidInstances() bool {
	// remove failed instances
	res := false
	for e := services.Front(); e != nil; e = e.Next() {
		se := e.Value.(*serviceEntry)
		for i := 0; i < len(se.instances); i++ {
			instance := se.instances[i]
			if !isValid(instance) {
				se.instances[len(se.instances)-1], se.instances[i] = se.instances[i], se.instances[len(se.instances)-1]
				se.instances = se.instances[:len(se.instances)-1]

				fmt.Printf("Instance %s removed due to excessive failures (or disabled)\n", se.toString())
				res = true
				break
			}
		}
	}
	return res
}

func (si *serviceEntry) toString() string {
	name := fmt.Sprintf("%s:%s", si.desc.Name, si.desc.Gurupath)
	return name
}

func isValid(si *serviceInstance) bool {
	MAXAGE := time.Duration(180)
	// time it out if there's no refresh!
	if time.Since(si.lastRefresh) > (time.Second * MAXAGE) {
		fmt.Printf("invalidating instance %s: has not refreshed for %d seconds\n", si.toString(), MAXAGE)
		return false
	}
	if si.disabled {
		fmt.Printf("invalidating instance %s: its disabled\n", si.toString())
		return false
	}
	if si.failures > *max_failures {
		fmt.Printf("invalidating instance %s: failed %d times\n", si.toString(), si.failures)
		return false
	}
	if si.hasApi(pb.Apitype_status) {
		if time.Since(si.lastSuccess) > (time.Second * 30) {
			fmt.Printf("invalidating instance %s: last success is too long ago\n", si.toString())
			return false
		}
	}
	return true
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
	if sn != desc.desc.Name {
		fmt.Printf("Reported Service: \"%s\", expected: \"%s\"\n", sn, desc.desc.Name)
		return errors.New("Servicename mismatch")
	}
	return nil
}

func (si *serviceInstance) hasApi(lf pb.Apitype) bool {
	ar := si.apitype
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

//
func FindServices(sd *pb.ServiceDescription) []*serviceEntry {
	var res []*serviceEntry
	for e := services.Front(); e != nil; e = e.Next() {
		sl := e.Value.(*serviceEntry)
		if (sl.desc.Gurupath != "") && (sd.Gurupath != "") {
			if sl.desc.Gurupath != sd.Gurupath {
				continue
			}
		}
		if sl.desc.Name == sd.Name {
			res = append(res, sl)
		}
	}
	return res
}

// this is not a good thing - it finds the FIRST entry by name
func FindService(sd *pb.ServiceDescription) *serviceEntry {
	for e := services.Front(); e != nil; e = e.Next() {
		sl := e.Value.(*serviceEntry)
		if (sl.desc.Gurupath != "") && (sd.Gurupath != "") {
			if sl.desc.Gurupath != sd.Gurupath {
				continue
			}
		}
		if sl.desc.Name == sd.Name {
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
		sln.desc = sd
		sl = &sln
		sl.instances = make([]*serviceInstance, 0)
		services.PushFront(sl)
	}
	// check if address sa already in location
	for _, instance := range sl.instances {
		if (instance.address.Host == hostname) && (instance.address.Port == port) {
			instance.lastRefresh = time.Now()
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
	si.lastRefresh = time.Now()
	si.address = pb.ServiceAddress{Host: hostname, Port: port}
	si.apitype = apitype
	sl.instances = append(sl.instances, si)
	//fmt.Printf("Apitype: %s\n", si.apitype)
	fmt.Printf("Registered new service %s at %s:%d (%d) [%s]\n", sd.Name, hostname, port, len(sl.instances), sd.Gurupath)
	UpdateTargets()
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
}

func (s *RegistryService) GetServiceAddress(ctx context.Context, gr *pb.GetRequest) (*pb.GetResponse, error) {
	/*
		peer, ok := peer.FromContext(ctx)
		if !ok {
			fmt.Println("Error getting peer ")
			return nil, errors.New("Error getting peer from contextn")
		}
	*/
	//fmt.Printf("%s called get service address for service %s\n", peer.Addr, gr.Service.Name)
	slv := FindServices(gr.Service)
	if len(slv) == 0 {
		fmt.Printf("Service \"%s\" is not currently registered\n", gr.Service.Name)
		return nil, errors.New("service not registered")
	}
	resp := pb.GetResponse{}
	resp.Service = slv[0].desc
	resp.Location = new(pb.ServiceLocation)
	resp.Location.Service = slv[0].desc
	for _, sl := range slv {
		//var serviceAddresses []pb.ServiceAddress
		for _, in := range sl.instances {
			sa := in.address
			resp.Location.Address = append(resp.Location.Address, &sa)
		}
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
	fmt.Printf("Deregistered Service %s\n", si.toString())
	UpdateTargets()
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
		fmt.Printf("Warning! no deploymentpath in registration request. Are you testing? (peer=%s, servicename=%s \n", peer, pr.Service.Name)
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
		nsa := &pb.ServiceAddress{Host: host, Port: address.Port}
		nsa.ApiType = []pb.Apitype{1, 2}
		rr.Location.Address = append(rr.Location.Address, nsa)
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
		if (pr.Name != "") && (pr.Name != se.desc.Name) {
			continue
		}
		fmt.Printf("Service %s has %d instances\n", se.desc.Name, len(se.instances))
		if len(se.instances) == 0 {
			continue
		}
		rr := pb.GetResponse{}
		lr.Service = append(lr.Service, &rr)

		rr.Service = se.desc
		rr.Location = new(pb.ServiceLocation)
		rr.Location.Service = rr.Service
		svcadr := []*pb.ServiceAddress{}
		for _, in := range se.instances {
			sa := &in.address
			sa.ApiType = in.apitype
			svcadr = append(svcadr, sa)
			fmt.Printf("Service %s @ %s:%d (%s)\n", se.desc.Name, in.address.Host, in.address.Port, in.apitype)
		}
		rr.Location.Address = svcadr
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

// find target based on deploymentpath & apitype...
func (s *RegistryService) GetTarget(ctx context.Context, pr *pb.GetTargetRequest) (*pb.ListResponse, error) {
	lr := &pb.ListResponse{}
	for e := services.Front(); e != nil; e = e.Next() {
		se := e.Value.(*serviceEntry)
		if pr.Gurupath != "" {
			if !isDeployPath(se.desc.Gurupath, pr.Gurupath) {
				fmt.Printf("No match \"%s\" and \"%s\"\n", se.desc.Gurupath, pr.Gurupath)
				continue
			}
		}
		if pr.Name != "" {
			if se.desc.Name != pr.Name {
				continue
			}
		}
		for _, si := range se.instances {
			if si.hasApi(pr.ApiType) {
				//fmt.Printf("Adding %s\n", si.toString())
				sd := se.desc
				gr := &pb.GetResponse{}
				gr.Service = sd
				gr.Location = &pb.ServiceLocation{}
				sa := &si.address
				sa.ApiType = si.apitype
				gr.Location.Address = append(gr.Location.Address, sa)
				lr.Service = append(lr.Service, gr)
			}
		}
	}
	return lr, nil
	//	return nil, errors.New("No such endpoint (%v)", pr)
}

// this needs to be smarter and handle stuff like "better" matches
// see github.com/GuruSystems/framework/server/server.go for path syntax
func isDeployPath(actual string, requested string) bool {
	ap := strings.Split(actual, "/")
	rp := strings.Split(requested, "/")

	if len(ap) != 4 {
		// invalid actual deploypath
		return false
	}
	// last section is optional
	if (len(rp) != 4) && (len(rp) != 3) {
		return false
	}

	for i := 0; i < len(rp); i++ {
		if i == 3 && rp[i] == "latest" {
			// see?this is bad, it's not "latest" it's coded as "any"
			continue
		}
		if rp[i] != ap[i] {
			return false
		}
	}
	return true
}
func (s *RegistryService) InformProcessShutdown(ctx context.Context, pr *pb.ProcessShutdownRequest) (*pb.EmptyResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
		return nil, errors.New("Error getting peer from contextn")
	}
	adr := pr.IP
	if adr == "" {
		peerhost, _, err := net.SplitHostPort(peer.Addr.String())
		if err != nil {
			return nil, errors.New("Invalid peer")
		}
		adr = peerhost
	}
	fmt.Printf("called shutdown service from address %s with adr %s\n", peer.Addr.String(), adr)
	for e := services.Front(); e != nil; e = e.Next() {
		sloc := e.Value.(*serviceEntry)
		for _, instance := range sloc.instances {
			if instance.address.Host != adr {
				continue
			}
			for _, dp := range pr.Port {
				if instance.address.Port == dp {
					fmt.Printf("Disabled %s\n", instance.toString())
					instance.disabled = true
				}
			}
		}
	}
	return &pb.EmptyResponse{}, nil
}
