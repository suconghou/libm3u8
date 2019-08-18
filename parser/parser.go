package parser

import (
	"bufio"
	"io"
	"strings"
)

// Parse until scanner end and give each url,caller should stop scanner
func Parse(scanner *bufio.Scanner, formater func(string) string) io.Reader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		urls := map[string]bool{}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if formater != nil {
				line = formater(line)
				if line == "" {
					continue
				}
			}
			if urls[line] {
				continue
			}
			w.Write([]byte(line + "\n"))
			urls[line] = true
		}
		if err := scanner.Err(); err != nil {
			w.CloseWithError(err)
		} else {
			w.CloseWithError(io.EOF)
		}
	}(w)
	return r
}
