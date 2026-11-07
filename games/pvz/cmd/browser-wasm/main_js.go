//go:build js && wasm

package main

import (
	"github.com/k0ngk0ng/wire-board/games/pvz/runtimebridge"
	"syscall/js"
)

func main() {
	b := runtimebridge.New()
	call := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"ok":false,"error":"expected one JSON string"}`
		}
		return b.JSON(args[0].String())
	})
	js.Global().Set("pvzWasm", call)
	select {}
}
