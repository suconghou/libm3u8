package main

import (
	"io"
	"os"

	"github.com/suconghou/libm3u8"
	"github.com/suconghou/libm3u8/util"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "play":
			play()
		case "list":
			list()
		}
	} else {
		stream()
	}
}

func play() {
	m := libm3u8.NewFromURL(func() string { return os.Args[2] })
	if _, err := io.Copy(os.Stdout, m.Play()); err != nil {
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
	if _, err := io.Copy(os.Stdout, m.Play()); err != nil {
		util.Log.Print(err)
	}
}
