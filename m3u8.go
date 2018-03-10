package libm3u8

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
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
			resp *http.Response
			err  error
		)
		for {
			if url == "" {
				w.CloseWithError(io.EOF)
				return
			}
			resp, err = getResp(url, tryTimes)
			if err != nil {
				mlog.Print(err)
			}
			_, err = io.Copy(w, resp.Body)
			resp.Body.Close()
			if err != nil {
				mlog.Print(err)
			}
			time.Sleep(time.Second * 2)
			url = nextURL()
		}
	}(w)
	m := NewFromReader(bufio.NewScanner(r))
	m.base = strings.Replace(path.Dir(url), ":/", "://", 1)
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
	r := Parse(scanner)
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
			w.Write([]byte(m.base + line))
		}
	}(w)
	return r
}
