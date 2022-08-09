package libm3u8

import (
	"bufio"
	"io"

	"libm3u8/multipipe"
	"libm3u8/util"
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
		if err := scanner.Err(); err != nil {
			util.Log.Print(err)
		}
		return ""
	}, nil)
}
