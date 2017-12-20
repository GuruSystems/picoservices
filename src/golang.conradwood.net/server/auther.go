package server

import (
	"errors"
	"fmt"
	"golang.conradwood.net/auth"
	apb "golang.conradwood.net/auth/proto"
	"golang.conradwood.net/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"time"
)

var (
	authconnCache *grpc.ClientConn
)

const (
	COOKIE_NAME = "Auth-Token"
)

func addUserToCache(token string, id string) {
	uc := UserCache{UserID: id, created: time.Now()}
	usercache[token] = &uc
}

// we must not return useful errormessages here,
// so we print them to stdout instead and return a generic message
func authenticate(ctx context.Context, meta metadata.MD) (context.Context, error) {
	if len(meta["token"]) != 1 {
		fmt.Println("RPCServer: Invalid number of tokens: ", len(meta["token"]))
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	token := meta["token"][0]
	return authenticateToken(ctx, token)
}
func authenticateToken(ctx context.Context, token string) (context.Context, error) {
	var err error
	authconn, err := client.DialWrapper("auth.AuthenticationService")
	if err != nil {
		fmt.Printf("Could not establish connection to auth service:%s\n", err)
		return nil, err
	}
	if authconn == nil {
		fmt.Printf("no authentication server connection?\n")
		return nil, errors.New("Now auth available\n")
	}
	defer authconn.Close()
	uc := getUserFromCache(token)
	if uc != "" {
		ai := auth.AuthInfo{UserID: uc}
		nctx := context.WithValue(ctx, "authinfo", ai)
		return nctx, nil
	}
	authc := apb.NewAuthenticationServiceClient(authconn)
	req := &apb.VerifyRequest{Token: token}
	repeat := 4
	var resp *apb.VerifyResponse
	for {
		resp, err = authc.VerifyUserToken(ctx, req)
		if err == nil {
			break
		}
		ps := "unknown"
		peer, ok := peer.FromContext(ctx)
		if ok {
			ps = fmt.Sprintf("%v", peer)
		}

		fmt.Printf("(%d) VerifyUserToken(%s) failed: %s for request from %s\n", repeat, token, err, ps)
		if repeat <= 1 {
			return nil, err
		}

		fmt.Printf("Due to failure (%s) verifying token we re-connect...\n", err)
		authconn, err = client.DialWrapper("auth.AuthenticationService")
		defer authconn.Close()
		if err != nil {
			fmt.Printf("Resetting the connection to auth service did not help either:%s\n", err)
			return nil, err
		}
		authc = apb.NewAuthenticationServiceClient(authconn)

		repeat--
	}
	// should never happen - but it's auth, so extra check doesn't hurt
	if (resp == nil) || (resp.UserID == "") {
		fmt.Println("RPCServer: BUG: a user was authenticated but no userid returned!")
		return nil, grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	addUserToCache(token, resp.UserID)
	ai := auth.AuthInfo{UserID: resp.UserID}
	fmt.Printf("RPCServer: Authenticated user \"%s\".\n", resp.UserID)
	nctx := context.WithValue(ctx, "authinfo", ai)
	return nctx, nil
}
func GetUserID(ctx context.Context) auth.AuthInfo {
	ai := ctx.Value("authinfo").(auth.AuthInfo)
	return ai
}

// this is dangerous - a potential timebomb, so we cache
// which makes it difficult to respond to errors
func GetAuthClient() (apb.AuthenticationServiceClient, error) {
	if authconnCache != nil {
		client := apb.NewAuthenticationServiceClient(authconnCache)
		return client, nil
	}
	authconn, err := client.DialWrapper("auth.AuthenticationService")

	if err != nil {
		fmt.Printf("Could not establish connection to auth service:%s\n", err)
		return nil, err
	}
	authconnCache = authconn
	client := apb.NewAuthenticationServiceClient(authconn)
	return client, nil
}
