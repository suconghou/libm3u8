package packer

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/suconghou/libm3u8"
)

type Packer struct {
	m     *libm3u8.M3U8
	h     *strings.Builder
	p     int64
	fname string
}

func New(m *libm3u8.M3U8, fname string) *Packer {
	return &Packer{m, &strings.Builder{}, 0, fname}
}

// New之后必须调用此方法，此调用阻塞
func (s *Packer) Receive() (int64, error) {
	var (
		isFirst = true
		fd      *os.File
	)
	for ts := range s.m.List() {
		if isFirst && ts.MAP() != "" {
			b, err := ts.Bytes(true)
			if err != nil {
				return s.p, err
			}
			if fd == nil {
				if f, err := s.file(); err == nil {
					fd = f
				} else {
					return s.p, err
				}
			}
			n, err := fd.Write(b)
			if err != nil {
				return s.p, err
			}
			if _, err = fd.WriteAt(s.header(s.p, n, 0), 0); err != nil {
				return s.p, err
			}
			s.p += int64(n)
			isFirst = false
		}
		b, err := ts.Bytes(false)
		if err != nil {
			return s.p, err
		}
		if fd == nil {
			if f, err := s.file(); err == nil {
				fd = f
			} else {
				return s.p, err
			}
		}
		n, err := fd.Write(b)
		if err != nil {
			return s.p, err
		}
		if _, err = fd.WriteAt(s.header(s.p, n, ts.Duration()), 0); err != nil {
			return s.p, err
		}
		s.p += int64(n)
	}
	return s.p, s.m.Err()
}

// 初始化文件，注意初始化文件指针即指向后续二进制数据部分
func (s *Packer) file() (*os.File, error) {
	f, err := os.Create(s.fname)
	if err != nil {
		return nil, err
	}
	s.p, err = f.Seek(65536, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Packer) header(offset int64, n int, d float64) []byte {
	var isFirst = s.h.Len() < 1
	if isFirst {
		s.h.WriteByte('[')
		fmt.Fprintf(s.h, "[%.1f,[%d,%d]]", d, offset, n)
	} else {
		fmt.Fprintf(s.h, ",[%.1f,[%d,%d]]", d, offset, n)
	}
	return padding(s.h.String())
}

// 返回的字节数为65536
func padding(s string) []byte {
	var x = 65535 - len(s)
	return append([]byte(s+"]"), bytes.Repeat([]byte(" "), x)...)
}
