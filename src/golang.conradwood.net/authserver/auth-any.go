package main

// TODO: how/when do we close database connections? (pooling?)

import (
	"errors"
	"golang.conradwood.net/auth"
	pb "golang.conradwood.net/auth/proto"
)

type AnyAuthenticator struct {
}

func (pga *AnyAuthenticator) GetUserDetail(user string) (*auth.User, error) {
	au := auth.User{
		FirstName: "john",
		LastName:  "doe",
		Email:     "john.doe@microsoft.com",
		ID:        "1",
	}
	return &au, nil
}
func (pga *AnyAuthenticator) Authenticate(token string) (string, error) {
	return "1", nil
}

func (pga *AnyAuthenticator) CreateVerifiedToken(email string, pw string) string {
	return "generated_token"
}
func (pga *AnyAuthenticator) CreateUser(*pb.CreateUserRequest) (string, error) {
	return "", errors.New("CreateUser() not yet implemented")
}
