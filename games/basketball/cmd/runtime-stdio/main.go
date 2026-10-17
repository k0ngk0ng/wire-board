// runtime-stdio is the native counterpart of the browser Wasm protocol.
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/k0ngk0ng/wire-board/games/basketball/runtimebridge"
)

func main() {
	bridge := runtimebridge.New()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 65536), runtimebridge.MaxRequestBytes+1)
	writer := bufio.NewWriter(os.Stdout)
	for scanner.Scan() {
		fmt.Fprintln(writer, bridge.JSON(scanner.Text()))
		if err := writer.Flush(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
