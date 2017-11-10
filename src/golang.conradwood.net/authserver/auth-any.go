package main

// TODO: how/when do we close database connections? (pooling?)

import (
	"golang.conradwood.net/auth"
)

type AnyAuthenticator struct {
	auth.Authenticator
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
