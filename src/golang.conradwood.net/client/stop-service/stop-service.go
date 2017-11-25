package main

import (
	"flag"
	"fmt"
	"net/http"
	//"golang.conradwood.net/client"
	"crypto/tls"
	"os"
	"strings"
)

var (
	service = flag.String("service", "", "Service to shutdown, either host:port or name")
)

func main() {
	flag.Parse()
	if *service == "" {
		fmt.Printf("Please provide service name or host:port with --service option\n")
		os.Exit(10)
	}
	if strings.Contains(*service, ":") {
		doHttp()
	} else {
		doRegistry()
	}
}
func doHttp() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	url := fmt.Sprintf("https://%s/internal/pleaseshutdown", *service)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %s\n", url, err)
	} else {
		fmt.Printf("Response: %s\n", resp)
	}
}
func doRegistry() {

}
