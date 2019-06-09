package util

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	Log    = log.New(os.Stderr, "", log.Lshortfile)
	urlreg = regexp.MustCompile(`^(?i:https?)://[[:print:]]{4,}$`)
	client = &http.Client{Timeout: time.Duration(60) * time.Second}
)

func IsURL(url string) bool {
	return urlreg.MatchString(url)
}

func RespOk(resp *http.Response) bool {
	return resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed
}

func GetResp(url string, tryTimes uint8) (*http.Response, error) {
	var (
		resp  *http.Response
		err   error
		times uint8
	)
	for {
		resp, err = client.Get(url)
		times++
		if err == nil {
			if RespOk(resp) {
				break
			} else {
				err = fmt.Errorf(resp.Status)
			}
		}
		if times > tryTimes {
			break
		}
		time.Sleep(time.Millisecond)
	}
	return resp, err
}

func GetContent(url string, tryTimes uint8) ([]byte, error) {
	resp, err := GetResp(url, tryTimes)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// parse string like #EXT-X-MEDIA-SEQUENCE:1586
func GetValue(line string, k string) (bool, string) {
	if strings.HasPrefix(line, k) {
		str := strings.Replace(line, k+":", "", 1)
		return true, str
	}
	return false, ""
}
