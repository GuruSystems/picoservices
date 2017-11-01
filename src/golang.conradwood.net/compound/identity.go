package common

// TODO: how/when do we close database connections? (pooling?)

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq"
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

type PostGresAuthenticator struct {
	dbcon  *sql.DB
	dbinfo string
	Authenticator
	connDetails map[string]string
}

func (pga *PostGresAuthenticator) Authenticate(token string) (*User, error) {
	rows, err := pga.dbcon.Query("SELECT usertable.id as uid,email,firstname,lastname FROM usertoken,usertable where usertoken.userid = usertable.id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		user := User{}
		err = rows.Scan(&user.id, &user.FirstName, &user.LastName, &user.Email)
		if err == nil {
			return nil, errors.New("error scanning row")
		}
		fmt.Println("uid | username | department | created ")
		fmt.Printf("%3v | %8v | %6v | %6v\n", &user.id, &user.FirstName, &user.LastName, &user.Email)
	}

	return nil, errors.New(fmt.Sprintf("Not a valid token: \"%s\"", token))
}

func NewPostgresAuthenticator(host string, database string, username string, password string) (Authenticator, error) {
	var err error
	var now string
	fmt.Printf("Connecting to host %s\n", host)

	res := PostGresAuthenticator{}

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
