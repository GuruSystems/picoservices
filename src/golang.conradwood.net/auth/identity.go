package auth

// TODO: how/when do we close database connections? (pooling?)

import (
	"errors"
	"fmt"
)

type User struct {
	FirstName string
	LastName  string
	Email     string
	id        string
}

type Authenticator interface {
	Authenticate(token string) (*User, error)
}
