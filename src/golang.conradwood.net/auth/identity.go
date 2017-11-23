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
	// give token -> return userid
	Authenticate(token string) (string, error)
	GetUserDetail(userid string) (*User, error)
	// given a previous challenge and an email, will return a token if challenge and password stuff matches
	CreateVerifiedToken(email string, password string) string
}
