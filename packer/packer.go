package packer

import (
	"bytes"
	"fmt"
	"os"

	"github.com/suconghou/libm3u8"
)

type Packer struct {
	m     *libm3u8.M3U8
	h     *bytes.Buffer
	p     int64
	fname string
	l     int
}

func New(m *libm3u8.M3U8, fname string) *Packer {
	return &Packer{m, &bytes.Buffer{}, 0, fname, 65536}
}

// 设置header空间，单位KB，大小应在 4 - 512 之间 （4KB-512KB）
func (s *Packer) Limit(n int) {
	s.l = 1024 * n
}

// 文件写入，此调用阻塞, 执行过程中将会调用progress上报当前文件大小及剩余header空间，progress必须在剩余header空间较小时通知m关闭
// progress返回非nil时，强制停止文件写入并返回此错误，返回nil时则等待外部m终止后平滑停止
func (s *Packer) Receive(progress func(int64, int) error) (int64, error) {
	var (
		isFirst = true
		fd      *os.File
		buf     = &bytes.Buffer{}
	)
	for ts := range s.m.List() {
		if isFirst && ts.MAP() != "" {
			if err := ts.Bytes(buf, true); err != nil {
				return s.p, err
			}
			if fd == nil {
				if f, err := s.file(); err == nil {
					fd = f
					defer fd.Close()
				} else {
					return s.p, err
				}
			}
			n, err := fd.Write(buf.Bytes())
			if err != nil {
				return s.p, err
			}
			header, l := s.header(s.p, n, 0)
			if _, err = fd.WriteAt(header, 0); err != nil {
				return s.p, err
			}
			s.h.Truncate(l)
			s.p += int64(n)
			isFirst = false
		}
		if err := ts.Bytes(buf, false); err != nil {
			return s.p, err
		}
		if fd == nil {
			if f, err := s.file(); err == nil {
				fd = f
				defer fd.Close()
			} else {
				return s.p, err
			}
		}
		n, err := fd.Write(buf.Bytes())
		if err != nil {
			return s.p, err
		}
		header, l := s.header(s.p, n, ts.Duration())
		if _, err = fd.WriteAt(header, 0); err != nil {
			return s.p, err
		}
		s.h.Truncate(l)
		s.p += int64(n)
		free := s.l - l
		// 通知外部文件大小及剩余header空间，progress应在free较小时通知m关闭，如果progress返回非nil，则立即强制关闭，否则等待m关闭后平滑停止
		// 但是如果平滑停止过程中header不足，则提前退出
		if err = progress(s.p, free); err != nil {
			return s.p, err
		} else if free < 50 {
			return s.p, s.m.Err()
		}
	}
	return s.p, s.m.Err()
}

// 初始化文件，注意初始化文件指针即指向后续二进制数据部分
func (s *Packer) file() (*os.File, error) {
	f, err := os.Create(s.fname)
	if err != nil {
		return nil, err
	}
	s.p, err = f.Seek(int64(s.l), 0)
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// 返回的字节数为设定的p.l字节数,第二个为后续有效数据
func (s *Packer) header(offset int64, n int, d float64) ([]byte, int) {
	var isFirst = s.h.Len() < 1
	if isFirst {
		fmt.Fprintf(s.h, "[[%.1f,[%d,%d]]", d, offset, n)
	} else {
		fmt.Fprintf(s.h, ",[%.1f,[%d,%d]]", d, offset, n)
	}
	var l = s.h.Len()   // 后续保持数据
	var x = s.l - 1 - l // 补空格数
	s.h.WriteByte(']')
	s.h.Write(bytes.Repeat([]byte(" "), x))
	return s.h.Bytes(), l
}
