package libm3u8

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

const tryTimes uint8 = 5

var (
	mlog   = log.New(os.Stderr, "", log.Lshortfile)
	urlreg = regexp.MustCompile(`^(?i:https?)://[[:print:]]+$`)
	client = &http.Client{Timeout: time.Duration(60) * time.Second}
)

func isURL(url string) bool {
	return urlreg.MatchString(url)
}

func respOk(resp *http.Response) bool {
	return resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed
}

func getResp(url string, tryTimes uint8) (*http.Response, error) {
	var (
		resp  *http.Response
		err   error
		times uint8
	)
	for {
		resp, err = client.Get(url)
		times++
		if err == nil {
			if respOk(resp) {
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

func getContent(url string, tryTimes uint8) ([]byte, error) {
	resp, err := getResp(url, tryTimes)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// parse string like #EXT-X-MEDIA-SEQUENCE:1586
func getValue(line string, k string) (bool, string) {
	if strings.HasPrefix(line, k) {
		str := strings.Replace(line, k+":", "", 1)
		return true, str
	}
	return false, ""
}
