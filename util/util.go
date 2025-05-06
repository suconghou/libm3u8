package util

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	// Log to stderr
	Log    = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	client = &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   3 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
)

// GetResp try max 3 times to get http response and make sure 200-299
func GetResp(url string) (*http.Response, error) {
	var (
		resp  *http.Response
		err   error
		times uint8
	)
	for ; times < 3; times++ {
		resp, err = client.Get(url)
		if err == nil {
			if resp.StatusCode/100 == 2 {
				break
			} else {
				resp.Body.Close()
				err = fmt.Errorf("%s %s : %s", resp.Request.Method, resp.Request.URL.String(), resp.Status)
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
