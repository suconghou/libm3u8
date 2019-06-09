package parser

import (
	"bufio"
	"io"
	"strings"

	"github.com/suconghou/libm3u8/util"
)

const (
	endList  = "#EXT-X-ENDLIST"
	inf      = "#EXTINF"
	duration = "#EXT-X-TARGETDURATION"
)

// Parse do loop parse
func Parse(scanner *bufio.Scanner) io.Reader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		set := make(map[string]bool)
		var flag bool
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if found, _ := util.GetValue(line, endList); found {
				w.CloseWithError(io.EOF)
				break
			}
			if found, _ := util.GetValue(line, inf); found {
				flag = true
				continue
			}
			if found, _ := util.GetValue(line, "#"); (!found) && flag {
				if set[line] {
					continue
				} else {
					w.Write([]byte(line + "\n"))
					set[line] = true
				}
			}
			flag = false
		}
		if err := scanner.Err(); err != nil {
			w.CloseWithError(err)
		} else {
			w.CloseWithError(io.EOF)
		}
	}(w)
	return r
}
