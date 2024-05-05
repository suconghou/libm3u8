package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"libm3u8"
	"libm3u8/packer"
	"libm3u8/util"
)

var (
	ur = regexp.MustCompile(`^/(?i:https?):/{1,2}[[:print:]]+$`)
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

func play(u string) {
	m := libm3u8.NewFromURL(func() string { return u })
	if _, err := io.Copy(os.Stdout, m.Stream(nil)); err != nil {
		util.Log.Print(err)
	}
}

func list(u string) {
	m := libm3u8.NewFromURL(func() string { return u })
	for ts := range m.List() {
		if _, err := fmt.Println(ts.URL()); err != nil {
			util.Log.Print(err)
		}
	}
}

func stream() {
	m := libm3u8.NewFromReader(os.Stdin, nil)
	if _, err := io.Copy(os.Stdout, m.Stream(nil)); err != nil {
		util.Log.Print(err)
	}
}

func pack(u string) {
	fname := fmt.Sprintf("%d", time.Now().Unix())
	s, err := packer.New(libm3u8.NewFromURL(func() string { return u }), fname)
	if err != nil {
		util.Log.Panic(err)
	}
	util.Log.Print(s.Receive())
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

func file(w http.ResponseWriter, r *http.Request) error {
	var fname = strings.TrimLeft(r.URL.Path, "/")
	if strings.HasSuffix(fname, ".m3u8") {
		fname = strings.Replace(fname, ".m3u8", "", -1)
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
		var s strings.Builder
		var maxDuration float64
		for _, el := range segments {
			d := el[0].(float64)
			if maxDuration < d {
				maxDuration = d
			}
			arr := el[1].([]any)
			offset := int(arr[0].(float64))
			length := int(arr[1].(float64))
			s.WriteString(fmt.Sprintf("#EXTINF:%.1f\n", d))
			s.WriteString(fmt.Sprintf("%s?range=%d-%d\n", fname, offset, offset+length-1))
		}
		var body strings.Builder
		body.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
		body.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(math.Ceil(maxDuration))))
		body.WriteString(s.String())
		_, err = w.Write([]byte(body.String()))
		return err
	}
	var ran = r.URL.Query().Get("range")
	if r.Header.Get("range") == "" && ran != "" {
		r.Header.Set("range", fmt.Sprintf("bytes=%s", ran))
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
		m       = libm3u8.NewFromURL(func() string {
			select {
			case <-r.Context().Done():
				return "" // 标记关闭输入端
			default:
				return m3u8URL
			}
		})
		stream = m.Stream(nil)
	)
	n, err := io.Copy(w, stream)
	stream.Close() // 关闭ts合成流
	if err != nil {
		if n < 1 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			util.Log.Print(err)
		}
	}
}
