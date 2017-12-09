package main

import (
	"bufio"
	"fmt"
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
	m, err := libm3u8.NewFromURL(os.Args[2], nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	io.Copy(os.Stdout, m.Play())
}

func playList() {
	m, err := libm3u8.NewFromURL(os.Args[2], nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	io.Copy(os.Stdout, m)
}

func playStream() {
	r := os.Stdin
	scanner := bufio.NewScanner(r)
	m := libm3u8.NewReader(scanner)
	io.Copy(os.Stdout, m)
}
