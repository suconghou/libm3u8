package util

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	// Log to stderr
	Log    = log.New(os.Stderr, "", log.Lshortfile)
	client = &http.Client{
		Timeout: time.Minute * 5,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
)

// GetResp try max 5 time to get http response and make sure 200-299
func GetResp(url string) (*http.Response, error) {
	var (
		resp  *http.Response
		err   error
		times uint8
	)
	for ; times < 5; times++ {
		resp, err = client.Get(url)
		if err == nil {
			if resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed {
				break
			} else {
				err = fmt.Errorf(resp.Status)
			}
		}
		time.Sleep(time.Millisecond)
	}
	return resp, err
}

// GetBody return http response body
func GetBody(url string) (io.ReadCloser, error) {
	resp, err := GetResp(url)
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// GetBodyContent read http url 200 response body
func GetBodyContent(url string) ([]byte, error) {
	body, err := GetBody(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}
