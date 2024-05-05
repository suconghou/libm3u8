package packer

import (
	"fmt"
	"libm3u8"
	"os"
	"strings"
)

type Packer struct {
	f *os.File
	m *libm3u8.M3U8
	h strings.Builder
	p int64
}

func New(m *libm3u8.M3U8, fname string) (*Packer, error) {
	f, err := os.Create(fname)
	if err != nil {
		return nil, err
	}
	s := &Packer{f, m, strings.Builder{}, 0}
	s.p, err = f.Seek(65536, 0)
	if err != nil {
		return s, err
	}
	return s, nil
}

// New之后必须调用此方法，此调用阻塞
func (s *Packer) Receive() (int64, error) {
	for ts := range s.m.List() {
		b, err := ts.Bytes()
		if err != nil {
			return s.p, err
		}
		n, err := s.f.Write(b)
		if err != nil {
			return s.p, err
		}
		_, h := s.header(s.p, n, ts.Duration())
		_, err = s.f.WriteAt(h, 0)
		if err != nil {
			return s.p, err
		}
		s.p += int64(n)
	}
	return s.p, nil
}

func (s *Packer) header(offset int64, n int, d float64) (bool, []byte) {
	var isFirst = s.h.Len() < 1
	if isFirst {
		s.h.WriteByte('[')
		s.h.WriteString(fmt.Sprintf("[%.1f,[%d,%d]]", d, offset, n))
	} else {
		s.h.WriteString(fmt.Sprintf(",[%.1f,[%d,%d]]", d, offset, n))
	}
	return isFirst, padding(s.h.String())
}

// 返回的字节数为65536
func padding(s string) []byte {
	var x = 65535 - len(s)
	return []byte(s + "]" + strings.Repeat(" ", x))
}
