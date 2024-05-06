package packer

import (
	"fmt"
	"os"
	"strings"

	"github.com/suconghou/libm3u8"
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
	var isFirst = true
	for ts := range s.m.List() {
		if isFirst && ts.MAP() != "" {
			b, err := ts.Bytes(true)
			if err != nil {
				return s.p, err
			}
			n, err := s.f.Write(b)
			if err != nil {
				return s.p, err
			}
			if _, err = s.f.WriteAt(s.header(s.p, n, 0), 0); err != nil {
				return s.p, err
			}
			s.p += int64(n)
			isFirst = false
		}
		b, err := ts.Bytes(false)
		if err != nil {
			return s.p, err
		}
		n, err := s.f.Write(b)
		if err != nil {
			return s.p, err
		}
		if _, err = s.f.WriteAt(s.header(s.p, n, ts.Duration()), 0); err != nil {
			return s.p, err
		}
		s.p += int64(n)
	}
	return s.p, nil
}

func (s *Packer) header(offset int64, n int, d float64) []byte {
	var isFirst = s.h.Len() < 1
	if isFirst {
		s.h.WriteByte('[')
		s.h.WriteString(fmt.Sprintf("[%.1f,[%d,%d]]", d, offset, n))
	} else {
		s.h.WriteString(fmt.Sprintf(",[%.1f,[%d,%d]]", d, offset, n))
	}
	return padding(s.h.String())
}

// 返回的字节数为65536
func padding(s string) []byte {
	var x = 65535 - len(s)
	return []byte(s + "]" + strings.Repeat(" ", x))
}
