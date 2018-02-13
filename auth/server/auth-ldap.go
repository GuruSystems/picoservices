package main

// TODO: authenticates against an ldap backend.
// docs: https://godoc.org/gopkg.in/ldap.v2

// this also needs a secondary store because our ldap schema doesn't store all the stuff we need

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"github.com/GuruSystems/framework/auth"
	pb "github.com/GuruSystems/framework/proto/auth"
	"gopkg.in/ldap.v2"
	"strconv"
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
func (pga *LdapAuthenticator) CreateUser(c *pb.CreateUserRequest) (string, error) {
	return "", errors.New("CreateUser() rpc not implemented for ldap")
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

//********************************************************8
// CREATE A USER
//********************************************************8
func CreateLdapUser(cn string, sn string, uid string, pw string) error {

	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", *ldaphost, *ldapport))
	if err != nil {
		fmt.Printf("Failed to connect to ldap host %s:%d: %s\n", *ldaphost, *ldapport, err)
		return err
	}
	defer l.Close()

	// Reconnect with TLS
	err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		fmt.Printf("Failed to do stuff: %s", err)
		return err
	}

	// First bind with a read only user
	err = l.Bind(*bindusername, *bindpw)
	if err != nil {
		fmt.Printf("Failed to do stuff: %s", err)
		return err
	}

	uidNumber, err := getNextFreeUidNumber(l)
	if err != nil {
		fmt.Printf("Failed to get a free uid number: %s\n", err)
		return err
	}
	fmt.Printf("Next free UID: %d\n", uidNumber)
	gid := uidNumber

	add := ldap.NewAddRequest(fmt.Sprintf("cn=%s,%s", uid, *ldaporg))
	add.Attribute("objectClass", []string{"person", "posixAccount", "shadowAccount", "top"})
	add.Attribute("cn", []string{cn})
	add.Attribute("gidNumber", []string{fmt.Sprintf("%d", gid)})
	add.Attribute("homeDirectory", []string{fmt.Sprintf("/home/%s", uid)})
	add.Attribute("sn", []string{sn})
	add.Attribute("uid", []string{uid})
	add.Attribute("uidNumber", []string{fmt.Sprintf("%d", uidNumber)})
	add.Attribute("userPassword", []string{pw})
	err = l.Add(add)
	if err != nil {
		fmt.Printf("add failed: %s\n", err)
		return err
	}
	fmt.Printf("Created user %s\n", cn)
	return nil
}

func getNextFreeUidNumber(l *ldap.Conn) (int, error) {
	ldapClass := "posixAccount"
	fmt.Printf("Searching for class %s\n", ldapClass)
	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		*ldaporg,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=%s))", ldapClass),
		[]string{"uidNumber", "gidNumber"},
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		fmt.Printf("Failed to do search for %s: %s\n", ldapClass, err)
		return 0, err
	}
	var tak []int
	for _, e := range sr.Entries {
		uidS := e.GetAttributeValue("uidNumber")
		gidS := e.GetAttributeValue("gidNumber")
		uid, err := strconv.Atoi(uidS)
		if err != nil {
			fmt.Printf("Warning - dodgy uid: %s (%s)\n", uid, err)
			continue
		}
		gid, err := strconv.Atoi(gidS)
		if err != nil {
			fmt.Printf("Warning - dodgy gid: %s (%s)\n", gid, err)
			continue
		}
		tak = taken(uid, tak)
		tak = taken(gid, tak)
		fmt.Printf("UID: %d, GID: %d (%d)\n", uid, gid, len(tak))
	}
	for i := 10000; i < 10500; i++ {
		if !isTaken(i, tak) {
			return i, nil
		}
	}
	return 0, errors.New("no more free uids")
}

func isTaken(id int, ar []int) bool {
	for _, i := range ar {
		if i == id {
			return true
		}
	}
	return false
}
func taken(id int, ar []int) []int {
	z := append(ar, id)
	return z
}
