package main

import (
	"bufio"
	"io"
	"log"
	"os"

	"github.com/suconghou/libm3u8"
)

var (
	mlog = log.New(os.Stderr, "", log.Lshortfile)
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "play":
			play()
		case "list":
			playList()
		}
	} else {
		playStream()
	}
}

func play() {
	m := libm3u8.NewFromURL(func() string { return os.Args[2] })
	_, err := io.Copy(os.Stdout, m.Play())
	pe(err)
}

func playList() {
	m := libm3u8.NewFromURL(func() string { return os.Args[2] })
	_, err := io.Copy(os.Stdout, m)
	pe(err)
}

func playStream() {
	m := libm3u8.NewReader(bufio.NewScanner(os.Stdin))
	_, err := io.Copy(os.Stdout, m)
	pe(err)
}

func pe(err error) {
	if err != nil {
		mlog.Print(err)
	}
}
