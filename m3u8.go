package libm3u8

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/suconghou/libm3u8/parser"
	"github.com/suconghou/libm3u8/util"
)

const tryTimes uint8 = 5

const (
	endList  = "#EXT-X-ENDLIST"
	inf      = "#EXTINF"
	duration = "#EXT-X-TARGETDURATION"
)

// M3U8 resource
type M3U8 struct {
	io.Reader
	base string
}

// NewFromURL return m3u8
func NewFromURL(nextURL func() string) *M3U8 {
	r, w := io.Pipe()
	url := nextURL()
	go func(w *io.PipeWriter) {
		var (
			resp  *http.Response
			err   error
			buf   bytes.Buffer
			timer time.Time = time.Now()
		)
		for {
			if url == "" {
				w.CloseWithError(io.EOF)
				return
			}
			resp, err = util.GetResp(url, tryTimes)
			if err != nil {
				util.Log.Print(err)
				w.CloseWithError(err) // get response failed many times then exit
				return
			}
			_, err = io.Copy(w, io.TeeReader(resp.Body, &buf))
			resp.Body.Close()
			if err != nil {
				util.Log.Print(err) // copy failed but we check content by getWaitTime ensure if exit
			}
			t, last := getWaitTime(&buf)
			buf.Reset()
			if last {
				if err == nil {
					w.CloseWithError(io.EOF)
				} else {
					w.CloseWithError(err)
				}
				return
			}
			t = (t - time.Since(timer).Seconds()) * 1000
			if t > 0 {
				duration := time.Duration(t) * time.Millisecond
				time.Sleep(duration)
			}
			timer = time.Now()
			url = nextURL()
		}
	}(w)
	m := NewFromReader(bufio.NewScanner(r))
	m.base = strings.Replace(path.Dir(url), ":/", "://", 1) + "/"
	return m
}

// NewFromFile parse file content
func NewFromFile(path string) (*M3U8, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewFromReader(bufio.NewScanner(file)), nil
}

// NewFromReader get data from reader
func NewFromReader(scanner *bufio.Scanner) *M3U8 {
	r := parser.Parse(scanner)
	return &M3U8{Reader: r}
}

// SetBaseURL set base url
func (m *M3U8) SetBaseURL(url string) {
	m.base = url
}

// Play ts file
func (m *M3U8) Play() io.Reader {
	return NewReader(bufio.NewScanner(m.PlayList()))
}

// PlayList get play list
func (m *M3U8) PlayList() io.Reader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		scanner := bufio.NewScanner(m)
		for scanner.Scan() {
			line := scanner.Text()
			w.Write([]byte(m.base + line + "\n"))
		}
		if err := scanner.Err(); err != nil {
			w.CloseWithError(err)
		} else {
			w.CloseWithError(io.EOF)
		}
	}(w)
	return r
}

func getWaitTime(buf *bytes.Buffer) (float64, bool) {
	by := bytes.TrimSpace(buf.Bytes())
	if bytes.HasSuffix(by, []byte(endList)) {
		return 0, true
	}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && strings.HasPrefix(line, duration) {
			_, value := util.GetValue(line, duration)
			if t, err := strconv.Atoi(value); err == nil {
				return float64(t), false
			}
		}
	}
	return 0, true // not found endList or duration mybe content is not m3u8 response flag to exit
}
