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
	"sync/atomic"
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
	args, headers := parseArgs(os.Args[1:])
	if arg(args, 0) == "serve" {
		serve(args[1:])
		return
	}
	if u := arg(args, 1); u != "" {
		switch args[0] {
		case "play":
			play(u, headers)
		case "list":
			list(u, headers)
		case "pack":
			pack(u, arg(args, 2), headers)
		}
		return
	}
	// 未指定URL(或未指定子命令)时回退为从标准输入读取播放列表，输出到标准输出
	stream(headers)
}

// parseArgs 一次遍历分离出位置参数与 -H 指定的请求头，-H 可重复出现且位置任意
func parseArgs(args []string) ([]string, http.Header) {
	var (
		positional = make([]string, 0, len(args))
		headers    = make(http.Header)
	)
	for i := 0; i < len(args); i++ {
		if args[i] == "-H" && i+1 < len(args) {
			i++
			if key, value, ok := strings.Cut(args[i], ":"); ok {
				headers.Add(strings.TrimSpace(key), strings.TrimSpace(value))
			}
			continue
		}
		positional = append(positional, args[i])
	}
	return positional, headers
}

// arg 返回第 n 个位置参数，不存在时返回空字符串
func arg(args []string, n int) string {
	if n >= 0 && n < len(args) {
		return args[n]
	}
	return ""
}

func play(u string, headers http.Header) {
	m := libm3u8.NewFromURL(ctx, func() string { return u }, headers)
	fetcher := func(url string) (io.ReadCloser, error) {
		return util.GetBody(ctx, url, headers)
	}
	if _, err := io.Copy(os.Stdout, m.Stream(fetcher)); err != nil {
		util.Log.Print(err)
	}
}

func list(u string, headers http.Header) {
	m := libm3u8.NewFromURL(ctx, func() string { return u }, headers)
	for ts := range m.List() {
		if _, err := fmt.Println(ts.URL()); err != nil {
			util.Log.Print(err)
		}
	}
}

func stream(headers http.Header) {
	m := libm3u8.NewFromReader(ctx, os.Stdin, headers, nil)
	fetcher := func(url string) (io.ReadCloser, error) {
		return util.GetBody(ctx, url, headers)
	}
	if _, err := io.Copy(os.Stdout, m.Stream(fetcher)); err != nil {
		util.Log.Print(err)
	}
}

// minFree 剩余header空间低于此值时，通知输入端停止拉取新的播放列表，
// 需明显大于packer单条索引项长度，以保证header空间耗尽前完成平滑停止
const minFree = 500

// pack 下载并打包 m3u8 为单文件，prefix 为可选的文件名前缀，最终文件名为 前缀+时间戳
func pack(u, prefix string, headers http.Header) {
	var (
		fname = fmt.Sprintf("%s%d", strings.TrimSpace(prefix), time.Now().Unix())
		// stop 由progress回调(main协程)写入、由nextURL(M3U8后台协程)读取，必须使用原子变量
		stop atomic.Bool
		p    = packer.New(libm3u8.NewFromURL(ctx, func() string {
			if stop.Load() {
				return ""
			}
			return u
		}, headers), fname)
		progress = func(size int64, free int) error {
			if free < minFree {
				stop.Store(true)
			}
			return nil
		}
	)
	util.Log.Println(u, fname)
	n, err := p.Receive(progress)
	// n == 0 表示没有可用数据，此时不会创建输出文件，不应记为成功
	if err != nil || n == 0 {
		util.Log.Println(n, err)
		return
	}
	util.Log.Println(fname, n)
}

func serve(args []string) {
	var (
		port = flag.Int("p", 6060, "listen port")
		host = flag.String("h", "", "bind address")
	)
	if err := flag.CommandLine.Parse(args); err != nil {
		util.Log.Panic(err)
	}
	http.HandleFunc("/", routeMatch)
	util.Log.Printf("Starting up on port %d", *port)
	util.Log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), nil))
}

func file(w http.ResponseWriter, r *http.Request) error {
	var fname = strings.TrimLeft(r.URL.Path, "/")
	if before, found := strings.CutSuffix(fname, ".m3u8"); found {
		fname = before
		f, err := os.Open(fname)
		if err != nil {
			return err
		}
		defer f.Close()
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
	} else if before, found := strings.CutSuffix(fname, ".ts"); found {
		fname = before
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
