package libm3u8

import (
	"bufio"
	"io"
	"os"
)

// NewReader join url response to one io.Reader
func NewReader(scanner *bufio.Scanner) io.Reader {
	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		for scanner.Scan() {
			url := scanner.Text()
			if isURL(url) {
				resp, err := getResp(url, tryTimes)
				if err != nil {
					mlog.Print(err)
					continue
				}
				defer resp.Body.Close()
				_, err = io.Copy(w, resp.Body)
				if err != nil {
					mlog.Print(err)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			w.CloseWithError(err)
		} else {
			w.CloseWithError(io.EOF)
		}
	}(w)
	return r
}

// NewReaderFromURL return io.reader which join urllist response from url
func NewReaderFromURL(url string) (io.Reader, error) {
	resp, err := getResp(url, tryTimes)
	if err != nil {
		return nil, err
	}
	return NewReader(bufio.NewScanner(resp.Body)), nil
}

// NewReaderFromFile return io.reader which join urllist response from file
func NewReaderFromFile(path string) (io.Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return NewReader(bufio.NewScanner(file)), nil
}
