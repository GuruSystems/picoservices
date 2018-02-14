package main

// TODO: how/when do we close database connections? (pooling?)

// in a dir we have
// [bla].token where [bla] is a valid user token
//    these files contain a single line with the userid this token belongs to
// [bla].user where [bla] is a user id
//    these files contain lines: userid/firstname/lastname/email/
import (
	"bufio"
	"errors"
	"fmt"
	"github.com/GuruSystems/framework/auth"
	pb "github.com/GuruSystems/framework/proto/auth"
	"io/ioutil"
	"os"
	"strings"
)

type FileAuthenticator struct {
	dir string
}

type userFile struct {
	a  *auth.User
	pw string
}

// given a token will look for a file called "bla.token"
// reads it and parses it -> returns userid
func (fa *FileAuthenticator) Authenticate(token string) (string, error) {
	var read []string
	if (strings.Contains(token, "/")) || (strings.Contains(token, "~")) {
		return "", errors.New(fmt.Sprintf("invalid token: \"%s\"", token))
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
	u, err := fa.readUid(userid)
	if err != nil {
		return nil, err
	}
	return u.a, nil
}
func (fa *FileAuthenticator) readUid(userid string) (*userFile, error) {
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
	if len(read) < 5 {
		return nil, errors.New("Invalid user file - does not contain enough lines")
	}
	a := &userFile{
		a: &auth.User{
			ID:        read[0],
			FirstName: read[1],
			LastName:  read[2],
			Email:     read[3],
		},
		pw: read[4],
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

func (pga *FileAuthenticator) CreateVerifiedToken(email string, pw string) string {
	df, err := ioutil.ReadDir(pga.dir)
	if err != nil {
		fmt.Printf("Failed to read directory \"%s\": %s\n,", pga.dir, err)
		return ""
	}
	for _, file := range df {
		if !strings.HasSuffix(file.Name(), ".user") {
			continue
		}
		uid := strings.TrimSuffix(file.Name(), ".user")
		au, err := pga.readUid(uid)
		if err != nil {
			fmt.Println("Error: ", err)
			return ""
		}
		if au.a.Email == email {
			if au.pw == "" {
				fmt.Println("user has no password set")
				return ""
			}
			if pw != au.pw {
				fmt.Println("Found user but password mismatch")
				return ""
			}
			// got match - yeah
			fmt.Printf("Creating Token for user %v\n", au.a)
			return CreateTokenInFileSystem(pga.dir, au.a)

		}
		fmt.Printf("File: %s, uid=%s\n", file.Name(), uid)
	}
	return ""
}

func CreateTokenInFileSystem(dir string, au *auth.User) string {
	tk := RandomString(30)
	err := os.Chdir(dir)
	if err != nil {
		fmt.Printf("Failed to chdir to %s: %s\n", dir, err)
		return ""
	}
	s1 := fmt.Sprintf("%s.token", tk)
	s2 := fmt.Sprintf("%s.user", au.ID)
	os.Symlink(s2, s1)
	if err != nil {
		fmt.Printf("Failed to symlink to %s: %s\n", s1, s2)
		return ""
	}
	return tk
}
func (pga *FileAuthenticator) CreateUser(*pb.CreateUserRequest) (string, error) {
	return "", errors.New("CreateUser() not yet implemented")
}
