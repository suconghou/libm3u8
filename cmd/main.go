package main

import (
	"fmt"
	"io"
	"os"

	"github.com/suconghou/libm3u8"
)

func main() {
	m, err := libm3u8.NewFromURL(os.Args[1], nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	io.Copy(os.Stdout, m.Play())
}
