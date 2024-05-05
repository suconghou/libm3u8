package libm3u8

import (
	"bufio"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"libm3u8/lruset"
	"libm3u8/multipipe"
	"libm3u8/util"
)

type TS struct {
	duration float64
	url      string
}

// M3U8 resource
type M3U8 struct {
	l  *lruset.LRUSet
	ts chan *TS
}

// 每次读取分析一片playlist，返回nil则终止,若读取到#EXT-X-ENDLIST则也终止,ReadCloser读取异常则将终止
// 程序按行解析，忽略最近的重复行，忽略`#EXT-X-`相关
func New(r func() io.ReadCloser, formater func(string) string) *M3U8 {
	m := &M3U8{lruset.NewLRUSet(200), make(chan *TS, 5)}
	go func() {
		defer close(m.ts)
		for {
			n := time.Now()
			a := r()
			if a == nil {
				return
			}
			s := bufio.NewScanner(a)
			defer a.Close()
			var t float64
			for s.Scan() {
				line := strings.TrimSpace(s.Text())
				if line == "#EXT-X-ENDLIST" {
					return
				}
				if line == "" || strings.HasPrefix(line, "#EXTM3U") || strings.HasPrefix(line, "#EXT-X-") {
					continue
				}
				if strings.HasPrefix(line, "#EXTINF") {
					if x, err := value(line); err != nil {
						util.Log.Print(err)
						return
					} else {
						t = x
					}
				} else {
					if m.l.Exists(line) {
						continue
					}
					if formater != nil {
						line = formater(line)
						if line == "" {
							continue
						}
					}
					m.ts <- &TS{t, line}
					m.l.Add(line)
				}
			}
			if s.Err() != nil {
				util.Log.Print(s.Err())
				return
			}
			time.Sleep(time.Second.Truncate(time.Since(n)))
		}
	}()
	return m
}

// 返回ts文件合成流
func (m *M3U8) Stream(loader func(string) (io.ReadCloser, error)) *io.PipeReader {
	return multipipe.ConcatReaderByURL(func() (string, error) {
		ts, ok := <-m.ts
		if ok {
			return ts.url, nil
		}
		return "", io.EOF
	}, loader)
}

// 按序返回所有ts地址
func (m *M3U8) List() <-chan *TS {
	return m.ts
}

// NewFromReader 从reader中读取输入行，程序按行解析，忽略最近的重复行，忽略`#EXT-X-`相关，读取到EOF则退出
func NewFromReader(r io.ReadCloser, formater func(string) string) *M3U8 {
	var x = false
	return New(func() io.ReadCloser {
		if x {
			return nil
		}
		x = true
		return r
	}, formater)
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
	var (
		url = nextURL()
		fn  = func() io.ReadCloser {
			url = nextURL()
			if url == "" {
				return nil
			}
			body, err := util.GetBody(url)
			if err != nil {
				util.Log.Print(err)
				return nil
			}
			return body
		}
		base     = strings.Replace(path.Dir(url), ":/", "://", 1) + "/"
		ur       = regexp.MustCompile(`^(?i:https?)://[[:print:]]{4,}$`)
		basePart = strings.SplitAfterN(base, "/", 4)
		formater = func(u string) string {
			if ur.MatchString(u) {
				return u
			}
			if strings.HasPrefix(u, "/") {
				basePart[3] = strings.TrimLeft(u, "/")
				return strings.Join(basePart, "")
			}
			return base + u
		}
	)
	return New(fn, formater)
}

func value(line string) (float64, error) {
	colonIndex := strings.IndexByte(line, ':')
	commaIndex := strings.IndexByte(line, ',')
	if colonIndex == -1 {
		colonIndex = 0
	}
	if commaIndex == -1 {
		commaIndex = len(line) - 1
	}
	timeString := strings.TrimSpace(line[colonIndex+1 : commaIndex])
	return strconv.ParseFloat(timeString, 64)
}

func (t *TS) URL() string {
	return t.url
}

func (t *TS) Bytes() ([]byte, error) {
	var (
		times uint8
		err   error
		body  io.ReadCloser
		b     []byte
	)
	for ; times < 5; times++ {
		body, err = util.GetBody(t.url)
		if err == nil {
			b, err = io.ReadAll(body)
			body.Close()
			if err == nil {
				return b, nil
			}
		}
	}
	return b, err
}

func (t *TS) Duration() float64 {
	return t.duration
}
