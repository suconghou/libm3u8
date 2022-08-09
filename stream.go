package libm3u8

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"libm3u8/util"
)

// NewFromReader 从reader中读取输入行
func NewFromReader(r io.Reader, formater func(string) string) *M3U8 {
	return &M3U8{Reader: pipeThrough(bufio.NewScanner(r), formater)}
}

// NewFromFile 从文件中读取输入行
func NewFromFile(path string, formater func(string) string) (*M3U8, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewFromReader(file, formater), nil
}

// NewFromURL 根据返回的URL,下载解析URL body作为输入行
func NewFromURL(nextURL func() string) *M3U8 {
	r, w := io.Pipe()
	url := nextURL()
	go func(w *io.PipeWriter) {
		var (
			body  io.ReadCloser
			err   error
			buf   bytes.Buffer
			timer = time.Now()
		)
		for {
			if url == "" {
				w.Close()
				return
			}
			w.Write([]byte("\n"))
			body, err = util.GetBody(url)
			if err != nil {
				w.CloseWithError(err) // get response failed many times then exit
				return
			}
			_, err = io.Copy(w, io.TeeReader(body, &buf))
			body.Close()
			if err != nil {
				w.CloseWithError(err)
				return
			}
			t, last := getSegmentInfo(&buf)
			buf.Reset()
			if last {
				w.Close()
				return
			}
			st := int64((float64(t) - time.Since(timer).Seconds()) * 1000)
			if st > 0 {
				time.Sleep(time.Duration(st) * time.Millisecond)
			}
			timer = time.Now()
			url = nextURL()
		}
	}(w)
	var (
		base     = strings.Replace(path.Dir(url), ":/", "://", 1) + "/"
		ur       = regexp.MustCompile(`^(?i:https?)://[[:print:]]{4,}$`)
		basePart = strings.SplitAfterN(base, "/", 4)
	)
	m := NewFromReader(r, func(u string) string {
		if ur.MatchString(u) {
			return u
		}
		if strings.HasPrefix(u, "/") {
			basePart[3] = strings.TrimLeft(u, "/")
			return strings.Join(basePart, "")
		}
		return base + u

	})
	return m
}

// 从scanner中读取行，过滤掉注释和重复的行，返回不重复的行
func pipeThrough(scanner *bufio.Scanner, formater func(string) string) io.Reader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		urls := map[string]bool{}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if formater != nil {
				line = formater(line)
				if line == "" {
					continue
				}
			}
			if urls[line] {
				continue
			}
			w.Write([]byte(line + "\n"))
			urls[line] = true
		}
		if err := scanner.Err(); err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	}(w)
	return r
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
