package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/tidwall/gjson"
)

// authClient contains the HTTP client components
type authClient struct {
	Username   string
	Password   string
	HTTPClient *http.Client
}

// newBasicAuthClient returns an instance of authClient
func newBasicAuthClient(username, password string) *authClient {
	return &authClient{
		Username:   username,
		Password:   password,
		HTTPClient: httpAuthClient(),
	}
}

// getJSON expects a URL and a top-level json dict name to scrape.  It returns
// that dict name as a gjson object.
func (s *authClient) getJSON(url, tlj string) (gjson.Result, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("request error: %v", err)
	}
	bytes, err := s.doRequest(req)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("node request error: %v", err)
	}
	return gjson.GetBytes(bytes, tlj), nil
}

// httpAuthClient creates a new instance of http.Client with support for
// additional rootCAs.  As XClarity is frequently installed as an appliance,
// with a self-signed cert, this appears to be quite useful.
func httpAuthClient() *http.Client {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		log.Fatal(err)
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	certs, err := ioutil.ReadFile(cfg.API.CertFile)
	if err != nil {
		log.Println("No additional certificates imported")
	} else if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("No certs imported.  Proceeding with system CAs.")
	}
	config := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            rootCAs,
	}
	tr := &http.Transport{TLSClientConfig: config}
	return &http.Client{Transport: tr}
}

// doRequest does an HTTP URL request and returns it as a byte array
func (s *authClient) doRequest(req *http.Request) ([]byte, error) {
	req.SetBasicAuth(s.Username, s.Password)
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		log.Fatalf("HTTP request error: %v\n", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Response error: %v\n", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Status error: %s\n", string(body))
	}
	return body, nil
}
