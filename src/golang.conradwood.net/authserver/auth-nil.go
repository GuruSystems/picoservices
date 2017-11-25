package main

import (
	"errors"
	"golang.conradwood.net/auth"
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
