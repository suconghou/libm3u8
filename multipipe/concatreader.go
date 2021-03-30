package multipipe

import (
	"io"

	"github.com/suconghou/libm3u8/util"
)

// ConcatReader return a reader concat all given readers, io.EOF value to stop
func ConcatReader(fn func() (io.ReadCloser, error)) io.Reader {
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

// ConcatReaderByURL read url concat its response, empty string to stop
func ConcatReaderByURL(fn func() string) io.Reader {
	return ConcatReader(func() (io.ReadCloser, error) {
		url := fn()
		if url == "" {
			return nil, io.EOF
		}
		return util.GetBody(url)
	})
}
