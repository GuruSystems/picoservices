package main

import (
	"fmt"
	"google.golang.org/grpc"
	//	"github.com/golang/protobuf/proto"
	"container/list"
	"flag"

	"golang.conradwood.net/auth"
	pb "golang.conradwood.net/keyvalueserver/proto"
	"golang.conradwood.net/server"
	"golang.org/x/net/context"
	"google.golang.org/grpc/peer"
)

type ValueEntry struct {
	Key   string
	Value string
	Owner string
}

// static variables for flag parser
var (
	port        = flag.Int("port", 4999, "The server port")
	objectStore *list.List
)

// callback from the compound initialisation
func st(server *grpc.Server) error {
	s := new(KeyValueServer)
	// Register the handler object
	pb.RegisterKeyValueServiceServer(server, s)
	return nil
}

func main() {
	flag.Parse() // parse stuff. see "var" section above
	sd := server.NewServerDef()
	sd.Port = *port
	sd.DeployPath = "testing/keyvalue/server/1"
	objectStore = list.New()
	sd.Register = st
	err := server.ServerStartup(sd)
	if err != nil {
		fmt.Printf("failed to start server: %s\n", err)
	}
	fmt.Printf("Done\n")
	return
}

/**********************************
* implementing the functions here:
***********************************/
type KeyValueServer struct {
	wtf int
}

// in C we put methods into structs and call them pointers to functions
// in java/python we also put pointers to functions into structs and but call them "objects" instead
// in Go we don't put functions pointers into structs, we "associate" a function with a struct.
// (I think that's more or less the same as what C does, just different Syntax)
func (s *KeyValueServer) Put(ctx context.Context, pr *pb.PutRequest) (*pb.PutResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	// injected at the unary interceptor after call to authservice
	ai := ctx.Value("authinfo").(auth.AuthInfo)
	fmt.Printf("%s@%s called put %s=%s\n", ai.UserID, peer.Addr, pr.Key, pr.Value)
	ve := find(ai.UserID, pr.Key)
	if ve == nil {
		fmt.Printf("Creating new key %s with value %s for user \"%s\"\n", pr.Key, pr.Value, ai.UserID)
		objectStore.PushFront(&ValueEntry{Key: pr.Key,
			Value: pr.Value,
			Owner: ai.UserID,
		})
	}
	resp := pb.PutResponse{}
	return &resp, nil
}

func (s *KeyValueServer) Get(ctx context.Context, pr *pb.GetRequest) (*pb.GetResponse, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		fmt.Println("Error getting peer ")
	}
	// injected at the unary interceptor after call to authservice
	ai := ctx.Value("authinfo").(auth.AuthInfo)
	fmt.Printf("%s@%s called get %s\n", ai.UserID, peer.Addr, pr.Key)
	resp := &pb.GetResponse{}
	ve := find(ai.UserID, pr.Key)
	if ve != nil {
		resp.Key = ve.Key
		resp.Value = ve.Value
	}
	return resp, nil
}

func find(owner string, key string) *ValueEntry {
	for e := objectStore.Front(); e != nil; e = e.Next() {
		ve := e.Value.(*ValueEntry)
		if (ve.Owner == owner) && (ve.Key == key) {
			return ve
		}
	}
	return nil
}
