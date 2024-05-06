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

	"github.com/suconghou/libm3u8/fifoset"
	"github.com/suconghou/libm3u8/multipipe"
	"github.com/suconghou/libm3u8/util"
)

type TS struct {
	duration float64
	url      string
	xmuri    string
}

// M3U8 resource
type M3U8 struct {
	l      *fifoset.FIFOSet
	ts     chan *TS
	err    chan error
	hasErr error
}

// 每次读取分析一片playlist，返回nil则终止,若读取到#EXT-X-ENDLIST则也终止,ReadCloser读取异常则将终止
// 程序按行解析，忽略最近的重复行，忽略`#EXT-X-`相关
func New(r func() (io.ReadCloser, error), formater func(string) string) *M3U8 {
	m := &M3U8{fifoset.NewFIFOSet(200), make(chan *TS, 5), make(chan error, 5), nil}
	go func() {
		defer close(m.ts)
		defer close(m.err)
		for {
			n := time.Now()
			a, err := r()
			if err != nil {
				m.hasErr = err
				m.err <- err
				return
			}
			if a == nil {
				return
			}
			s := bufio.NewScanner(a)
			defer a.Close()
			var t float64
			var xm string
			for s.Scan() {
				line := strings.TrimSpace(s.Text())
				if line == "#EXT-X-ENDLIST" {
					return
				}
				if strings.HasPrefix(line, "#EXT-X-MAP") {
					xm = strings.Split(line, "\"")[1]
				}
				if line == "" || strings.HasPrefix(line, "#EXTM3U") || strings.HasPrefix(line, "#EXT-X-") {
					continue
				}
				if strings.HasPrefix(line, "#EXTINF") {
					if x, err := value(line); err != nil {
						m.hasErr = err
						m.err <- err
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
						if xm != "" {
							xm = formater(xm)
						}
					}
					m.ts <- &TS{t, line, xm}
					m.l.Add(line)
				}
			}
			if err := s.Err(); err != nil {
				m.hasErr = err
				m.err <- err
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
		select {
		case err := <-m.err:
			return "", err
		case ts, ok := <-m.ts:
			if ok {
				return ts.url, nil
			}
			return "", io.EOF
		}
	}, loader)
}

// 按序返回所有ts地址
func (m *M3U8) List() <-chan *TS {
	return m.ts
}

// 返回是否有错误
func (m *M3U8) Err() error {
	return m.hasErr
}

// NewFromReader 从reader中读取输入行，程序按行解析，忽略最近的重复行，忽略`#EXT-X-`相关，读取到EOF则退出
func NewFromReader(r io.ReadCloser, formater func(string) string) *M3U8 {
	var x = false
	return New(func() (io.ReadCloser, error) {
		if x {
			return nil, nil
		}
		x = true
		return r, nil
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
		fn  = func() (io.ReadCloser, error) {
			url = nextURL()
			if url == "" {
				return nil, nil
			}
			return util.GetBody(url)
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

func (t *TS) MAP() string {
	return t.xmuri
}

func (t *TS) Bytes(m bool) ([]byte, error) {
	var (
		times uint8
		err   error
		body  io.ReadCloser
		b     []byte
		u     = t.url
	)
	if m && t.xmuri != "" {
		u = t.xmuri
	}
	for ; times < 5; times++ {
		body, err = util.GetBody(u)
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
