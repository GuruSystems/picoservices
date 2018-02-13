package main

import (
	"errors"
	"github.com/GuruSystems/framework/auth"
	pb "github.com/GuruSystems/framework/proto/auth"
)

type NilAuthenticator struct {
}

func (pga *NilAuthenticator) Authenticate(token string) (string, error) {
	return "", nil
}
func (pga *NilAuthenticator) GetUserDetail(user string) (*auth.User, error) {
	return nil, errors.New("NIL backend does not authenticate")
}
func (pga *NilAuthenticator) CreateVerifiedToken(email string, pw string) string {
	return ""
}
func (pga *NilAuthenticator) CreateUser(*pb.CreateUserRequest) (string, error) {
	return "", errors.New("CreateUser() not yet implemented")
}
