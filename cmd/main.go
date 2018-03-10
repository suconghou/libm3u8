package main

import (
	"bufio"
	"io"
	"os"

	"github.com/suconghou/libm3u8"
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
	io.Copy(os.Stdout, m.Play())
}

func playList() {
	m := libm3u8.NewFromURL(func() string { return os.Args[2] })
	io.Copy(os.Stdout, m)
}

func playStream() {
	m := libm3u8.NewReader(bufio.NewScanner(os.Stdin))
	io.Copy(os.Stdout, m)
}
