package libm3u8

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/suconghou/libm3u8/parser"
	"github.com/suconghou/libm3u8/util"
)

// NewFromReader reader to parse url list
func NewFromReader(r io.Reader, formater func(string) string) *M3U8 {
	return &M3U8{Reader: parser.Parse(bufio.NewScanner(r), formater)}
}

// NewFromFile parse file content
func NewFromFile(path string, formater func(string) string) (*M3U8, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewFromReader(file, formater), nil
}

// NewFromURL return m3u8
func NewFromURL(nextURL func() string) *M3U8 {
	r, w := io.Pipe()
	url := nextURL()
	go func(w *io.PipeWriter) {
		var (
			body  io.ReadCloser
			err   error
			buf   bytes.Buffer
			timer time.Time = time.Now()
		)
		for {
			if url == "" {
				w.CloseWithError(io.EOF)
				return
			}
			body, err = util.GetBody(url)
			if err != nil {
				util.Log.Print(err)
				w.CloseWithError(err) // get response failed many times then exit
				return
			}
			_, err = io.Copy(w, io.TeeReader(body, &buf))
			body.Close()
			if err != nil {
				util.Log.Print(err) // copy failed but we check content by getSegmentInfo ensure if exit
			}
			t, last := getSegmentInfo(&buf)
			buf.Reset()
			if last {
				if err == nil {
					w.CloseWithError(io.EOF)
				} else {
					w.CloseWithError(err)
				}
				return
			}
			st := int64(float64(t)-time.Since(timer).Seconds()) * 1000
			if st > 0 {
				time.Sleep(time.Duration(st) * time.Millisecond)
			}
			timer = time.Now()
			url = nextURL()
		}
	}(w)
	var base = strings.Replace(path.Dir(url), ":/", "://", 1) + "/"
	m := NewFromReader(r, func(u string) string {
		if strings.HasPrefix(u, "http") {
			return u
		}
		return path.Join(base, u)
	})
	return m
}

func getSegmentInfo(buf *bytes.Buffer) (int, bool) {
	if bytes.HasSuffix(bytes.TrimSpace(buf.Bytes()), []byte("#EXT-X-ENDLIST")) {
		return 0, true
	}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "#") {
			continue
		}
		if ok, v := getLineValue(line, "#EXT-X-TARGETDURATION"); ok {
			if t, err := strconv.Atoi(v); err == nil {
				return t, false
			}
		}
	}
	return 0, true // not found endList or duration mybe content is not m3u8 response flag to exit
}

func getLineValue(line string, k string) (bool, string) {
	if strings.HasPrefix(line, k) {
		str := strings.Replace(line, k+":", "", 1)
		return true, strings.TrimSpace(str)
	}
	return false, ""
}
