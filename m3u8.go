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

	"github.com/suconghou/libm3u8/multipipe"
	"github.com/suconghou/libm3u8/util"
)

// M3U8 resource
type M3U8 struct {
	*io.PipeReader
	stream *io.PipeReader
}

// NewFromReader 从reader中读取输入行
func NewFromReader(r io.ReadCloser, formater func(string) string) *M3U8 {
	return &M3U8{pipeThrough(r, formater), nil}
}

// NewFromFile 从文件中读取输入行
func NewFromFile(path string, formater func(string) string) (*M3U8, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewFromReader(file, formater), nil
}

// NewFromURL 根据返回的URL（返回值空时则正常终止）,下载解析URL body作为输入行
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
			_, err = w.Write([]byte("\n"))
			if err != nil {
				w.CloseWithError(err)
				return
			}
			body, err = util.GetBody(url)
			if err != nil {
				w.CloseWithError(err) // GetBody内部已重试多次，仍失败，放弃吧
				return
			}
			_, err = io.Copy(w, io.TeeReader(body, &buf))
			body.Close()
			if err != nil {
				w.CloseWithError(err)
				return
			}
			t, last := parse(&buf)
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
func pipeThrough(rr io.ReadCloser, formater func(string) string) *io.PipeReader {
	var scanner = bufio.NewScanner(rr)
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		defer rr.Close()
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
			if _, err := w.Write([]byte(line + "\n")); err != nil {
				w.CloseWithError(err)
				return
			}
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

func parse(buf *bytes.Buffer) (int, bool) {
	if bytes.HasPrefix(bytes.TrimSpace(buf.Bytes()), []byte("#EXT-X-ENDLIST")) {
		return 0, true
	}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "#") {
			continue
		}
		if ok, v := value(line, "#EXT-X-TARGETDURATION"); ok {
			if t, err := strconv.Atoi(v); err == nil {
				return t, false
			}
		}
	}
	//既不存在ENDLIST也不存在TARGETDURATION，则认为是不合规的m3u8片段，则终止掉
	return 0, true
}

func value(line string, k string) (bool, string) {
	if strings.HasPrefix(line, k) {
		str := strings.Replace(line, k+":", "", 1)
		return true, strings.TrimSpace(str)
	}
	return false, ""
}

// return ts file streaming
func (m *M3U8) Stream(loader func(string) (io.ReadCloser, error)) *io.PipeReader {
	if m.stream != nil {
		return m.stream
	}
	var scanner = bufio.NewScanner(m)
	m.stream = multipipe.ConcatReaderByURL(func() string {
		if scanner.Scan() {
			return scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			util.Log.Print(err)
		}
		return ""
	}, loader)
	return m.stream
}
