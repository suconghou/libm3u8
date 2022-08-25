package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/suconghou/libm3u8"
	"github.com/suconghou/libm3u8/util"
)

var (
	ur = regexp.MustCompile(`^/(?i:https?):/{1,2}[[:print:]]+$`)
)

func main() {
	if len(os.Args) >= 3 {
		switch os.Args[1] {
		case "play":
			play()
		case "list":
			list()
		case "serve":
			serve()
		}
	} else if len(os.Args) >= 2 && os.Args[1] == "serve" {
		serve()
	} else {
		stream()
	}
}

func play() {
	m := libm3u8.NewFromURL(func() string { return os.Args[2] })
	if _, err := io.Copy(os.Stdout, m.Stream(nil)); err != nil {
		util.Log.Print(err)
	}
}

func list() {
	m := libm3u8.NewFromURL(func() string { return os.Args[2] })
	if _, err := io.Copy(os.Stdout, m); err != nil {
		util.Log.Print(err)
	}
}

func stream() {
	m := libm3u8.NewFromReader(os.Stdin, nil)
	if _, err := io.Copy(os.Stdout, m.Stream(nil)); err != nil {
		util.Log.Print(err)
	}
}

func serve() {
	var (
		port = flag.Int("p", 6060, "listen port")
		host = flag.String("h", "", "bind address")
	)
	flag.CommandLine.Parse(os.Args[2:])
	http.HandleFunc("/", routeMatch)
	util.Log.Printf("Starting up on port %d", *port)
	util.Log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *host, *port), nil))
}

func routeMatch(w http.ResponseWriter, r *http.Request) {
	if !ur.MatchString(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	var u = r.RequestURI
	u = strings.Replace(strings.TrimPrefix(u, "/"), ":/", "://", 1)
	target, err := url.Parse(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var m3u8URL = target.String()
	var m = libm3u8.NewFromURL(func() string {
		select {
		case <-r.Context().Done():
			return "" // 标记关闭输入端
		default:
			return m3u8URL
		}
	})
	var stream = m.Stream(nil)
	io.Copy(w, stream)
	m.Close()      // 关闭地址分析
	stream.Close() // 关闭ts合成流
}
