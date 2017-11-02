package auth

// TODO: how/when do we close database connections? (pooling?)

import ()

type User struct {
	FirstName string
	LastName  string
	Email     string
	ID        string
}

// injected into a context
type AuthInfo struct {
	UserID string
}

type Authenticator interface {
	Authenticate(token string) (*User, error)
}
