package main

// TODO: how/when do we close database connections? (pooling?)

import (
	"golang.conradwood.net/auth"
)

type NilAuthenticator struct {
	auth.Authenticator
}

func (pga *NilAuthenticator) Authenticate(token string) (string, error) {
	return "", nil
}
