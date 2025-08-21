package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
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
	URL = regexp.MustCompile(`^(?i:https?)://[[:print:]]{4,}$`)
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
				err = errors.Join(resp.Body.Close(), fmt.Errorf("%s %s : %s", resp.Request.Method, resp.Request.URL, resp.Status))
			}
		}
		time.Sleep(time.Second)
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

// 输入需要为http绝对地址，返回基路径，结尾带/
func BaseURL(s string) string {
	// 首先需要去除?或者#, 从左向右遍历，找到?或者#则终止截断
	var n = len(s)
	for i := range n {
		if s[i] == '?' || s[i] == '#' {
			n = i
			break
		}
	}
	s = s[:n]
	//  倒序遍历，找到/,但是必须确保找到的/不是http://协议里的， 可以判断前8个字段是安全字符不查找
	for i := len(s) - 1; i >= 8; i-- {
		if s[i] == '/' {
			return s[:i+1]
		}
	}
	return s + "/"
}
