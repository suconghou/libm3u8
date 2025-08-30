package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/suconghou/libm3u8"
	"github.com/suconghou/libm3u8/packer"
	"github.com/suconghou/libm3u8/util"
)

var (
	ur  = regexp.MustCompile(`^/(?i:https?):/{1,2}[[:print:]]+$`)
	ctx = context.Background()
)

func main() {
	if len(os.Args) >= 3 {
		switch os.Args[1] {
		case "play":
			play(os.Args[2])
		case "list":
			list(os.Args[2])
		case "pack":
			pack(os.Args[2])
		case "serve":
			serve()
		}
	} else if len(os.Args) >= 2 && os.Args[1] == "serve" {
		serve()
	} else {
		stream()
	}
}
func parseHeaders() http.Header {
	headers := make(http.Header)
	// 定义 -H 标志，支持多次使用
	headerFlags := make([]string, 0)
	// 临时解析命令行参数来获取 -H 标志
	tempArgs := os.Args[1:]
	for i := 0; i < len(tempArgs); i++ {
		if tempArgs[i] == "-H" && i+1 < len(tempArgs) {
			headerFlags = append(headerFlags, tempArgs[i+1])
			i++ // 跳过参数值
		}
	}
	// 解析每个header
	for _, header := range headerFlags {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers.Add(key, value)
		}
	}
	return headers
}

func play(u string) {
	headers := parseHeaders()
	m := libm3u8.NewFromURL(ctx, func() string { return u }, headers)
	fetcher := func(url string) (io.ReadCloser, error) {
		return util.GetBody(ctx, url, headers)
	}
	if _, err := io.Copy(os.Stdout, m.Stream(fetcher)); err != nil {
		util.Log.Print(err)
	}
}

func list(u string) {
	headers := parseHeaders()
	m := libm3u8.NewFromURL(ctx, func() string { return u }, headers)
	for ts := range m.List() {
		if _, err := fmt.Println(ts.URL()); err != nil {
			util.Log.Print(err)
		}
	}
}

func stream() {
	headers := parseHeaders()
	m := libm3u8.NewFromReader(ctx, os.Stdin, headers, nil)
	fetcher := func(url string) (io.ReadCloser, error) {
		return util.GetBody(ctx, url, headers)
	}
	if _, err := io.Copy(os.Stdout, m.Stream(fetcher)); err != nil {
		util.Log.Print(err)
	}
}

func pack(u string) {
	var (
		headers = parseHeaders()
		fname   = fmt.Sprintf("%d", time.Now().Unix())
		stop    = false
		pack    = packer.New(libm3u8.NewFromURL(ctx, func() string {
			if stop {
				return ""
			}
			return u
		}, headers), fname)
		progress = func(size int64, free int) error {
			if free < 500 {
				stop = true
			}
			return nil
		}
	)
	util.Log.Println(u, fname)
	util.Log.Print(pack.Receive(progress))
}

func serve() {
	var (
		port = flag.Int("p", 6060, "listen port")
		host = flag.String("h", "", "bind address")
	)
	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		util.Log.Panic(err)
	}
	http.HandleFunc("/", routeMatch)
	util.Log.Printf("Starting up on port %d", *port)
	util.Log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), nil))
}

func file(w http.ResponseWriter, r *http.Request) error {
	var fname = strings.TrimLeft(r.URL.Path, "/")
	if strings.HasSuffix(fname, ".m3u8") {
		fname = strings.ReplaceAll(fname, ".m3u8", "")
		f, err := os.Open(fname)
		if err != nil {
			return err
		}
		var header = make([]byte, 65536)
		if _, err = io.ReadFull(f, header); err != nil {
			return err
		}
		var segments [][]any
		if err := json.Unmarshal(header, &segments); err != nil {
			return err
		}
		var ll = len(segments)
		var live = r.URL.Query().Get("live")
		var cut = 0
		if live == "1" && ll >= 20 {
			cut = ll - 10
		}
		var s = &strings.Builder{}
		var maxDuration float64
		var x, y int
		for index, el := range segments {
			d := el[0].(float64)
			if maxDuration < d {
				maxDuration = d
			}
			arr := el[1].([]any)
			offset := int(arr[0].(float64))
			length := int(arr[1].(float64))
			if index == 0 && d < 0.1 {
				x = offset
				y = offset + length - 1
				continue
			}
			if index < cut {
				continue
			}
			fmt.Fprintf(s, "#EXTINF:%.1f\n", d)
			fmt.Fprintf(s, "%s.ts?range=%d-%d\n", fname, offset, offset+length-1)
		}
		var body = &strings.Builder{}
		body.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
		fmt.Fprintf(body, "#EXT-X-TARGETDURATION:%d\n", int(math.Ceil(maxDuration)))
		if x > 0 && y > 0 && y > x {
			fmt.Fprintf(body, "#EXT-X-MAP:URI=\"%s.ts?range=%d-%d\"\n", fname, x, y)
		}
		body.WriteString(s.String())
		if live == "0" {
			body.WriteString("#EXT-X-ENDLIST")
		}
		_, err = w.Write([]byte(body.String()))
		return err
	} else if strings.HasSuffix(fname, ".ts") {
		fname = strings.ReplaceAll(fname, ".ts", "")
		f, err := os.Open(fname)
		if err != nil {
			return err
		}
		defer f.Close()
		var arr = strings.Split(r.URL.Query().Get("range"), "-")
		if len(arr) != 2 {
			return nil
		}
		start, err := strconv.ParseInt(arr[0], 10, 64)
		if err != nil {
			return err
		}
		end, err := strconv.ParseInt(arr[1], 10, 64)
		if err != nil {
			return err
		}
		var buf = make([]byte, end-start+1)
		if _, err = f.ReadAt(buf, start); err != nil {
			return err
		}
		_, err = w.Write(buf)
		return err
	}
	http.ServeFile(w, r, fname)
	return nil
}

func routeMatch(w http.ResponseWriter, r *http.Request) {
	if !ur.MatchString(r.URL.Path) {
		if err := file(w, r); err != nil {
			util.Log.Print(err)
		}
		return
	}
	var u = r.RequestURI
	u = strings.Replace(strings.TrimPrefix(u, "/"), ":/", "://", 1)
	target, err := url.Parse(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var (
		m3u8URL = target.String()
		m       = libm3u8.NewFromURL(ctx, func() string {
			select {
			case <-r.Context().Done():
				return "" // 标记关闭输入端
			default:
				return m3u8URL
			}
		}, r.Header)
		stream = m.Stream(func(s string) (io.ReadCloser, error) {
			return util.GetBody(r.Context(), s, r.Header)
		})
	)
	n, err := io.Copy(w, stream)
	_ = stream.Close() // 关闭ts合成流
	if err != nil {
		if n < 1 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			util.Log.Print(err)
		}
	}
}
