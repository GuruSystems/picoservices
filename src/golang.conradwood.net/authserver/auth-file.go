package main

// TODO: how/when do we close database connections? (pooling?)

// in a dir we have
// [bla].token where [bla] is a valid user token
//    these files contain a single line with the userid this token belongs to
// [bla].user where [bla] is a user id
//    these files contain lines: firstname/lastname/email/userid
import (
	"bufio"
	"errors"
	"fmt"
	"golang.conradwood.net/auth"
	"os"
	"strings"
)

type FileAuthenticator struct {
	dir string
	auth.Authenticator
}

// given a token will look for a file called "bla.token"
// reads it and parses it -> returns userid
func (fa *FileAuthenticator) Authenticate(token string) (string, error) {
	var read []string
	if (strings.Contains(token, "/")) || (strings.Contains(token, "~")) {
		return "", errors.New("invalid token")
	}
	fname := fmt.Sprintf("%s/%s.token", fa.dir, token)
	fmt.Printf("Looking for token in %s\n", fname)
	fileHandle, err := os.Open(fname)
	if err != nil {
		fmt.Printf("Unable to open %s: %s\n", fname, err)
		return "", err
	}
	defer fileHandle.Close()
	fileScanner := bufio.NewScanner(fileHandle)

	for fileScanner.Scan() {
		read = append(read, fileScanner.Text())
	}
	userid := read[0]
	fmt.Printf("token %s ==> user #%s\n", token, userid)
	return userid, nil
}
func (fa *FileAuthenticator) GetUserDetail(userid string) (*auth.User, error) {
	var read []string
	if (strings.Contains(userid, "/")) || (strings.Contains(userid, "~")) {
		return nil, errors.New("invalid userid")
	}
	fname := fmt.Sprintf("%s/%s.user", fa.dir, userid)
	fmt.Printf("Looking for userdetail in %s\n", fname)
	fileHandle, err := os.Open(fname)
	if err != nil {
		fmt.Printf("Unable to open %s: %s\n", fname, err)
		return nil, err
	}
	defer fileHandle.Close()
	fileScanner := bufio.NewScanner(fileHandle)

	for fileScanner.Scan() {
		read = append(read, fileScanner.Text())
	}

	a := &auth.User{
		FirstName: read[0],
		LastName:  read[1],
		Email:     read[2],
		ID:        read[3],
	}
	return a, nil
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
