package libm3u8

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	mlog   = log.New(os.Stderr, "", 0)
	urlreg = regexp.MustCompile(`^[a-zA-z]+://[^\s]+$`)
)

type stream struct {
	duration float64
	start    float64
	url      string
	index    int32
}

// M3U8 resource
type M3U8 struct {
	url        string
	base       string
	nextURL    func() string
	offline    bool
	duration   float64
	parts      []*mpart
	streams    []*stream
	streamchan chan *stream
	play       *playItem
}

type playItem struct {
	w io.Writer
	r io.Reader
}

type mpart struct {
	partID         string
	targetDuration int
	streams        []*stream
	offline        bool
	fileNum        int32
	duration       float64
}

// NewFromURL return m3u8
func NewFromURL(url string, nextURL func() string) (*M3U8, error) {
	resp, err := getResp(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	part, err := parsePart(scanner)
	if err != nil {
		return nil, err
	}
	m := &M3U8{
		url:        url,
		base:       strings.Replace(path.Dir(url), ":/", "://", 1),
		nextURL:    nextURL,
		streamchan: make(chan *stream, 8192),
	}
	updateM3U8(m, part)
	if part.offline {
		close(m.streamchan)
		return m, nil
	}
	var partID = part.partID
	go func(m *M3U8) {
		for {
			part, err := parseUntil(m)
			if err != nil {
				mlog.Print(err)
				close(m.streamchan)
				return
			}
			if partID == part.partID {
				time.Sleep(time.Second)
				continue
			}
			partID = part.partID
			updateM3U8(m, part)
			if m.offline {
				close(m.streamchan)
				return
			}
			time.Sleep(time.Second * time.Duration(part.targetDuration))
		}
	}(m)
	return m, nil
}

// NewReader streams
func NewReader(scanner *bufio.Scanner) io.Reader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		for scanner.Scan() {
			url := scanner.Text()
			if isURL(url) {
				resp, err := getResp(url)
				if err != nil {
					mlog.Print(err)
				}
				defer resp.Body.Close()
				_, err = io.Copy(w, resp.Body)
				if err != nil {
					mlog.Print(err)
				}
			}
		}
		w.CloseWithError(io.EOF)
	}(w)
	return &playItem{r: r, w: w}
}

// GetDuration return media total duration
func (m *M3U8) GetDuration() (float64, bool) {
	return m.duration, m.offline
}

// GetAvailableList return all ts file link
func (m *M3U8) GetAvailableList() ([]string, float64, bool) {
	var avaible []string
	for _, item := range m.streams {
		avaible = append(avaible, item.url)
	}
	return avaible, m.duration, m.offline
}

func (m *M3U8) Read(p []byte) (int, error) {
	for {
		stream, more := <-m.streamchan
		if more {
			return bytes.NewBufferString((m.base + "/" + stream.url + "\n")).Read(p)
		}
		return 0, io.EOF
	}
}

// Play ts file
func (m *M3U8) Play() io.Reader {
	if m.play != nil {
		return m.play
	}
	r, w := io.Pipe()
	go func(m *M3U8) {
		for {
			stream, more := <-m.streamchan
			if more {
				u := m.base + "/" + stream.url
				resp, err := getResp(u)
				if err != nil {
					mlog.Print(err)
				}
				defer resp.Body.Close()
				_, err = io.Copy(w, resp.Body)
				if err != nil {
					mlog.Print(err)
				}
			} else {
				w.CloseWithError(io.EOF)
				return
			}
		}
	}(m)
	m.play = &playItem{r: r, w: w}
	return m.play
}

func parseUntil(m *M3U8) (*mpart, error) {
	var url = m.url
	if m.nextURL != nil {
		url = m.nextURL()
		if url == "" {
			return nil, fmt.Errorf("nextURL return eof")
		}
	}
	resp, err := getResp(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	part, err := parsePart(scanner)
	if err != nil {
		return part, err
	}
	return part, nil
}

func (play *playItem) Read(p []byte) (int, error) {
	return play.r.Read(p)
}

func parsePart(scanner *bufio.Scanner) (*mpart, error) {
	var (
		index int32
		st    float64
	)
	part := &mpart{}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE") {
			part.partID = strings.Replace(line, "#EXT-X-MEDIA-SEQUENCE:", "", 1)
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION") {
			dur, err := strconv.Atoi(strings.Replace(line, "#EXT-X-TARGETDURATION:", "", 1))
			if err != nil {
				return part, err
			}
			part.targetDuration = dur
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-ENDLIST") {
			part.offline = true
			continue
		}
		if strings.HasPrefix(line, "#EXTINF") {
			durstr, err := strconv.ParseFloat(strings.Split(strings.Replace(line, "#EXTINF:", "", 1), ",")[0], 32)
			if err != nil {
				return part, err
			}
			st = durstr
			index++
		} else {
			if st > 0 && index > 0 {
				start := part.duration
				part.streams = append(part.streams, &stream{
					index:    index,
					start:    start,
					duration: st,
					url:      line,
				})
				part.duration = start + st
			}
			st = 0
		}
	}
	part.fileNum = index
	return part, nil
}

func updateM3U8(m *M3U8, part *mpart) {
	m.parts = append(m.parts, part)
	m.duration += part.duration
	m.offline = part.offline
	for _, item := range part.streams {
		if notIn(m.streams, item) {
			m.streams = append(m.streams, item)
			m.streamchan <- item
		}
	}
}

func notIn(streams []*stream, item *stream) bool {
	for _, one := range streams {
		if one.url == item.url {
			return false
		}
	}
	return true
}

func isURL(url string) bool {
	return urlreg.MatchString(url)
}

func respOk(resp *http.Response) bool {
	return resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed
}

func getResp(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		time.Sleep(time.Second)
		resp, err = http.Get(url)
		if err != nil {
			return nil, err
		}
	}
	if !respOk(resp) {
		resp.Body.Close()
		time.Sleep(time.Second)
		resp, err = http.Get(url)
		if err != nil {
			time.Sleep(time.Second)
			resp, err = http.Get(url)
			if err != nil {
				return nil, err
			}
		}
	}
	if !respOk(resp) {
		time.Sleep(time.Second)
		resp, err = http.Get(url)
		if err != nil {
			if !respOk(resp) {
				return resp, fmt.Errorf(resp.Status)
			}
			return resp, err
		}
		return resp, err
	}
	return resp, nil
}
