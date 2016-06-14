/* Testing basic authentication and get.
This script simply gets info in JSON format from my test server, but does not do anything 
with the JSON other than print to screen. */


package main

import (
	"fmt"
	"net/http"
	"crypto/tls"
	"io/ioutil"
	"os"
)

var (
	url string 
	resource string
	)


func main () {
	
	tr := &http.Transport {
		TLSClientConfig: &tls.Config{InsecureSkipVerify : true},
	}

	client := &http.Client{Transport: tr}  // Create an http client
	
	/* Build a request that does auth */
	url = "https://share.4dtech.us:4285"
	resource = "/api/computer"
	fullPath := url+resource

	req, err := http.NewRequest("GET", fullPath, nil)
	req.SetBasicAuth("username", "password")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error : %s", err)
	}

	/* Get the results */
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%s", err )
		os.Exit(1)
		}

	fmt.Printf("%s\n", string(contents))
	
//	fmt.Println(resp)

	}

