package main

import (
	"bufio"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/pvz/runtimebridge"
	"os"
)

func main() {
	b := runtimebridge.New()
	s := bufio.NewScanner(os.Stdin)
	s.Buffer(make([]byte, 65536), 4<<20)
	for s.Scan() {
		fmt.Println(b.JSON(s.Text()))
	}
	if s.Err() != nil {
		os.Exit(1)
	}
}
