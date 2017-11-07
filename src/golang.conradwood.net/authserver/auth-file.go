package main

// TODO: how/when do we close database connections? (pooling?)

import (
	"errors"
	"fmt"
	"golang.conradwood.net/auth"
	"os"
)

type FileAuthenticator struct {
	dir string
	auth.Authenticator
}

func (fa *FileAuthenticator) Authenticate(token string) (*auth.User, error) {
	return nil, nil
}
func NewFileAuthenticator(tokendir string) (auth.Authenticator, error) {
	st, err := os.Stat(tokendir)
	if err != nil {
		fmt.Printf("Cannot stat %s: %s", tokendir, err)
		return nil, errors.New("unable to stat dir")
	}
	if !st.Mode().IsDir() {
		fmt.Printf(" %s is not a directory", tokendir)
		return nil, errors.New("not a directory")
	}

	fd := FileAuthenticator{dir: tokendir}
	return &fd, nil
}
