package libm3u8

import (
	"bufio"
	"io"

	"github.com/suconghou/libm3u8/multipipe"
)

// M3U8 resource
type M3U8 struct {
	io.Reader
}

// Play ts file
func (m *M3U8) Play() io.Reader {
	var scanner = bufio.NewScanner(m)
	return multipipe.ConcatReaderByURL(func() string {
		if scanner.Scan() {
			return scanner.Text()
		}
		return ""
	})
}
