package main

// TODO: how/when do we close database connections? (pooling?)

import (
	"fmt"
	"flag"
	"errors"
	"database/sql"
	//
	_ "github.com/lib/pq"
	//
	"github.com/GuruSystems/framework/auth"
	pb "github.com/GuruSystems/framework/proto/auth"
)

var (
	dbhost = flag.String("dbhost", "postgres", "hostname of the postgres database rdms")
	dbdb   = flag.String("database", "rpcusers", "database to use for authentication")
	dbuser = flag.String("dbuser", "root", "username for the database to use for authentication")
	dbpw   = flag.String("dbpw", "pw", "password for the database to use for authentication")
)

type PostGresAuthenticator struct {
	dbcon       *sql.DB
	dbinfo      string
	connDetails map[string]string
}

func (pga *PostGresAuthenticator) Authenticate(token string) (string, error) {
	fmt.Printf("Attempting to authenticate token \"%s\"\n", token)
	rows, err := pga.dbcon.Query("SELECT usertable.id as uid,email,firstname,lastname FROM usertoken,usertable where usertoken.userid = usertable.id")
	if err != nil {
		return "", err
	}
	for rows.Next() {
		user := auth.User{}
		err = rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email)
		if err != nil {
			return "", errors.New("error scanning row")
		}
		fmt.Println("uid | username | department | created ")
		fmt.Printf("%3v | %8v | %6v | %6v\n", &user.ID, &user.FirstName, &user.LastName, &user.Email)
	}

	return "", errors.New(fmt.Sprintf("Not a valid token: \"%s\"", token))
}

func NewPostgresAuthenticator() (auth.Authenticator, error) {
	var err error
	var now string
	host := *dbhost
	username := *dbuser
	database := *dbdb
	password := *dbpw
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
func (pga *PostGresAuthenticator) CreateVerifiedToken(email string, pw string) string {
	uid := pga.getUserIDfromEmail(email)
	if uid == "" {
		fmt.Printf("User \"%s\" has no id (does not exist in database)\n", email)
		return ""
	}
	fmt.Printf("User \"%s\" has id %s\n", email, uid)
	return ""
}
func (pga *PostGresAuthenticator) GetUserDetail(user string) (*auth.User, error) {
	return nil, errors.New("Not implemented")
}

func (pga *PostGresAuthenticator) getUserIDfromEmail(email string) string {
	var userid int
	rows, err := pga.dbcon.Query("SELECT id FROM usertable where email = $1", email)
	if err != nil {
		fmt.Printf("Error quering database: %s\n", err)
		return ""
	}
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
func (pga *PostGresAuthenticator) CreateUser(*pb.CreateUserRequest) (string, error) {
	return "", errors.New("CreateUser() not yet implemented")
}
