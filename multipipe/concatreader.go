package multipipe

import (
	"io"

	"github.com/suconghou/libm3u8/util"
)

// ConcatReader 将函数返回的ReadCloser流组装为一个Reader流，直到fn返回错误（io.EOF视为正确结束，其他视为错误）或者io.Copy错误
func ConcatReader(fn func() (io.ReadCloser, error)) *io.PipeReader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		for {
			source, err := fn()
			if err == io.EOF {
				w.Close()
				return
			}
			if err != nil {
				w.CloseWithError(err)
				return
			}
			_, err = io.Copy(w, source)
			source.Close()
			if err != nil {
				w.CloseWithError(err)
				return
			}
		}
	}(w)
	return r
}

// ConcatReaderByURL 将函数返回的url视为文件地址，程序请求此http地址并将这些流全部拼接为一个Reader,函数返回空地址视为正确结束
func ConcatReaderByURL(fn func() (string, error), loader func(string) (io.ReadCloser, error)) *io.PipeReader {
	if loader == nil {
		loader = util.GetBody
	}
	return ConcatReader(func() (io.ReadCloser, error) {
		url, err := fn()
		if err != nil {
			return nil, err
		}
		if url == "" {
			return nil, io.EOF
		}
		return loader(url)
	})
}
