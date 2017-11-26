package main

// TODO: authenticates against an ldap backend.
// docs: https://godoc.org/gopkg.in/ldap.v2

// this also needs a secondary store because our ldap schema doesn't store all the stuff we need

import (
	"crypto/tls"
	"flag"
	"fmt"
	"golang.conradwood.net/auth"
	"gopkg.in/ldap.v2"
)

var (
	ldaphost     = flag.String("ldap_server", "localhost", "the ldap server to authenticate users against")
	ldapport     = flag.Int("ldap_port", 10389, "the ldap server's port to authenticate users against")
	bindusername = flag.String("ldap_bind_user", "", "The user to look up a users cn with prior to authentication")
	bindpw       = flag.String("ldap_bind_pw", "", "The password of the user to look up a users cn with prior to authentication")
	ldaporg      = flag.String("ldap_org", "", "The cn of the top level tree to search for the user in")
)

type LdapAuthenticator struct {
}

func (pga *LdapAuthenticator) GetUserDetail(user string) (*auth.User, error) {
	au := auth.User{
		FirstName: "john",
		LastName:  "doe",
		Email:     "john.doe@microsoft.com",
		ID:        "1",
	}
	return &au, nil
}
func (pga *LdapAuthenticator) Authenticate(token string) (string, error) {
	return "1", nil
}

func (pga *LdapAuthenticator) CreateVerifiedToken(email string, pw string) string {
	return CheckLdapPassword(email, pw)
}
func CheckLdapPassword(username string, pw string) string {
	// The username and password we want to check
	password := pw

	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", *ldaphost, *ldapport))
	if err != nil {
		fmt.Printf("Failed to connect to ldap host %s:%d: %s\n", *ldaphost, *ldapport, err)
		return ""
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		fmt.Printf("Failed to do stuff: %s", err)
		return ""
	}

	// First bind with a read only user
	err = l.Bind(*bindusername, *bindpw)
	if err != nil {
		fmt.Printf("Failed to do stuff: %s", err)
		return ""
	}

	ldapClass := "posixAccount"
	fmt.Printf("Searching for class %s and uid=%s\n", ldapClass, username)
	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		*ldaporg,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=%s)(cn=%s))", ldapClass, username),
		[]string{"cn"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		fmt.Printf("Failed to do search for %s: %s\n", username, err)
		return ""
	}

	if len(sr.Entries) < 1 {
		fmt.Printf("User \"%s\" does not exist\n", username)
		return ""
	}
	if len(sr.Entries) > 1 {
		fmt.Printf("Too many user entries returned: %d\n", len(sr.Entries))
		for _, e := range sr.Entries {
			fmt.Printf("  %v\n", e)
		}
		return ""
	}

	userdn := sr.Entries[0].DN
	fmt.Printf("Found userobject: %s\n", userdn)
	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		fmt.Printf("Failed to do bind as user %s: %s\n", username, err)
		return ""
	}

	au := ldapToUser(sr.Entries[0])
	if au == nil {
		fmt.Printf("Failed to create user from ldap entry.\n")
		return ""
	}

	tk := RandomString(64)

	// Rebind as the read only user for any further queries
	err = l.Bind(*bindusername, *bindpw)
	if err != nil {
		fmt.Printf("Failed to do stuff: %s", err)
		return ""
	}

	return tk
}

func ldapToUser(entry *ldap.Entry) *auth.User {
	a := auth.User{
		FirstName: entry.GetAttributeValue("cn"),
		LastName:  entry.GetAttributeValue("sn"),
	}
	return &a
}
