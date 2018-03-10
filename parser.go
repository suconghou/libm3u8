package libm3u8

import (
	"bufio"
	"io"
	"strings"
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
			if found, _ := getValue(line, "#EXT-X-ENDLIST"); found {
				w.CloseWithError(io.EOF)
				break
			}
			if found, _ := getValue(line, "#EXTINF"); found {
				flag = true
				continue
			}
			if found, _ := getValue(line, "#"); (!found) && flag {
				if set[line] {
					continue
				} else {
					w.Write([]byte(line + "\n"))
					set[line] = true
				}
			}
			flag = false
		}
	}(w)
	return r
}
