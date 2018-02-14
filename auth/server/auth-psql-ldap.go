package main

// authenticate password against ldap
// save rest in postgres

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/GuruSystems/framework/auth"
	pb "github.com/GuruSystems/framework/proto/auth"
)

type PsqlLdapAuthenticator struct {
	dbcon       *sql.DB
	dbinfo      string
	connDetails map[string]string
}
type dbUser struct {
	a      *auth.User
	ldapcn string
}

// return the userid if found
func (pga *PsqlLdapAuthenticator) Authenticate(token string) (string, error) {
	fmt.Printf("Attempting to authenticate token \"%s\"\n", token)
	rows, err := pga.dbcon.Query("SELECT userid FROM usertoken where token = $1", token)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var uid int
		err = rows.Scan(&uid)
		if err != nil {
			return "", errors.New("error scanning row")
		}
		return fmt.Sprintf("%d", uid), nil

	}

	return "", errors.New(fmt.Sprintf("Not a valid token: \"%s\"", token))
}

func NewLdapPsqlAuthenticator() (auth.Authenticator, error) {
	var err error
	var now string
	host := *dbhost
	username := *dbuser
	database := *dbdb
	password := *dbpw
	fmt.Printf("Connecting to host %s\n", host)

	res := PsqlLdapAuthenticator{}

	res.dbinfo = fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=require",
		host, username, password, database)
	res.dbcon, err = sql.Open("postgres", res.dbinfo)
	if err != nil {
		fmt.Printf("Failed to connect to %s on host \"%s\" as \"%s\"\n", database, host, username)
		return nil, err
	}
	err = res.dbcon.QueryRow("SELECT NOW() as now").Scan(&now)
	if err != nil {
		fmt.Printf("Failed to scan %s on host \"%s\" as \"%s\"\n", database, host, username)
		return nil, err
	}
	fmt.Printf("Time in database: %s\n", now)
	return &res, nil
}

// we authenticate a user by email & password.
// we lookup email in postgres. if none we look up email as ldapcn.
// if there's neither we fail. Otherwise we use ldap to authenticate against ldap, otherwise fail
func (pga *PsqlLdapAuthenticator) CreateVerifiedToken(email string, pw string) string {
	uid := pga.getUserIDfromEmail(email)
	if uid == "" {
		fmt.Printf("User \"%s\" has no id (does not exist in database)\n", email)
		return ""
	}

	dbu, err := pga.getUser(uid)
	if err != nil {
		fmt.Printf("User %s with id %s has no user?\n", email, uid)
		return ""
	}
	cn := dbu.ldapcn
	tk := CheckLdapPassword(cn, pw)
	if tk == "" {
		fmt.Printf("Email %s (cn=%s) failed ldap authentication\n", cn, email)
		return ""
	}
	fmt.Printf("User \"%s\" has id %s\n", email, uid)
	err = pga.addTokenToUser(uid, tk, 10*365*24*60*60) // valid 10 years...
	if err != nil {
		fmt.Printf("Failed to add token to user: %s\n", err)
		return ""
	}
	fmt.Printf("Token: %s\n", tk)
	return tk
}

// given a userid returns user struct
func (pga *PsqlLdapAuthenticator) GetUserDetail(userid string) (*auth.User, error) {
	u, err := pga.getUser(userid)
	if err != nil {
		return nil, err
	}
	return u.a, nil
}

func (pga *PsqlLdapAuthenticator) getUserIDfromEmail(email string) string {
	var userid int
	rows, err := pga.dbcon.Query("SELECT id FROM usertable where email = $1 or ldapcn = $1", email)
	if err != nil {
		fmt.Printf("Error quering database: %s\n", err)
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&userid)
		if err != nil {
			fmt.Printf("Failed to scan row: %s\n", err)
			return ""
		}
		return fmt.Sprintf("%d", userid)
	}
	return ""
}

// given a userid this will retrieve the corresponding row from db and return user struct
func (pga *PsqlLdapAuthenticator) getUser(userid string) (*dbUser, error) {

	rows, err := pga.dbcon.Query("SELECT id,firstname,lastname,email,ldapcn FROM usertable where id = $1", userid)
	if err != nil {
		s := fmt.Sprintf("Error quering database: %s\n", err)
		return nil, errors.New(s)
	}
	defer rows.Close()
	for rows.Next() {
		user := auth.User{}
		dbUser := dbUser{a: &user}
		err = rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &dbUser.ldapcn)
		if err != nil {
			s := fmt.Sprintf("Failed to scan row: %s\n", err)
			return nil, errors.New(s)
		}
		return &dbUser, nil
	}
	return nil, errors.New("No matching user found")
}

// given a userid this will create a token and add it to the useraccount
func (pga *PsqlLdapAuthenticator) addTokenToUser(userid string, token string, validsecs int) error {
	_, err := pga.dbcon.Exec("insert into usertoken (token,userid) values ($1,$2)", token, userid)
	if err != nil {
		fmt.Printf("Error inserting usertoken: %s\n", err)
		return err
	}
	return nil

}
func (pga *PsqlLdapAuthenticator) CreateUser(c *pb.CreateUserRequest) (string, error) {
	pw := c.Password
	if pw == "" {
		pw = RandomString(64)
	}
	err := CreateLdapUser(c.UserName, c.LastName, c.UserName, pw)
	/*
		// continue anyways, perhaps botched 1. attempt and this is second?
				if err != nil {
					return "", err
				}
	*/
	_, err = pga.dbcon.Exec("insert into usertable (firstname,lastname,email,ldapcn) values ($1,$2,$3,$4)", c.FirstName, c.LastName, c.Email, c.UserName)
	if err != nil {
		return "", err
	}
	return pw, nil
}
